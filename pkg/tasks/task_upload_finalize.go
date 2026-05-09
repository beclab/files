package tasks

import (
	"bytes"
	"encoding/json"
	"errors"
	"files/pkg/drivers/posix/upload"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/files"
	"files/pkg/models"
	uploadwh "files/pkg/webhook/upload"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"k8s.io/klog/v2"
)

// PosixFinalizeParams holds everything the async posix finalize goroutine
// needs after the HTTP handler has returned.
type PosixFinalizeParams struct {
	Info            upload.FileInfo
	UploadTempPath  string
	InnerIdentifier string
	FileParam       *models.FileParam
	ResumableInfo   *models.ResumableInfo
}

// UploadFinalizePosix returns a phase function that moves the assembled
// temp file to its final destination and cleans up metadata.
func (t *Task) UploadFinalizePosix(p *PosixFinalizeParams) func() error {
	return func() error {
		klog.Infof("[Task] Id: %s, UploadFinalizePosix start, src: %s, dst: %s, size: %d",
			t.id, p.UploadTempPath, p.Info.FullPath, p.Info.FileSize)

		onProgress := files.ProgressFunc(func(written int64) {
			if p.Info.FileSize > 0 {
				pct := int(written * 99 / p.Info.FileSize)
				if pct > 99 {
					pct = 99
				}
				t.mu.Lock()
				t.progress = pct
				t.transfer = written
				t.mu.Unlock()
			}
		})

		if err := upload.MoveFileByInfo(p.Info, p.UploadTempPath, onProgress); err != nil {
			klog.Errorf("[Task] Id: %s, UploadFinalizePosix move failed: %v", t.id, err)
			return err
		}

		upload.FileInfoManager.DelFileInfo(p.InnerIdentifier, p.InnerIdentifier, p.UploadTempPath)

		uploadwh.Enqueue(uploadwh.NewItem{
			Path:        uploadwh.BuildFrontendPath(p.FileParam, p.ResumableInfo),
			StoragePath: p.Info.FullPath,
			Mime:        uploadwh.GuessMime(p.Info.FullPath),
			UploadedAt:  uploadwh.NowMillis(),
			Owner:       t.param.Owner,
		})

		klog.Infof("[Task] Id: %s, UploadFinalizePosix completed", t.id)
		return nil
	}
}

// SyncFinalizeParams holds everything the async sync finalize goroutine
// needs to proxy the last chunk to seafile after the HTTP handler returns.
type SyncFinalizeParams struct {
	BodyBytes       []byte
	Headers         map[string]string
	UploadType      string
	UID             string
	OriginalUID     string
	Owner           string
	UploadReq       SyncFinalizeUploadReq
}

// SyncFinalizeUploadReq is a minimal snapshot of the upload request fields
// needed for token-refresh retry logic inside the async goroutine.
type SyncFinalizeUploadReq struct {
	DriveType          string
	RepoId             string
	ParentDir          string
	ResumableFilename  string
}

// UploadFinalizeSync returns a phase function that proxies the last chunk
// request to seafile and waits for its response in the background.
func (t *Task) UploadFinalizeSync(p *SyncFinalizeParams) func() error {
	return func() error {
		klog.Infof("[Task] Id: %s, UploadFinalizeSync start, uid: %s", t.id, p.UID)

		var done atomic.Bool
		var maxPct atomic.Int32
		maxPct.Store(95)
		go t.simulateProgress(&done, &maxPct, t.totalSize)

		executeRequest := func(uid string) (*http.Response, error) {
			reqUrl := fmt.Sprintf("http://seafile:8082/%s/%s?ret-json=1", p.UploadType, uid)
			klog.Infof("[Task] Id: %s, UploadFinalizeSync request: %s", t.id, reqUrl)

			req, err := http.NewRequest(http.MethodPost, reqUrl, bytes.NewReader(p.BodyBytes))
			if err != nil {
				return nil, err
			}
			for k, v := range p.Headers {
				req.Header.Set(k, v)
			}
			if req.Header.Get("Content-Disposition") == "" {
				req.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", url.PathEscape(p.UploadReq.ResumableFilename)))
			}
			req.ContentLength = int64(len(p.BodyBytes))

			return http.DefaultClient.Do(req)
		}

		closeBody := func(r *http.Response) {
			if r != nil && r.Body != nil {
				r.Body.Close()
			}
		}

		readBody := func(r *http.Response) []byte {
			if r == nil || r.Body == nil {
				return nil
			}
			b, _ := io.ReadAll(r.Body)
			return b
		}

		pollWait := func() error {
			maxPct.Store(99)
			waitErr := t.waitForSeafileCommit(
				p.UploadReq.RepoId,
				p.UploadReq.ParentDir,
				p.UploadReq.ResumableFilename,
				30*time.Minute,
			)
			done.Store(true)
			seahub.DeleteAccessToken(p.OriginalUID)
			return waitErr
		}

		resp, err := executeRequest(p.UID)

		if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
			if isSeafileTimeout(err, resp) {
				klog.Infof("[Task] Id: %s, seafile timeout/reset detected, entering poll-wait", t.id)
				closeBody(resp)
				return pollWait()
			}

			retryNeeded := false
			var firstBody []byte
			if err != nil {
				retryNeeded = isConnectionReset(err)
			} else {
				firstBody = readBody(resp)
				var errMsg struct{ Error string `json:"error"` }
				_ = json.Unmarshal(firstBody, &errMsg)
				retryNeeded = errMsg.Error == "Access token not found."
			}
			closeBody(resp)

			if !retryNeeded {
				done.Store(true)
				seahub.DeleteAccessToken(p.OriginalUID)
				if err != nil {
					return fmt.Errorf("seafile request failed: %v", err)
				}
				return fmt.Errorf("seafile returned status %d: %s", resp.StatusCode, string(firstBody))
			}

			trimmed := strings.Trim(p.UploadReq.ParentDir, "/")
			path := "/" + p.UploadReq.DriveType + "/" + p.UploadReq.RepoId + "/" + trimmed + "/"
			fileParam, fpErr := models.CreateFileParam(p.Owner, path)
			if fpErr != nil {
				done.Store(true)
				return fmt.Errorf("create file param for token refresh: %v", fpErr)
			}
			newUid, refreshErr := seahub.GetUploadLink(fileParam, "web", false, true)
			if refreshErr != nil {
				seahub.DeleteAccessToken(p.OriginalUID)
				done.Store(true)
				return fmt.Errorf("token refresh failed: %v", refreshErr)
			}
			seahub.SetAccessToken(p.OriginalUID, newUid)

			resp, err = executeRequest(newUid)
			if err != nil {
				closeBody(resp)
				if isSeafileTimeout(err, resp) {
					klog.Infof("[Task] Id: %s, retry also timed out, entering poll-wait", t.id)
					return pollWait()
				}
				seahub.DeleteAccessToken(p.OriginalUID)
				done.Store(true)
				return fmt.Errorf("retry after token refresh failed: %v", err)
			}
		}

		defer closeBody(resp)
		done.Store(true)

		if resp.StatusCode != http.StatusOK {
			bodyRes := readBody(resp)
			seahub.DeleteAccessToken(p.OriginalUID)
			return fmt.Errorf("seafile returned status %d: %s", resp.StatusCode, string(bodyRes))
		}

		bodyRes := readBody(resp)
		klog.Infof("[Task] Id: %s, UploadFinalizeSync seafile response: %s", t.id, string(bodyRes))
		seahub.DeleteAccessToken(p.OriginalUID)
		return nil
	}
}

// simulateProgress drives progress/transferred forward at an estimated rate
// until done is set to true. maxPct can be adjusted dynamically (e.g. raised
// from 95 to 99 when entering the 504 poll-wait phase).
func (t *Task) simulateProgress(done *atomic.Bool, maxPct *atomic.Int32, totalSize int64) {
	if totalSize <= 0 {
		return
	}

	const estimatedSpeed int64 = 10 * 1024 * 1024 // 10 MB/s
	const tickInterval = 2 * time.Second

	bytesPerTick := estimatedSpeed * int64(tickInterval/time.Second)

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	var simulated int64
	for range ticker.C {
		if done.Load() {
			return
		}
		cap := int(maxPct.Load())
		maxTransferred := totalSize * int64(cap) / 100

		simulated += bytesPerTick
		if simulated > maxTransferred {
			simulated = maxTransferred
		}
		pct := int(simulated * 100 / totalSize)
		if pct > cap {
			pct = cap
		}
		t.mu.Lock()
		t.progress = pct
		t.transfer = simulated
		t.mu.Unlock()
	}
}

// isSeafileTimeout returns true when the error or response indicates a
// gateway timeout (504) or a connection-level timeout (EOF, reset) that
// suggests seafile is still processing the file in the background.
func isSeafileTimeout(err error, resp *http.Response) bool {
	if resp != nil && resp.StatusCode == http.StatusGatewayTimeout {
		return true
	}
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return true
		}
		msg := err.Error()
		return strings.Contains(msg, "connection reset by peer") ||
			strings.Contains(msg, "i/o timeout") ||
			strings.Contains(msg, "context deadline exceeded")
	}
	return false
}

// waitForSeafileCommit polls GetFileIdByPath until the file appears in
// seafile or maxWait is exceeded.
func (t *Task) waitForSeafileCommit(repoId, parentDir, filename string, maxWait time.Duration) error {
	filePath := strings.TrimRight(parentDir, "/") + "/" + filename
	klog.Infof("[Task] Id: %s, waitForSeafileCommit start, repo: %s, path: %s, maxWait: %v",
		t.id, repoId, filePath, maxWait)

	const pollInterval = 10 * time.Second
	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		fileId, err := seaserv.GlobalSeafileAPI.GetFileIdByPath(repoId, filePath)
		if err != nil {
			klog.Warningf("[Task] Id: %s, waitForSeafileCommit poll error: %v", t.id, err)
		}
		if fileId != "" {
			klog.Infof("[Task] Id: %s, waitForSeafileCommit file committed, fileId: %s", t.id, fileId)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("seafile did not commit file within %v", maxWait)
		}
		klog.Infof("[Task] Id: %s, waitForSeafileCommit still waiting...", t.id)
	}
	return fmt.Errorf("waitForSeafileCommit ticker stopped unexpectedly")
}

func isConnectionReset(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "connection reset")
}
