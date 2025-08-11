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
	"path/filepath"
	"strings"
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

func (c *Handler) getSize(fs string, remote string, isFile bool) (int64, error) {
	cmd := rclone.Command
	if remote == "" {
		resp, err := cmd.GetOperation().Size(fs)
		if err != nil {
			return 0, err
		}

		return resp.Bytes, nil
	}

	var opts = &operations.OperationsOpt{}
	if isFile {
		opts.FilesOnly = true
	} else {
		opts.DirsOnly = true
	}

	resp, err := cmd.GetOperation().Stat(fs, remote, opts)
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

	srcFileOrDirName, isFile := utils.GetFileNameFromPath(c.src.Path)
	dstFileOrDirName, _ := utils.GetFileNameFromPath(c.dst.Path)

	var srcFileExt string
	if isFile {
		srcFileExt = filepath.Ext(srcFileOrDirName)
	}

	var listDstFs, listDstUri string
	var listResult *operations.OperationsList
	var opts = &operations.OperationsOpt{}

	klog.Infof("cloudTransfer - owner: %s, srcName: %s, dstName: %s, isFile: %v", c.owner, srcFileOrDirName, dstFileOrDirName, isFile)

	// check dst name exsts
	if c.dst.FileType == utils.Drive || c.dst.FileType == utils.Cache || c.dst.FileType == utils.External {
		listDstUri, err = c.dst.GetResourceUri()
		if err != nil {
			return err
		}
		listDstFs = "local:" + listDstUri + utils.GetPrefixPath(c.dst.Path)
	} else {
		listDstConfigName := fmt.Sprintf("%s_%s_%s", c.owner, c.dst.FileType, c.dst.Extend)
		dstConfig, err := cmd.GetConfig().GetConfig(listDstConfigName)
		if err != nil {
			return err
		}
		listDstFs = fmt.Sprintf("%s:%s%s", listDstConfigName, dstConfig.Bucket, utils.GetPrefixPath(c.dst.Path))
	}

	if isFile {
		opts.FilesOnly = true
	} else {
		opts.DirsOnly = true
	}

	klog.Infof("cloudTransfer - get list, fs: %s", listDstFs)
	listResult, err = cmd.GetOperation().List(listDstFs, opts)
	if err != nil {
		klog.Errorf("cloudTransfer - get dst list error: %v", err)
		return err
	}

	if listResult != nil && listResult.List != nil && len(listResult.List) > 0 {
		klog.Infof("cloudTransfer - check name exists, count: %d", len(listResult.List))
		var dupNames []string
		for _, item := range listResult.List {
			if isFile {
				var tmpExt = filepath.Ext(item.Name)
				if tmpExt != srcFileExt {
					continue
				}
				var tmp = strings.TrimSuffix(item.Name, srcFileExt)
				if strings.Contains(tmp, strings.TrimSuffix(srcFileOrDirName, srcFileExt)) {
					dupNames = append(dupNames, tmp)
				}
			} else {
				if strings.Contains(item.Name, srcFileOrDirName) {
					dupNames = append(dupNames, item.Name)
				}
			}
		}

		var newName string
		dstPrefixPath := utils.GetPrefixPath(c.dst.Path)

		klog.Infof("cloudTransfer - check name exists, dstPath: %s, dstPrefixPath: %s, dupNames: %v", c.dst.Path, dstPrefixPath, dupNames)
		if len(dupNames) > 0 {
			newName = utils.GenerateDupCommonName(dupNames, strings.TrimSuffix(dstFileOrDirName, srcFileExt), dstFileOrDirName)
		}

		if newName != "" {
			newName = dstPrefixPath + newName + srcFileExt
			if !isFile {
				newName = newName + "/"
			}
			c.dst.Path = newName
		}

		klog.Infof("cloudTransfer - check name exists done! newDstPath: %s", newName)
	}

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
	size, err := c.getSize(srcFs, srcRemote, isFile)
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
		if c.dst.FileType == utils.AwsS3 { // todo  tencent test
			var srcConfigName, srcPrefixPath string

			var srcName, _ = utils.GetFileNameFromPath(c.src.Path)
			if c.src.FileType == utils.Drive || c.src.FileType == utils.Cache || c.src.FileType == utils.External {
				srcUri, err := c.src.GetResourceUri()
				if err != nil {
					return err
				}
				srcConfigName = utils.Local
				srcPrefixPath = srcUri + utils.GetPrefixPath(c.src.Path)
			} else {
				srcConfigName = fmt.Sprintf("%s_%s_%s", c.owner, c.src.FileType, c.src.Extend)
				srcPrefixPath = utils.GetPrefixPath(c.src.Path)
			}

			var dstConfigName, dstPrefixPath string
			var dstName, _ = utils.GetFileNameFromPath(c.dst.Path)
			if c.dst.FileType == utils.Drive || c.dst.FileType == utils.Cache || c.dst.FileType == utils.External {
				dstUri, err := c.dst.GetResourceUri()
				if err != nil {
					return err
				}
				dstConfigName = utils.Local
				dstPrefixPath = dstUri + utils.GetPrefixPath(c.dst.Path)
			} else {
				dstConfigName = fmt.Sprintf("%s_%s_%s", c.owner, c.dst.FileType, c.dst.Extend)
				dstPrefixPath = utils.GetPrefixPath(c.dst.Path)
			}

			klog.Infof("cloudTransfer - generate mk empty dir, srcConfig: %s, dstConfig: %s, srcPath: %s, dstPath: %s, srcName: %s, dstName: %s", srcConfigName, dstConfigName, srcPrefixPath, dstPrefixPath, srcName, dstName)

			if err := cmd.GenerateS3EmptyDirectories(srcConfigName, dstConfigName, srcPrefixPath, dstPrefixPath, srcName, dstName); err != nil {
				return err
			}
		}

		copyResp, err = cmd.GetOperation().Copy(srcFs, dstFs, &async)
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
