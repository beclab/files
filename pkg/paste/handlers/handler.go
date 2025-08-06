package handlers

import (
	"context"
	"encoding/json"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

type Interface interface {
	Rsync() error // include rsync and mv

	UploadToSync() error
	UploadToCloud() error

	DownloadFromFiles() error
	DownloadFromSync() error
	DownloadFromCloud() error

	SyncCopy() error
	CloudCopy() error
}

type Handler struct {
	ctx             context.Context
	owner           string
	action          string
	src             *models.FileParam
	dst             *models.FileParam
	Exec            func() error
	UpdateTotalSize func(totalSize int64)
	UpdateProgress  func(progress int, transfer int64)
}

func NewHandler(ctx context.Context, param *models.PasteParam) *Handler {
	return &Handler{
		ctx:    ctx,
		owner:  param.Owner,
		action: param.Action,
		src:    param.Src,
		dst:    param.Dst,
	}
}

func (c *Handler) getSize(fs string, remote string) (int64, error) {
	cmd := rclone.Command
	if remote == "" {
		resp, err := cmd.GetOperation().Size(fs)
		if err != nil {
			return 0, err
		}

		return resp.Bytes, nil
	}

	resp, err := cmd.GetOperation().Stat(fs, remote)
	if err != nil {
		return 0, err
	}

	return resp.Item.Size, nil
}

func (c *Handler) cloudTransfer() error {
	// UploadToCloud
	// DownloadFromCloud
	// CopyToCloud

	klog.Infof("cloudTransfer - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	var err error

	cmd := rclone.Command

	_, isFile := c.src.IsFile()

	var srcFs, dstFs string
	var srcRemote, dstRemote string
	srcFs, err = cmd.FormatFs(c.src)
	if err != nil {
		klog.Errorf("cloudTransfer - format src fs error: %v", err)
		return err
	}

	dstFs, err = cmd.FormatFs(c.dst)
	if err != nil {
		klog.Errorf("cloudTransfer - format dst fs error: %v", err)
		return err
	}

	if isFile {
		srcRemote, err = cmd.FormatRemote(c.src)
		if err != nil {
			klog.Errorf("cloudTransfer - format src remote error: %v", err)
			return err
		}
		dstRemote, _ = cmd.FormatRemote(c.dst)
	}

	// update size
	size, err := c.getSize(srcFs, srcRemote)
	if err != nil {
		return err
	}
	c.UpdateTotalSize(size)

	klog.Infof("cloudTransfer - isfile: %v, srcFs: %s, dstFs: %s, srcRemote: %s, dstRemote: %s", isFile, srcFs, dstFs, srcRemote, dstRemote)

	var done bool
	var async = true
	var copyResp *operations.OperationsCopyFileResp
	if isFile {
		copyResp, err = cmd.GetOperation().Copyfile(srcFs, srcRemote, dstFs, dstRemote, &async)
	} else {
		copyResp, err = cmd.GetOperation().Copy(srcFs, dstFs)
	}

	if err != nil {
		return err
	}

	if copyResp.JobId == nil {
		return fmt.Errorf("job id invalid")
	}

	var jobId = *copyResp.JobId

	klog.Infof("cloudTransfer - jobId: %d", jobId)

	var ticker = time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	defer func() error {
		if !done && jobId > 0 {
			_, stopErr := cmd.GetJob().Stop(jobId)
			if stopErr != nil {
				return fmt.Errorf("stop job failed: %v, error: %v", stopErr, err)
			}
		}
		if err != nil {
			return err
		}

		return nil
	}()

	for {
		select {
		case <-ticker.C:
			resp, err := cmd.GetJob().Stats(jobId)
			if err != nil {
				klog.Errorf("cloudTransfer - get job stats error: %v, jobId: %d", err, jobId)
				return err
			}
			var data *job.CoreStatsResp
			if err := json.Unmarshal(resp, &data); err != nil {
				klog.Errorf("cloudTransfer - unmarshal job stats error: %v, jobId: %d", err, jobId)
				return err
			}

			klog.Infof("cloudTransfer - get job data: %s", utils.ToJson(data))

			var totalTransfers = data.TotalBytes //data.TotalTransfers
			var transfers = data.Bytes           //data.Transfers

			if transfers != totalTransfers {
				var progress = transfers * 100 / totalTransfers
				klog.Infof("cloudTransfer - uploading, jobId: %d, progress: %d, transfers: %d, totals: %d", jobId, progress, transfers, totalTransfers)
				c.UpdateProgress(int(progress), data.Bytes)
				continue
			}

			if transfers == totalTransfers && data.Transferring == nil && data.Bytes == data.TotalBytes {
				klog.Infof("cloudTransfer - upload success, jobId: %d", jobId)
				c.UpdateProgress(100, data.TotalBytes)
				done = true
				return nil
			}

		case <-c.ctx.Done():
			klog.Infof("cloudTransfer - context cancel, jobId: %d", jobId)
			return c.ctx.Err()
		}
	}
}
