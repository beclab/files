// Package precheck centralizes the cheap, synchronous validation that
// every paste request must pass before a task is allocated.
//
// The historical paste pipeline (PasteMethod -> driver.Paste ->
// TaskManager.CreateTask -> task.Execute) only validates the source
// path at task-execution time, well after a 200 + task_id has been
// returned to the client. Depending on the route, a missing source
// can either fail loudly mid-task (Rsync / rclone) or silently
// "succeed" with zero bytes (GetFromSyncFileCount returning (0, nil)
// for a missing dirent, DownloadFromFiles returning nil for an empty
// SSE stream) -- leaving the user with a phantom Completed task and
// nothing transferred.
//
// SourceExists fails the request up front when the source can't be
// located on its backend, regardless of which combination of src/dst
// storage is involved. The HTTP layer collapses every such failure
// into a single status + message; callers here don't differentiate
// "really not there" from "unreachable right now" because the
// downstream impact is identical.
package precheck

import (
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/integration"
	"files/pkg/models"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// remoteStatTimeout bounds the cross-node /api/resources existence
// probe. Generous enough for a slow tree walk on a large external disk,
// short enough that a hung peer doesn't keep the inbound HTTP request
// queued indefinitely.
const remoteStatTimeout = 10 * time.Second

// remoteClient is reused across calls so we don't churn TCP/TLS state
// when many pastes happen in quick succession.
var remoteClient = &http.Client{Timeout: remoteStatTimeout}

// SourceExists verifies that paste.Src points to a real file/dir on
// its backend. It is node-aware: when the source is on a different
// node than the one currently handling the request (only possible for
// the posix-on-disk family -- cache / external / internal / smb / usb
// / hdd), the check is forwarded to the owning files-pod's
// /api/resources endpoint over HTTP. Sync and cloud backends are
// reachable from any node via their respective RPCs.
//
// share=1 requests carry the share grantor's identity in SrcOwner /
// SrcSharePath; we honor it so the existence probe runs as the right
// user. The internal share token gate already happened in PasteMethod
// before this function is called.
func SourceExists(p *models.PasteParam) error {
	if p == nil {
		return errors.New("paste param is nil")
	}
	if p.Src == nil {
		return errors.New("paste source param is nil")
	}

	// Resolve the effective src + owner. For share=1 the grantor
	// (SrcOwner) is the owning identity even though the request was
	// posted by a different user.
	src := p.Src
	owner := src.Owner
	if p.Share == 1 {
		if p.SrcOwner != "" {
			owner = p.SrcOwner
		}
		// SrcSharePath carries the share-URL form of the path; when
		// present we still resolve against the real Src params because
		// those have already been re-mapped by the share proxy.
		// SrcSharePath is kept for permission logs only.
	}

	switch src.FileType {
	case common.Sync:
		return checkSync(src)
	case common.AwsS3, common.TencentCos, common.GoogleDrive, common.DropBox:
		return checkCloud(src)
	case common.Drive:
		// Drive (Home/Data) is always on the master node and the
		// paste handler that reaches here is also the master, so
		// stat the local fs directly.
		return checkLocal(src)
	case common.Cache, common.External, common.Internal, common.Smb, common.Usb, common.Hdd:
		// These backends are node-scoped. When src.Extend names a
		// different node than us, ask that node over HTTP rather
		// than stat the local fs (which would always say "missing"
		// for someone else's mount).
		if src.Extend == "" || src.Extend == global.CurrentNodeName {
			return checkLocal(src)
		}
		return checkRemote(src, owner)
	default:
		return fmt.Errorf("unsupported source file type for precheck: %s", src.FileType)
	}
}

// checkLocal stats the resolved on-disk path. Treats stat errors other
// than "not exist" (e.g. permission denied) as fatal too -- the paste
// won't succeed in any case.
func checkLocal(src *models.FileParam) error {
	uri, err := src.GetResourceUri()
	if err != nil {
		return fmt.Errorf("resolve src uri: %w", err)
	}
	full := uri + src.Path
	if !files.FilePathExists(full) {
		return fmt.Errorf("source not found: %s/%s%s", src.FileType, src.Extend, src.Path)
	}
	return nil
}

// checkSync asks the seafile RPC whether the path resolves to a file
// or a directory in the named repo. The file-vs-dir decision is made
// by the trailing-slash convention used everywhere else in this
// codebase (see FileParam.IsFile) -- a path ending in "/" is a dir,
// anything else is a file. We deliberately do NOT fall back to the
// other RPC when the first one returns empty: a caller asking for
// "/foo" when "/foo/" is what exists is a structural mismatch in the
// request, not something the precheck should paper over.
func checkSync(src *models.FileParam) error {
	if src.Extend == "" {
		return errors.New("sync source not found: repo id is empty")
	}

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(src.Extend)
	if err != nil {
		return fmt.Errorf("sync repo lookup: %w", err)
	}
	if repo == nil {
		return fmt.Errorf("sync source not found: repo %s", src.Extend)
	}

	// Repo root: the repo's existence is the only thing to verify.
	if src.Path == "" || src.Path == "/" {
		return nil
	}

	isDir := strings.HasSuffix(src.Path, "/")
	// Both seafile RPCs reject a trailing slash on the path, so we
	// strip it for the lookup. The original src.Path is still used
	// to derive isDir above and is what appears in error messages.
	path := strings.TrimRight(src.Path, "/")

	if isDir {
		did, e := seaserv.GlobalSeafileAPI.GetDirIdByPath(src.Extend, path, false)
		if e != nil {
			return fmt.Errorf("sync dir lookup: %w", e)
		}
		if did == "" {
			return fmt.Errorf("sync source not found: sync/%s%s", src.Extend, src.Path)
		}
		return nil
	}

	fid, e := seaserv.GlobalSeafileAPI.GetFileIdByPath(src.Extend, path)
	if e != nil {
		return fmt.Errorf("sync file lookup: %w", e)
	}
	if fid == "" {
		return fmt.Errorf("sync source not found: sync/%s%s", src.Extend, src.Path)
	}
	return nil
}

// checkCloud delegates to rclone's Stat/Size, which already handles
// both files and dirs and returns an error for missing remotes. Any
// rclone error is treated as "source unreachable" for the HTTP layer;
// the detailed reason stays in the wrapped error chain for logging.
func checkCloud(src *models.FileParam) error {
	if _, err := rclone.Command.GetFilesSize(src); err != nil {
		return fmt.Errorf("cloud source not found: %s/%s%s (rclone: %v)",
			src.FileType, src.Extend, src.Path, err)
	}
	return nil
}

// checkRemote validates a node-scoped posix-like source that lives on
// a different node than the one currently servicing the request.
//
// We re-use the per-node files-pod IP discovered via the integration
// manager (the same pod the cross-node DownloadFromFiles task targets)
// and probe /api/resources/<fsType>/<extend><path>. A 2xx response
// means the path resolves; non-2xx is treated as "not found". This is
// intentionally aggressive: 500 / 502 / etc. from the remote also
// surface as a precheck failure here, because the alternative is to
// fail-open and let the task silently succeed with zero bytes on
// DownloadFromFiles -- which is exactly the bug this layer exists to
// prevent.
func checkRemote(src *models.FileParam, owner string) error {
	pod, err := integration.IntegrationManager().GetFilesPod(src.Extend)
	if err != nil {
		return fmt.Errorf("locate files pod for node %s: %w", src.Extend, err)
	}
	if pod.Status.PodIP == "" {
		return fmt.Errorf("files pod for node %s has no IP yet", src.Extend)
	}

	// Path begins with "/" already; sprintf without TrimPrefix keeps
	// the leading separator after the <extend> segment.
	url := fmt.Sprintf("http://%s/api/resources/%s/%s%s",
		pod.Status.PodIP, src.FileType, src.Extend, src.Path)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build remote stat request: %w", err)
	}
	if owner != "" {
		req.Header.Set(common.REQUEST_HEADER_OWNER, owner)
	}
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := remoteClient.Do(req)
	if err != nil {
		return fmt.Errorf("remote stat %s: %w", url, err)
	}
	defer func() {
		// Drain so the underlying connection can be reused even
		// when the body is large (e.g. dir listing).
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	klog.Warningf("[precheck] remote stat %s returned %d", url, resp.StatusCode)
	return fmt.Errorf("remote source not found: %s/%s%s (remote status %d)",
		src.FileType, src.Extend, src.Path, resp.StatusCode)
}

// DestinationWritable verifies paste.Dst can be written to BEFORE a
// task is created -- the dst-side counterpart to SourceExists. Per
// backend:
//
//   - posix (drive/cache/external/internal/smb/usb/hdd) on this node:
//     1-byte probe via files.WriteTempFile (distinguishes EACCES from
//     EROFS).
//   - posix on a different node: forward the probe via
//     GET /api/resources/<fsType>/<extend><parent>?probe=write.
//   - sync: CheckFolderPermission on the dst parent; only "rw" passes.
//   - cloud: list the bucket root. Catches bad creds / missing bucket
//     without writing anything; prefix-level write denies still
//     surface async via formatJobStatusError.
//
// share=1 swaps in p.DstOwner so the probe runs as the grantor.
func DestinationWritable(p *models.PasteParam) error {
	if p == nil {
		return errors.New("paste param is nil")
	}
	if p.Dst == nil {
		return errors.New("paste destination param is nil")
	}

	// Shallow copy + override Owner -- p.Dst is reused by handler.Paste
	// right after this and must not be mutated as a side effect.
	dst := *p.Dst
	if p.Share == 1 && p.DstOwner != "" {
		dst.Owner = p.DstOwner
	}

	switch dst.FileType {
	case common.Sync:
		return checkSyncWritable(&dst)
	case common.AwsS3, common.TencentCos, common.GoogleDrive, common.DropBox:
		return checkCloudWritable(&dst)
	case common.Drive:
		return checkLocalWritable(&dst)
	case common.Cache, common.External, common.Internal, common.Smb, common.Usb, common.Hdd:
		if dst.Extend == "" || dst.Extend == global.CurrentNodeName {
			return checkLocalWritable(&dst)
		}
		return checkRemoteWritable(&dst)
	default:
		return fmt.Errorf("unsupported destination file type for precheck: %s", dst.FileType)
	}
}

// probeParentDir returns dst.Path's parent with a trailing slash.
// WriteTempFile walks up from there to the deepest existing ancestor.
func probeParentDir(full string) string {
	tmp := strings.TrimSuffix(full, "/")
	if tmp == "" {
		return "/"
	}
	pos := strings.LastIndex(tmp, "/")
	if pos < 0 {
		return "/"
	}
	return tmp[:pos] + "/"
}

func checkLocalWritable(dst *models.FileParam) error {
	uri, err := dst.GetResourceUri()
	if err != nil {
		return fmt.Errorf("resolve dst uri: %w", err)
	}
	full := uri + dst.Path
	if err := files.WriteTempFile(probeParentDir(full)); err != nil {
		return fmt.Errorf("destination not writable: %s/%s%s (%v)",
			dst.FileType, dst.Extend, dst.Path, err)
	}
	return nil
}

// checkSyncWritable: only "rw" on the dst parent passes. "" / "r" /
// "cloud-edit" / etc. all fail.
func checkSyncWritable(dst *models.FileParam) error {
	if dst.Extend == "" {
		return errors.New("sync destination invalid: repo id is empty")
	}
	repo, err := seaserv.GlobalSeafileAPI.GetRepo(dst.Extend)
	if err != nil {
		return fmt.Errorf("sync repo lookup: %w", err)
	}
	if repo == nil {
		return fmt.Errorf("sync destination not found: repo %s", dst.Extend)
	}

	// dst.Path is the final entry's path; strip the basename to get
	// the dir we actually need to write into. Mirrors task_paste_sync.go.
	parent := strings.TrimSuffix(dst.Path, "/")
	if parent == "" {
		parent = "/"
	} else if pos := strings.LastIndex(parent, "/"); pos >= 0 {
		parent = parent[:pos+1]
	} else {
		parent = "/"
	}

	username := dst.Owner + "@auth.local"
	perm, err := seahub.CheckFolderPermission(username, dst.Extend, parent)
	if err != nil {
		return fmt.Errorf("sync permission lookup: %w", err)
	}
	if perm != "rw" {
		return fmt.Errorf("destination not writable: sync/%s%s (perm: %q)",
			dst.Extend, dst.Path, perm)
	}
	return nil
}

// checkCloudWritable lists the bucket root. About would be cleaner but
// S3 / Tencent COS don't implement it. Listing creates no probe object
// and catches bad creds / missing bucket. Prefix-level write ACLs are
// async-only (translated by formatJobStatusError).
func checkCloudWritable(dst *models.FileParam) error {
	if _, err := rclone.Command.GetFilesList(&models.FileParam{
		Owner:    dst.Owner,
		FileType: dst.FileType,
		Extend:   dst.Extend,
		Path:     "/",
	}, false); err != nil {
		return fmt.Errorf("cloud destination not reachable: %s/%s%s (rclone: %v)",
			dst.FileType, dst.Extend, dst.Path, err)
	}
	return nil
}

// checkRemoteWritable mirrors checkRemote but adds ?probe=write so the
// peer runs a write probe instead of a listing. Any non-2xx = fail
// (same aggressive policy as checkRemote).
func checkRemoteWritable(dst *models.FileParam) error {
	pod, err := integration.IntegrationManager().GetFilesPod(dst.Extend)
	if err != nil {
		return fmt.Errorf("locate files pod for node %s: %w", dst.Extend, err)
	}
	if pod.Status.PodIP == "" {
		return fmt.Errorf("files pod for node %s has no IP yet", dst.Extend)
	}

	url := fmt.Sprintf("http://%s/api/resources/%s/%s%s?probe=write",
		pod.Status.PodIP, dst.FileType, dst.Extend, dst.Path)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build remote probe request: %w", err)
	}
	if dst.Owner != "" {
		req.Header.Set(common.REQUEST_HEADER_OWNER, dst.Owner)
	}
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := remoteClient.Do(req)
	if err != nil {
		return fmt.Errorf("remote probe %s: %w", url, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	klog.Warningf("[precheck] remote probe %s returned %d", url, resp.StatusCode)
	return fmt.Errorf("destination not writable: %s/%s%s (remote status %d)",
		dst.FileType, dst.Extend, dst.Path, resp.StatusCode)
}
