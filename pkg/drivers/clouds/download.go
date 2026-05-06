package clouds

import (
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"path"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type Download struct {
	ctx            context.Context
	service        *service
	fileParam      *models.FileParam
	fileName       string
	filePath       string
	fileSize       int64
	fileTargetPath string
}

func NewDownloader(ctx context.Context, service *service, fileParam *models.FileParam, fileName string, filePath string, fileSize int64, fileTargetPath string) *Download {

	return &Download{
		ctx:            ctx,
		service:        service,
		fileParam:      fileParam,
		fileName:       fileName,
		filePath:       filePath,
		fileSize:       fileSize,
		fileTargetPath: fileTargetPath,
	}
}

func (d *Download) download() error {
	var owner = d.fileParam.Owner
	if err := d.checkCtx(); err != nil {
		return err
	}

	_, err := common.CheckDiskSpace(common.RootPrefix, d.fileSize, true) // d.checkFreeDiskSpace()
	if err != nil {
		return err
	}

	klog.Infof("Cloud download, owner: %s, param: %s, folder: %s, file: %s", owner, common.ToJson(d.fileParam), d.fileTargetPath, d.fileName)

	type asyncResult struct {
		jobId int
		err   error
	}

	// Buffered so the goroutine never blocks on send, even if we already
	// returned via <-d.ctx.Done() and no one is reading. Do NOT close(ch)
	// here: closing while the goroutine is still about to send would panic
	// the entire process. Letting GC reclaim the channel is safe.
	ch := make(chan asyncResult, 1)
	go func() {
		res, err := d.service.DownloadAsync(d.fileParam, d.fileTargetPath, d.fileName)
		ch <- asyncResult{jobId: res, err: err}
	}()

	var resp asyncResult
	select {
	case <-d.ctx.Done():
		return d.ctx.Err()
	case resp = <-ch:
	}

	if err := d.checkCtx(); err != nil {
		return err
	}

	if resp.err != nil {
		return resp.err
	}

	var ticker = time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-ticker.C:
		}

		res, err := d.service.QueryJob(resp.jobId)
		if err != nil {
			klog.Errorf("Cloud download, query job error: %v", err)
			return err
		}

		klog.Infof("Cloud download task status, user: %s, file: %s, id: %d, status:%s", owner, d.fileName, resp.jobId, common.ToJson(res))

		if res.Finished {
			return nil
		}

		// "job not found" used to fall through as success, masking
		// real failures (rclone evicted the job, the daemon was
		// restarted, jobid leaked across processes, ...). Treat it
		// as a hard error so the caller can decide to retry instead
		// of believing an unfinished download succeeded.
		if res.Error == "job not found" {
			return errors.New("rclone job not found (id=" + common.ToJson(resp.jobId) + "); download did not finish")
		}

		if !res.Success && !res.Finished {
			continue
		}

		return errors.New(res.Error)
	}
}

func (d *Download) generateBufferFolder() (string, error) {
	var owner = d.fileParam.Owner
	var exists = false
	// Use RootPrefix (env-driven, fallback /data) so the buffer dir
	// resolves to the same root the rest of the codebase uses (see
	// PR #261/#262). ROOT_PREFIX is a hardcoded "/data" constant
	// and ignores the deployment override.
	var bufferFolder = path.Join(common.RootPrefix, common.CacheBuffer, owner)
	if files.FilePathExists(bufferFolder) {
		exists = true
	}

	if !exists {
		if err := files.MkdirAllWithChown(nil, bufferFolder, 0755, true, 1000, 1000); err != nil {
			return "", err
		}
	}

	if !strings.HasSuffix(bufferFolder, "/") {
		bufferFolder = bufferFolder + "/"
	}

	return bufferFolder, nil
}

func (d *Download) checkCtx() error {
	select {
	case <-d.ctx.Done():
		return d.ctx.Err()
	default:
		return nil
	}
}
