package posix

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/diskcache"
	"files/pkg/drivers/base"
	"files/pkg/drivers/posix/upload"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/integration"
	"files/pkg/models"
	"files/pkg/preview"
	"files/pkg/tasks"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

var remoteClient = &http.Client{Timeout: 10 * time.Second}

// remoteLookupPodIP is a test seam over IntegrationManager().GetFilesPod.
var remoteLookupPodIP = func(node string) (string, error) {
	pod, err := integration.IntegrationManager().GetFilesPod(node)
	if err != nil {
		return "", fmt.Errorf("locate files pod for node %s: %w", node, err)
	}
	if pod.Status.PodIP == "" {
		return "", fmt.Errorf("files pod for node %s has no IP yet", node)
	}
	return pod.Status.PodIP, nil
}

var (
	Expand    = true
	Content   = true
	NoExpand  = false
	NoContent = false
)

type PosixStorage struct {
	handler *base.HandlerParam
	paste   *models.PasteParam
}

func NewPosixStorage(handler *base.HandlerParam) *PosixStorage {
	return &PosixStorage{
		handler: handler,
	}
}

// CheckPermission resolves the permission level for a POSIX-backed
// resource (drive Home/Data, cache, external).
//
// drive Home/Data and cache are owner-scoped: GetResourceUri builds
// their path from the owner's own PVC (GetPvcUser / GetPvcCache(owner)),
// so the owner is correctly admin over them. drive/Common is a
// cluster-wide shared volume gated by the owner's platform role.
//
// external (and internal/smb/usb/hdd) instead resolve to the shared
// EXTERNAL_PREFIX, which is NOT keyed by owner. Returning admin here
// assumes external-mount access is already gated upstream by node /
// device ownership. That assumption must be verified before CheckAccess
// is wired into a real call site for external resources.
// isPlatformAdmin is a seam over the global IntegrationService so the
// drive/Common role branch can be unit-tested without a live integration
// manager. Tests override it and restore in t.Cleanup.
var isPlatformAdmin = func(owner string) bool {
	return integration.IntegrationService != nil && integration.IntegrationService.IsPlatformAdmin(owner)
}

func (s *PosixStorage) CheckPermission(p *models.FileParam, owner string) (models.Level, error) {
	if p.IsDriveCommon() {
		if isPlatformAdmin(owner) {
			return models.LevelAdmin, nil
		}
		return models.LevelRead, nil
	}
	return models.LevelAdmin, nil
}

func (s *PosixStorage) ProbeExists(p *models.FileParam) error {
	if p == nil {
		return errors.New("file param is nil")
	}
	uri, err := p.GetResourceUri()
	if err != nil {
		return fmt.Errorf("resolve src uri: %w", err)
	}
	if !files.FilePathExists(uri + p.Path) {
		return fmt.Errorf("source not found: %s/%s%s", p.FileType, p.Extend, p.Path)
	}
	return nil
}

func (s *PosixStorage) ProbeIsDir(p *models.FileParam) (bool, error) {
	if p == nil {
		return false, errors.New("file param is nil")
	}
	uri, err := p.GetResourceUri()
	if err != nil {
		return false, fmt.Errorf("resolve uri: %w", err)
	}
	full := uri + p.Path
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("share target not found: %s/%s%s", p.FileType, p.Extend, p.Path)
		}
		return false, fmt.Errorf("stat share target %s: %w", full, err)
	}
	return info.IsDir(), nil
}

func (s *PosixStorage) ProbeWrite(dst *models.FileParam) error {
	if dst == nil {
		return errors.New("file param is nil")
	}
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

// probeParentDir returns full's parent with a trailing slash;
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

// RemoteStatusError lets callers wrap a non-2xx remote response with
// their own prefix; lookup / transport errors flow through unwrapped.
type RemoteStatusError struct {
	Code int
}

func (e *RemoteStatusError) Error() string { return fmt.Sprintf("remote status %d", e.Code) }

// logName ("stat" / "probe") is woven into error wrapping so callers
// keep the same klog/error text as before the helper was extracted.
func remoteRequest(p *models.FileParam, owner, query, logName string) (*http.Response, string, error) {
	if p == nil {
		return nil, "", errors.New("file param is nil")
	}
	ip, err := remoteLookupPodIP(p.Extend)
	if err != nil {
		return nil, "", err
	}
	url := fmt.Sprintf("http://%s/api/resources/%s/%s%s%s",
		ip, p.FileType, p.Extend, p.Path, query)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, url, fmt.Errorf("build remote %s request: %w", logName, err)
	}
	if owner != "" {
		req.Header.Set(common.REQUEST_HEADER_OWNER, owner)
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := remoteClient.Do(req)
	if err != nil {
		return nil, url, fmt.Errorf("remote %s %s: %w", logName, url, err)
	}
	return resp, url, nil
}

func drainRemote(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func RemoteExists(p *models.FileParam, owner string) error {
	resp, url, err := remoteRequest(p, owner, "", "stat")
	if err != nil {
		return err
	}
	defer drainRemote(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		klog.Warningf("[probe] remote stat %s returned %d", url, resp.StatusCode)
		return &RemoteStatusError{Code: resp.StatusCode}
	}
	return nil
}

func RemoteIsDir(p *models.FileParam, owner string) (bool, error) {
	resp, url, err := remoteRequest(p, owner, "", "stat")
	if err != nil {
		return false, err
	}
	defer drainRemote(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		klog.Warningf("[probe] remote stat %s returned %d", url, resp.StatusCode)
		return false, &RemoteStatusError{Code: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return false, fmt.Errorf("read remote stat response: %w", err)
	}
	var probe struct {
		IsDir bool `json:"isDir"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return false, fmt.Errorf("parse remote stat response: %w", err)
	}
	return probe.IsDir, nil
}

func RemoteProbeWrite(dst *models.FileParam) error {
	resp, url, err := remoteRequest(dst, dst.Owner, "?probe=write", "probe")
	if err != nil {
		return err
	}
	defer drainRemote(resp)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	klog.Warningf("[probe] remote probe %s returned %d", url, resp.StatusCode)
	return fmt.Errorf("destination not writable: %s/%s%s (remote status %d)",
		dst.FileType, dst.Extend, dst.Path, resp.StatusCode)
}

func (s *PosixStorage) List(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner
	var shareId = contextArgs.QueryParam.ShareId
	var shareNode, _ = url.PathUnescape(contextArgs.QueryParam.ShareNode)
	var sharePath = contextArgs.QueryParam.SharePath
	var shareFileType = contextArgs.QueryParam.ShareFileType
	var sharePermission = contextArgs.QueryParam.SharePermission
	var permission, _ = common.ParseInt(sharePermission)

	klog.Infof("Posix list, user: %s, args: %s, shareId: %s, sharePath: %s", owner, fileParam.Json(), shareId, sharePath)

	var (
		fileData *files.FileInfo
		err      error
	)
	if s.shouldUseFastExternalRootList(fileParam, shareId) {
		fileData, err = s.listExternalRootFast(fileParam)
	} else {
		fileData, err = s.getFiles(fileParam, Expand, Content)
	}
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			if shareId == "" {
				return nil, errors.New(common.ErrorMessageDirNotExists)
			} else {
				return nil, errors.New(common.ErrorMessageShareNotExists)
			}

		}
		return nil, err
	}

	if fileData.IsDir {
		if shareId != "" {

			fileData.FsType = common.Share
			fileData.FsExtend = shareId
			fileData.Path = sharePath
			fileData.Name = sharePath
			fileData.SharePermission = int32(permission)
			if sharePath == "/" {
				fileData.Name = ""
			}

			if shareFileType == common.External || shareFileType == common.Cache {
				fileData.FsExtend = fmt.Sprintf("%s_%s", shareId, shareNode)
			}

		}

		fileData.Listing.Sorting = files.DefaultSorting
		fileData.Listing.ApplySort()

		// Items lives on the embedded *Listing (nil for files).
		if len(fileData.Items) > 0 {
			if shareId != "" {
				for _, item := range fileData.Items {
					item.FsType = common.Share
					item.FsExtend = shareId
					item.SharePermission = int32(permission)
					if item.IsDir {
						item.Path = filepath.Join(sharePath, item.Name) + "/"
					} else {
						item.Path = filepath.Join(sharePath, item.Name)
					}

					if shareFileType == common.External || shareFileType == common.Cache {
						item.FsExtend = fmt.Sprintf("%s_%s", shareId, shareNode)
					}
				}
			}
		}
	}

	res, err := json.Marshal(fileData)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *PosixStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner

	klog.Infof("Posix preview, user: %s, args: %s", owner, common.ToJson(contextArgs))

	fileData, err := s.getFiles(fileParam, Expand, Content)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			return nil, errors.New("File not exist")
		}
		return nil, err
	}

	var previewFileName = fileParam.FileType + fileParam.Extend + fileData.Path + fileData.ModTime.String() + queryParam.PreviewSize
	var key = diskcache.GenerateCacheKey(previewFileName)

	klog.Infof("Posix preview, user: %s, fileType: %s, ext: %s, name: %s", owner, fileData.Type, fileData.Extension, fileData.Name)

	// get cache
	cachedData, ok, err := preview.GetPreviewCache(owner, key, common.CacheThumb)
	if err != nil {
		klog.Errorf("Posix preview, get cache failed, user: %s, error: %v", owner, err)

	} else if ok {

		klog.Infof("Posix preview, get cache, file: %s, cache name: %s, exists: %v", fileData.Path, previewFileName, ok)

		if cachedData != nil {
			return &models.PreviewHandlerResponse{
				FileName:     fileData.Name,
				FileModified: fileData.ModTime,
				Data:         cachedData,
			}, nil
		}
	}

	switch fileData.Type {
	case "image":
		data, err := preview.CreatePreview(owner, key, fileData, queryParam)
		if err != nil {
			return nil, err
		}
		return &models.PreviewHandlerResponse{
			FileName:     fileData.Name,
			FileModified: fileData.ModTime,
			Data:         data,
		}, nil
	default:
		return nil, fmt.Errorf("can't create preview for %s type", fileData.Type)
	}
}

func (s *PosixStorage) Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {

	var fileParam = contextArgs.FileParam
	var user = fileParam.Owner

	klog.Infof("Posix raw, user: %s, args: %s", user, common.ToJson(contextArgs))

	fileData, err := s.getFiles(fileParam, NoExpand, NoContent)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			return nil, errors.New("File not exist")
		}
		return nil, err
	}

	if fileData.IsDir {
		return nil, fmt.Errorf("not supported currently")
	}

	klog.Infof("Posix raw, file: %s", common.ToJson(fileData))

	r, err := getRawFile(fileData)
	if err != nil {
		return nil, err
	}

	return &models.RawHandlerResponse{
		FileName:     fileData.Name,
		FileModified: fileData.ModTime,
		Reader:       r,
		FileLength:   fileData.Size,
	}, nil
}

func (s *PosixStorage) Tree(contextArgs *models.HttpContextArgs, stopChan chan struct{}, dataChan chan string) error {
	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	klog.Infof("Posix tree, user: %s, args: %s", owner, fileParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return err
	}

	var fs = afero.NewBasePathFs(afero.NewOsFs(), resourceUri)

	fileData, err := s.getFiles(fileParam, Expand, NoContent)
	if err != nil {
		return err
	}

	if fileData.Listing == nil && fileData != nil {
		go func(stopChan <-chan struct{}, dataChan chan<- string) {
			defer close(dataChan)
			dataChan <- common.ToJson(fileData)

			select {
			case <-stopChan:
				return
			default:
			}

		}(stopChan, dataChan)
	} else {
		go s.generateListingData(fs, fileParam, fileData.Listing, stopChan, dataChan)
	}

	return nil
}

func (s *PosixStorage) Create(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return runWithExternalMountGuard(contextArgs.FileParam, "create", func() ([]byte, error) {
		var user = contextArgs.FileParam.Owner
		var fileParam = contextArgs.FileParam

		klog.Infof("Posix create, user: %s, param: %s", user, common.ToJson(contextArgs))

		dstFileOrDirName, isFile := files.GetFileNameFromPath(fileParam.Path)
		dstPrefixPath := files.GetPrefixPath(fileParam.Path)
		_, dstFileExt := common.SplitNameExt(dstFileOrDirName)

		resourceUri, err := contextArgs.FileParam.GetResourceUri()
		if err != nil {
			return nil, err
		}

		mode, err := strconv.ParseUint(contextArgs.QueryParam.FileMode, 8, 32)
		if err != nil {
			mode = 0755
		}

		fileMode := os.FileMode(mode)

		var afs = afero.NewOsFs()
		entries, err := afero.ReadDir(afs, filepath.Join(resourceUri, dstPrefixPath))
		if err != nil {
			klog.Errorf("Posix create read dir error: %v", err)
			entries = []os.FileInfo{}
		}

		var dupNames []string
		for _, entry := range entries {
			infoName := entry.Name()
			if isFile {
				_, infoExt := common.SplitNameExt(infoName)
				if infoExt != dstFileExt {
					continue
				}
				dupNames = append(dupNames, infoName)
			} else {
				if strings.Contains(infoName, dstFileOrDirName) {
					dupNames = append(dupNames, infoName)
				}
			}
		}

		klog.Infof("Posix create, dupNames %d: %+v", len(dupNames), dupNames)

		newName := files.GenerateDupName(dupNames, dstFileOrDirName, isFile)
		if newName == "" {
			newName = dstFileOrDirName
		}

		createPath := filepath.Join(resourceUri, dstPrefixPath, newName)
		klog.Infof("Posix create, create path: %s", createPath)

		if isFile {
			parentDir := filepath.Dir(createPath)
			if err = files.MkdirAllWithChown(afs, parentDir, fileMode, false, -1, -1); err != nil {
				klog.Errorf("Parent dir create error: %v", err)
				return nil, err
			}

			var file *os.File
			file, err = os.OpenFile(createPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
			if err != nil {
				klog.Errorf("File create error: %v", err)
				return nil, err
			}
			defer file.Close()

			if contextArgs.QueryParam.Body != nil {
				_, err = files.WriteFile(createPath, contextArgs.QueryParam.Body)
				if err != nil {
					klog.Errorf("File write error: %v", err)
					return nil, err
				}
			}

			uid, gid, e := files.GetUidGid(afs, parentDir)
			if e != nil {
				klog.Errorf("Get uid gid error: %v", e)
				return nil, e
			}
			if e = files.Chown(afs, createPath, uid, gid); e != nil {
				klog.Errorf("Chown file error: %v", e)
				return nil, e
			}
		} else {
			if !strings.HasSuffix(createPath, "/") {
				createPath += "/"
			}
			if err = files.MkdirAllWithChown(afs, createPath, fileMode, false, -1, -1); err != nil {
				klog.Errorf("Directory create error: %v", err)
				return nil, err
			}
		}

		return nil, nil
	})
}

func (s *PosixStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	var fileParam = fileDeleteArg.FileParam
	var dirents = fileDeleteArg.Dirents
	var user = fileParam.Owner

	var err error
	var deleteFailedPaths []string

	if len(dirents) == 0 {
		return nil, fmt.Errorf("dirents is empty")
	}

	fileData, err := s.getFiles(fileParam, NoExpand, NoContent)
	if err != nil {
		klog.Errorf("Posix delete, get file data error: %s, user: %s, path: %s", err, user, fileParam.Path)
		return nil, fmt.Errorf("%s: no such file or directory", fileParam.Path)
	}

	var invalidPaths []string

	for _, dirent := range dirents {
		d := strings.TrimSpace(dirent)
		if d == "" || d == "/" || !strings.HasPrefix(d, "/") {
			invalidPaths = append(invalidPaths, dirent)
			break
		}
	}

	if len(invalidPaths) > 0 {
		return common.ToBytes(invalidPaths), fmt.Errorf("invalid path")
	}

	_, err = runWithExternalMountGuard(fileParam, "delete_remove", func() (struct{}, error) {
		for _, dirent := range dirents {
			d := strings.TrimSpace(dirent)
			direntPath := fileData.Path + strings.TrimLeft(d, "/")

			// Pre-stat so missing dirents surface as errors instead of
			// RemoveAll's silent success. Lstat keeps dangling symlinks deletable.
			var statErr error
			if lstater, ok := fileData.Fs.(afero.Lstater); ok {
				_, _, statErr = lstater.LstatIfPossible(direntPath)
			} else {
				_, statErr = fileData.Fs.Stat(direntPath)
			}
			if statErr != nil {
				klog.Errorf("Posix delete, stat dirent error: %v, user: %s, path: %s", statErr, user, direntPath)
				deleteFailedPaths = append(deleteFailedPaths, dirent)
				continue
			}

			klog.Infof("Posix delete, remove dirent path: %s", direntPath)
			if err = fileData.Fs.RemoveAll(direntPath); err != nil {
				klog.Errorf("Posix delete, remove path error: %v, user: %s, path: %s", err, user, direntPath)
				deleteFailedPaths = append(deleteFailedPaths, dirent)
			}
		}
		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}

	// data shape matches sync/cloud: list of failed dirents + fixed message.
	// Per-item reasons stay in klog above.
	if len(deleteFailedPaths) > 0 {
		return common.ToBytes(deleteFailedPaths), fmt.Errorf("delete failed paths")
	}

	return nil, nil
}

func (s *PosixStorage) Rename(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return runWithExternalMountGuard(contextArgs.FileParam, "rename", func() ([]byte, error) {
		var fileParam = contextArgs.FileParam
		var owner = fileParam.Owner

		if fileParam.Path == "/" {
			return nil, fmt.Errorf("path invalid, path: %s", fileParam.Path)
		}

		var uri, err = fileParam.GetResourceUri()
		if err != nil {
			return nil, err
		}

		klog.Infof("Posix rename, user: %s, uri: %s, args: %s", owner, uri, common.ToJson(contextArgs))

		var srcName, isSrcFile = files.GetFileNameFromPath(fileParam.Path)
		srcName, err = url.PathUnescape(srcName)
		if err != nil {
			return nil, err
		}
		dstName, err := url.PathUnescape(contextArgs.QueryParam.Destination) // no /
		if err != nil {
			return nil, err
		}

		klog.Infof("Posix rename, user: %s, uri: %s, isFile: %v, src: %s, dst: %s, args: %s", owner, uri, isSrcFile, srcName, dstName, common.ToJson(contextArgs))

		if srcName == dstName {
			return nil, nil
		}

		// NOTE: previously this block computed srcFilenamePrefix /
		// dstFilenamePrefix (name minus extension) for filename
		// collision handling, but the values were never consumed by
		// the rest of the function. Removed to keep the lint baseline
		// clean. If extension-aware collision rename is added later,
		// reintroduce these locals together with the consumer.

		var afs = afero.NewOsFs()
		var srcPrefixPath = files.GetPrefixPath(fileParam.Path)
		file, err := files.NewFileInfo(files.FileOptions{
			Fs:       afero.NewBasePathFs(afs, uri),
			FsType:   fileParam.FileType,
			FsExtend: fileParam.Extend,
			Path:     srcPrefixPath,
			Expand:   Expand,
			Content:  NoContent,
		})
		if err != nil {
			return nil, err
		}

		if file == nil || file.Items == nil || len(file.Items) == 0 {
			return nil, fmt.Errorf("file %s not exists", fileParam.Path)
		}

		var existsName bool
		for _, item := range file.Items {
			if item.Name == dstName {
				existsName = true
				break
			}
		}

		if existsName {
			return nil, fmt.Errorf("The name '%s' already exists. Please choose another name.", dstName)
		}

		var rSrcPath = uri + fileParam.Path
		var rDstPath = uri + srcPrefixPath + dstName

		klog.Infof("Posix rename, src: %s, dst: %s", rSrcPath, rDstPath)

		if err = afs.Rename(rSrcPath, rDstPath); err != nil {
			return nil, common.SanitizeFsError(err)
		}

		return nil, nil
	})
}

func (s *PosixStorage) Edit(contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error) {
	return runWithExternalMountGuard(contextArgs.FileParam, "edit", func() (*models.EditHandlerResponse, error) {
		var fileParam = contextArgs.FileParam
		var user = fileParam.Owner

		klog.Infof("Posix edit, user: %s, path: %s, param: %s", user, fileParam.Path, common.ParseString(fileParam))

		fileName, isFile := files.GetFileNameFromPath(fileParam.Path)
		if !isFile {
			return nil, fmt.Errorf("path %s is not file", fileParam.Path)
		}
		_ = fileName

		uri, err := fileParam.GetResourceUri()
		if err != nil {
			return nil, err
		}

		filePath := uri + fileParam.Path

		klog.Infof("Posix edit, user: %s, file path: %s", user, filePath)

		exists := files.FilePathExists(filePath)
		if !exists {
			return nil, fmt.Errorf("file %s not exists", fileParam.Path)
		}

		info, err := files.WriteFile(filePath, contextArgs.QueryParam.Body)
		if err != nil {
			klog.Errorf("Posix edit, write file %s failed: %v", filePath, err)
			return nil, err
		}
		etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())

		return &models.EditHandlerResponse{
			Etag: etag,
		}, nil
	})
}

func (s *PosixStorage) generateListingData(fs afero.Fs, fileParam *models.FileParam, listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string) {
	defer close(dataChan)

	var streamFiles []*files.FileInfo
	if listing != nil && len(listing.Items) > 0 {
		streamFiles = append(streamFiles, listing.Items...)
	}

	for len(streamFiles) > 0 {
		firstItem := streamFiles[0]

		if firstItem.IsDir {
			var nestFileParam = &models.FileParam{
				Owner:    fileParam.Owner,
				FileType: fileParam.FileType,
				Extend:   fileParam.Extend,
				Path:     firstItem.Path,
			}

			nestFileData, err := s.getFiles(nestFileParam, Expand, NoContent)
			if err != nil {
				klog.Error(err)
				return
			}

			var nestedItems []*files.FileInfo
			if nestFileData.Listing != nil {
				nestedItems = append(nestedItems, nestFileData.Listing.Items...)
			}
			streamFiles = append(nestedItems, streamFiles[1:]...)
		} else {
			dataChan <- common.ToJson(firstItem)
			streamFiles = streamFiles[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func (s *PosixStorage) isExternal(fileType string, extend string) bool {
	return (fileType == common.External || fileType == common.Usb || fileType == common.Hdd || fileType == common.Internal || fileType == common.Smb) && extend != ""
}

func getExternalMountName(path string) (string, bool) {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return "", false
	}
	parts := strings.SplitN(trimmed, "/", 2)
	if parts[0] == "" {
		return "", false
	}
	return parts[0], true
}

func (s *PosixStorage) shouldUseFastExternalRootList(fileParam *models.FileParam, shareId string) bool {
	if shareId != "" || fileParam == nil || fileParam.FileType != common.External {
		return false
	}
	_, hasMountName := getExternalMountName(fileParam.Path)
	return !hasMountName
}

// listExternalRootFast lists External root directory entries without
// deep per-entry filesystem inspection, so a stale mountpoint cannot
// block the whole root listing response.
func (s *PosixStorage) listExternalRootFast(fileParam *models.FileParam) (*files.FileInfo, error) {
	resourceURI, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	targetPath := filepath.Join(resourceURI, strings.TrimPrefix(fileParam.Path, "/"))
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, err
	}

	rootPath := fileParam.Path
	if rootPath == "" {
		rootPath = "/"
	}
	root := &files.FileInfo{
		FsType:   fileParam.FileType,
		FsExtend: fileParam.Extend,
		Path:     rootPath,
		Name:     rootPath,
		IsDir:    true,
		Listing: &files.Listing{
			Items:   make([]*files.FileInfo, 0, len(entries)),
			Sorting: files.DefaultSorting,
		},
	}
	if rootPath == "/" {
		root.Name = ""
	}

	for _, entry := range entries {
		modeType := entry.Type()
		// Use dirent type bits only; avoid entry.Info()/Stat on each child.
		isDir := modeType.IsDir() || modeType == 0

		itemPath := filepath.ToSlash(filepath.Join(rootPath, entry.Name()))
		if !strings.HasPrefix(itemPath, "/") {
			itemPath = "/" + itemPath
		}
		if isDir && !strings.HasSuffix(itemPath, "/") {
			itemPath += "/"
		}

		item := &files.FileInfo{
			FsType:   fileParam.FileType,
			FsExtend: fileParam.Extend,
			Path:     itemPath,
			Name:     entry.Name(),
			IsDir:    isDir,
		}
		if mounted, ok := global.GlobalMounted.GetMountedByPath(entry.Name()); ok && mounted.Type != "" {
			item.ExternalType = mounted.Type
		} else {
			item.ExternalType = global.GlobalMounted.CheckExternalType(item.Path, item.IsDir)
		}

		if item.IsDir {
			root.Listing.NumDirs++
		} else {
			root.Listing.NumFiles++
		}
		root.Listing.Items = append(root.Listing.Items, item)
	}
	root.Listing.NumTotalFiles = len(root.Listing.Items)

	return root, nil
}

func (s *PosixStorage) getFiles(fileParam *models.FileParam, expand, content bool) (*files.FileInfo, error) {
	return runWithExternalMountGuard(fileParam, "get_files", func() (*files.FileInfo, error) {
		var resourceUri, err = fileParam.GetResourceUri()
		if err != nil {
			return nil, err
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:       afero.NewBasePathFs(afero.NewOsFs(), resourceUri),
			FsType:   fileParam.FileType,
			FsExtend: fileParam.Extend,
			Path:     fileParam.Path,
			Expand:   expand,
			Content:  content,
		})
		if err != nil {
			return nil, err
		}

		if file == nil {
			return nil, fmt.Errorf("file %s not exists", fileParam.Path)
		}

		if s.isExternal(fileParam.FileType, fileParam.Extend) {
			// klog.Infof("getFiles fileType: %s, extend: %s", fileParam.FileType, fileParam.Extend)
			file.ExternalType = global.GlobalMounted.CheckExternalType(file.Path, file.IsDir)
			if file.IsDir && file.Listing != nil {
				for _, f := range file.Items {
					f.ExternalType = global.GlobalMounted.CheckExternalType(f.Path, f.IsDir)
				}
			}
		}

		return file, nil
	})
}

/**
 * UploadLink
 */
func (s *PosixStorage) UploadLink(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return runWithExternalMountGuard(fileUploadArg.FileParam, "upload_link", func() ([]byte, error) {
		var user = fileUploadArg.FileParam.Owner
		var node = fileUploadArg.Node
		var from = fileUploadArg.From

		klog.Infof("Posix uploadLink, user: %s, node: %s, from: %s, share: %s %s %s, param: %s, totalSize: %dB",
			user, node, from,
			fileUploadArg.Share, fileUploadArg.ShareType, fileUploadArg.ShareBy,
			common.ToJson(fileUploadArg.FileParam), fileUploadArg.TotalSize)

		if fileUploadArg.TotalSize != 0 {
			if fileUploadArg.FileParam.IsSystem() {
				_, err := common.CheckDiskSpace(common.RootPrefix, fileUploadArg.TotalSize, true)
				if err != nil {
					return nil, err
				}
			} else if fileUploadArg.FileParam.FileType == common.External {
				diskCheckPath, err := fileUploadArg.FileParam.GetDiskCheckPath()
				if err != nil {
					return nil, fmt.Errorf("get disk check path error: %v", err)
				}
				_, err = common.CheckDiskSpace(diskCheckPath, fileUploadArg.TotalSize, false)
				if err != nil {
					return nil, err
				}
			}
		}

		data, err := upload.HandleUploadLink(fileUploadArg.FileParam, fileUploadArg.From)

		klog.Infof("Posix uploadLink, done! data: %s", string(data))

		return data, err
	})
}

/**
 * UploadedBytes
 */
func (s *PosixStorage) UploadedBytes(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return runWithExternalMountGuard(fileUploadArg.FileParam, "uploaded_bytes", func() ([]byte, error) {
		var node = fileUploadArg.Node
		var user = fileUploadArg.FileParam.Owner
		var identy = fileUploadArg.Identy
		var ua = fileUploadArg.UserAgentHash

		klog.Infof("Posix uploadBytes, user: %s, node: %s, identy: %s, ua: %s, share: %s %s, param: %s", user, node, identy, ua, fileUploadArg.Share, fileUploadArg.ShareType, common.ToJson(fileUploadArg.FileParam))

		data, err := upload.HandleUploadedBytes(fileUploadArg.FileParam, fileUploadArg.FileName, identy, ua)

		klog.Infof("Posix uploadBytes, done! data: %s", string(data))

		return data, err
	})
}

/**
 * UploadChunks
 */
func (s *PosixStorage) UploadChunks(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return runWithExternalMountGuard(fileUploadArg.FileParam, "upload_chunks", func() ([]byte, error) {
		var user = fileUploadArg.FileParam.Owner
		var chunkInfo = fileUploadArg.ChunkInfo
		var uploadId = fileUploadArg.UploadId
		var identy = chunkInfo.ResumableIdenty
		var ua = fileUploadArg.UserAgentHash

		klog.Infof("Posix uploadChunks, user: %s, uploadId: %s, identy: %s, ua: %s, param: %s, parentDir: %s, share: %s %s", user, uploadId, identy, ua, common.ToJson(fileUploadArg.FileParam), chunkInfo.ParentDir, chunkInfo.Share, chunkInfo.Shareby)
		//klog.Infof("Posix uploadChunks, user: %s, uploadId: %s, identy: %s, ua: %s, param: %s, parentDir: %s, share: %s %s, disk usage: %f", user, uploadId, identy, ua, common.ToJson(fileUploadArg.FileParam), chunkInfo.ParentDir, chunkInfo.Share, chunkInfo.Shareby, global.GlobalMounted.Usage)
		//
		//if global.GlobalMounted.Usage >= common.FreeLimit {
		//	return nil, errors.New(common.ErrorMessageNoSpace)
		//}

		_, fileInfo, err := upload.HandleUploadChunks(fileUploadArg.FileParam, fileUploadArg.UploadId, *chunkInfo, ua, fileUploadArg.Ranges)

		if err != nil {
			return nil, err
		}

		if fileInfo == nil {
			return common.ToBytes(&upload.FileUploadSucced{Success: true}), nil
		}

		// Large file: run MoveFileByInfo asynchronously via a task so the
		// HTTP response is sent before the platform proxy timeout fires.
		if fileInfo.FileInfo != nil && fileInfo.FileInfo.FileSize >= common.AsyncFinalizeThreshold {
			taskFileParam := &models.FileParam{
				Owner:    fileUploadArg.FileParam.Owner,
				FileType: fileUploadArg.FileParam.FileType,
				Extend:   fileUploadArg.FileParam.Extend,
				Path:     fileUploadArg.FileParam.Path + chunkInfo.ResumableRelativePath,
			}
			taskDisplayParam := taskFileParam
			if chunkInfo.Share != "" && chunkInfo.SharebyPath != "" {
				if sp, err := models.CreateFileParam(user, chunkInfo.SharebyPath+chunkInfo.ResumableRelativePath); err == nil {
					taskDisplayParam = sp
				}
			}
			uploadParam := &models.PasteParam{
				Owner:  user,
				Action: common.ActionUploadFinalize,
				Src:    taskDisplayParam,
				Dst:    taskDisplayParam,
			}
			task := tasks.TaskManager.CreateTask(uploadParam)
			task.SetTotalSize(fileInfo.FileInfo.FileSize)
			task.ExecuteAsync(task.UploadFinalizePosix(&tasks.PosixFinalizeParams{
				Info:            *fileInfo.FileInfo,
				UploadTempPath:  fileInfo.UploadTempPath,
				InnerIdentifier: fileInfo.Id,
				FileParam:       fileUploadArg.FileParam,
				ResumableInfo:   chunkInfo,
			}))
			fileInfo.TaskId = task.Id()
			klog.Infof("Posix uploadChunks, large file, async finalize task: %s", fileInfo.TaskId)
		}

		klog.Infof("Posix uploadChunks, done! data: %s", common.ToJson(fileInfo))

		var result []*upload.FileUploadState
		result = append(result, fileInfo)

		return common.ToBytes(result), err
	})
}
