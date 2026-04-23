// Package upload notifies an upload-completion webhook whenever a file
// finishes uploading via the local (posix) backend.
//
// Note: the webhook target is a separate service that happens to be
// co-located on the same node as terminusd. We reuse the TERMINUSD_HOST
// env var only to discover that node's IP; the port (18832) and path
// (/v1/new_items) are owned by the webhook service, not terminusd.
//
// Design summary:
//   - Disabled by default; gated by env var UPLOAD_WEBHOOK_ENABLED.
//   - Producers call Enqueue(item), which is a non-blocking select send onto
//     a bounded channel. The upload request path is never blocked.
//   - A single background worker drains the channel, opportunistically batches
//     up to maxBatchSize items per outgoing request, and POSTs them to
//     http://<host_ip>:18832/v1/new_items.
//   - No per-request timeout (intentional). Bounded memory and exactly one
//     in-flight request guarantee resource bounds even when the webhook is
//     slow. On any failure (network error, non-2xx, marshal error) the batch
//     is dropped with a warning; there is no retry.
package upload

import (
	"bytes"
	"encoding/json"
	"files/pkg/models"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/klog/v2"
)

const (
	envEnabled  = "UPLOAD_WEBHOOK_ENABLED"
	envHost     = "TERMINUSD_HOST" // borrowed only for the co-located node IP
	notifyPort  = "18832"
	notifyPath  = "/v1/new_items"
	queueCap    = 4096
	maxBatchSiz = 100
	logPrefix   = "[upload-webhook]"
)

// NewItem is the metadata for a single newly created file.
type NewItem struct {
	Path        string `json:"path"`         // frontend logical path, e.g. /drive/Home/photos/IMG.jpg
	StoragePath string `json:"storage_path"` // backend on-disk path, e.g. /data/<pvc>/Home/photos/IMG.jpg
	Mime        string `json:"mime"`
	UploadedAt  int64  `json:"uploaded_at"` // unix milliseconds
	Owner       string `json:"owner"`
}

type newItemsBody struct {
	Items []NewItem `json:"items"`
}

var (
	queue      chan NewItem
	startOnce  sync.Once
	httpClient = &http.Client{} // intentionally no Timeout; keep-alive enabled by default transport
	dropped    uint64           // atomic counter; updated when the queue is full
)

func enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envEnabled))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// Start launches the single background worker. Idempotent. No-op when the
// feature flag is not set to a truthy value.
func Start() {
	if !enabled() {
		klog.Infof("%s notify disabled (set %s=true to enable)", logPrefix, envEnabled)
		return
	}
	startOnce.Do(func() {
		queue = make(chan NewItem, queueCap)
		go runWorker()
		klog.Infof("%s notify enabled, queueCap=%d, maxBatchSize=%d, url=%s",
			logPrefix, queueCap, maxBatchSiz, notifyURL())
	})
}

// Enqueue is non-blocking. Drops the item (with a throttled warning) when
// either the feature is disabled or the queue is full. This call is safe to
// invoke from any request goroutine and is guaranteed not to block.
func Enqueue(item NewItem) {
	if queue == nil {
		return
	}
	select {
	case queue <- item:
	default:
		n := atomic.AddUint64(&dropped, 1)
		// Throttle the warning so a sustained outage doesn't flood logs.
		if n == 1 || n%100 == 0 {
			klog.Warningf("%s queue full (cap=%d), dropped item path=%s total_dropped=%d",
				logPrefix, queueCap, item.Path, n)
		}
	}
}

func runWorker() {
	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("%s worker panic: %v; restarting", logPrefix, r)
			go runWorker()
		}
	}()

	batch := make([]NewItem, 0, maxBatchSiz)
	for first := range queue {
		batch = append(batch[:0], first)

	drain:
		for len(batch) < maxBatchSiz {
			select {
			case it := <-queue:
				batch = append(batch, it)
			default:
				break drain
			}
		}
		post(batch)
	}
}

func post(items []NewItem) {
	if os.Getenv(envHost) == "" {
		klog.Warningf("%s %s empty, skip %d item(s)", logPrefix, envHost, len(items))
		return
	}

	body, err := json.Marshal(newItemsBody{Items: items})
	if err != nil {
		klog.Warningf("%s marshal failed: %v (skip %d item(s))", logPrefix, err, len(items))
		return
	}

	url := notifyURL()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		klog.Warningf("%s build request failed: %v", logPrefix, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		klog.Warningf("%s POST %s failed: %v (dropped %d item(s))", logPrefix, url, err, len(items))
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 300 {
		klog.Warningf("%s POST %s status=%d (dropped %d item(s))", logPrefix, url, resp.StatusCode, len(items))
		return
	}
	klog.Infof("%s POST %s ok, items=%d", logPrefix, url, len(items))
}

// notifyURL builds http://<ip>:18832/v1/new_items by extracting the host
// portion of TERMINUSD_HOST (which is normally "host:port" with no scheme)
// and forcing port 18832. The webhook service is a separate process from
// terminusd; we only borrow terminusd's host for the IP.
func notifyURL() string {
	host := os.Getenv(envHost)
	if h, _, err := net.SplitHostPort(host); err == nil && h != "" {
		host = h
	}
	return "http://" + net.JoinHostPort(host, notifyPort) + notifyPath
}

// BuildFrontendPath reconstructs the frontend logical path that mirrors what
// models.CreateFileParam would have parsed (i.e. the inverse of
// (*FileParam).convert). For share uploads, we re-derive from (Shareby,
// ParentDir) so the returned path resolves the same tree as info.FullPath
// (which is share-rewritten in handlefunc.go before MoveFileByInfo).
func BuildFrontendPath(fp *models.FileParam, ri *models.ResumableInfo) string {
	if fp == nil || ri == nil {
		return ""
	}
	fileType := fp.FileType
	extend := fp.Extend
	base := fp.Path
	if ri.Share != "" && ri.Shareby != "" {
		if shared, err := models.CreateFileParam(ri.Shareby, ri.ParentDir); err == nil {
			fileType, extend, base = shared.FileType, shared.Extend, shared.Path
		}
	}
	return path.Join("/", fileType, extend, base, ri.ResumableRelativePath)
}

// GuessMime returns a best-effort MIME type derived from the filename
// extension. We deliberately do not use the multipart chunk's Content-Type
// header because that's typically application/octet-stream.
func GuessMime(fullPath string) string {
	ext := strings.ToLower(filepath.Ext(fullPath))
	if ext == "" {
		return "application/octet-stream"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		// Strip any "; charset=..." parameters for a cleaner value.
		if idx := strings.Index(t, ";"); idx >= 0 {
			t = strings.TrimSpace(t[:idx])
		}
		return t
	}
	return "application/octet-stream"
}

// NowMillis returns the current unix time in milliseconds. Exposed so callers
// can compute the timestamp at the precise moment the file is committed to
// disk rather than when this package finally posts it.
func NowMillis() int64 { return time.Now().UnixMilli() }
