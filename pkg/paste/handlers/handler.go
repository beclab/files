package handlers

import (
	"context"
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/models"
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
	GetProgress     func() (string, int, int64, int64)
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

func (c *Handler) getSize(fsPrfix string, srcPath string, srcName string, srcPrefixPath string, isFile bool) (int64, error) {
	cmd := rclone.Command
	if !isFile {
		var fs = fsPrfix + srcPath
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

	var fs = fsPrfix + srcPrefixPath
	resp, err := cmd.GetOperation().Stat(fs, srcName, opts)
	if err != nil {
		return 0, err
	}

	return resp.Item.Size, nil
}

func (c *Handler) cloudPaste() error {
	var action = c.action

	var err = c.cloudTransfer()

	if err == nil {
		if action == "copy" {
			return nil
		}

		return c.clearTarget(true)
	}

	return c.clearTarget(false)
}

func (c *Handler) cloudTransfer() error {
	// UploadToCloud
	// DownloadFromCloud
	// CopyToCloud

	klog.Infof("cloudTransfer - start, owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, common.ToJson(c.src), common.ToJson(c.dst))

	var err error

	cmd := rclone.Command

	srcFileOrDirName, isFile := common.GetFileNameFromPath(c.src.Path)
	srcPrefix := common.GetPrefixPath(c.src.Path)
	dstFileOrDirName, _ := common.GetFileNameFromPath(c.dst.Path)
	dstPrefix := common.GetPrefixPath(c.dst.Path)

	var srcFileExt string
	if isFile {
		srcFileExt = filepath.Ext(srcFileOrDirName)
	}

	var listDstFs string
	var listResult *operations.OperationsList
	var opts = &operations.OperationsOpt{}

	klog.Infof("cloudTransfer - owner: %s, srcName: %s, dstName: %s, isFile: %v", c.owner, srcFileOrDirName, dstFileOrDirName, isFile)

	// get src fs prefix

	srcFsPrefix, err := cmd.GetFsPrefix(c.src) // configName:bucket or configName:
	if err != nil {
		return err
	}

	dstFsPrefix, err := cmd.GetFsPrefix(c.dst) // configName:bucket or configName:
	if err != nil {
		return err
	}

	// check src size
	size, err := c.getSize(srcFsPrefix, c.src.Path, srcFileOrDirName, srcPrefix, isFile)
	if err != nil {
		klog.Errorf("cloudTransfer - get size error: %v, srcFsPrefix: %s, srcPath: %s, srcName: %s, srcPrefix: %s, isFile: %v", err, srcFsPrefix, c.src.Path, srcFileOrDirName, srcPrefix, isFile)
		return err
	}

	klog.Errorf("cloudTransfer - get size done! srcFsPrefix: %s, srcPath: %s, srcName: %s, srcPrefix: %s, isFile: %v", srcFsPrefix, c.src.Path, srcFileOrDirName, srcPrefix, isFile)

	c.UpdateTotalSize(size)

	// check dst name exists
	if isFile {
		opts.FilesOnly = true
	} else {
		opts.DirsOnly = true
	}

	listDstFs = dstFsPrefix + dstPrefix

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

		dstPrefixPath := common.GetPrefixPath(c.dst.Path)

		klog.Infof("cloudTransfer - check name exists, dstPath: %s, dstPrefixPath: %s, dupNames: %v", c.dst.Path, dstPrefixPath, dupNames)

		var newName string
		if len(dupNames) > 0 {
			newName = common.GenerateDupCommonName(dupNames, strings.TrimSuffix(dstFileOrDirName, srcFileExt), dstFileOrDirName)
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

	// klog.Infof("cloudTransfer - addr, srcFs: %s, srcR: %s, dstFs: %s, dstR: %s, isFile: %v", srcFs, srcRemote, dstFs, dstRemote, isFile)

	var done bool
	var async = true
	var copyResp *operations.OperationsCopyFileResp

	if isFile {
		srcFs = srcFsPrefix + srcPrefix
		srcRemote = srcFileOrDirName
		dstFs = dstFsPrefix + dstPrefix
		dstRemote, _ = common.GetFileNameFromPath(c.dst.Path) // strings.Trim(newName, "/")

		klog.Infof("cloudTransfer - copyFile, srcFs: %s, dstFs: %s, srcRemote: %s, dstRemote: %s", srcFs, dstFs, srcRemote, dstRemote)

		// srcFs  configName:xxx
		copyResp, err = cmd.GetOperation().Copyfile(srcFs, srcRemote, dstFs, dstRemote, &async)
	} else {
		// dir
		dstName, _ := common.GetFileNameFromPath(c.dst.Path)
		srcFs = srcFsPrefix + c.src.Path
		dstFs = dstFsPrefix + dstPrefix + dstName

		klog.Infof("cloudTransfer - syncCopy, srcFs: %s, dstFs: %s", srcFs, dstFs)

		// if c.dst.FileType == utils.AwsS3 || c.dst.FileType == utils.TencentCos || c.dst.FileType == utils.DropBox || c.dst.FileType == utils.GoogleDrive {
		var srcConfigName, srcPrefixPath string

		var srcName, _ = common.GetFileNameFromPath(c.src.Path)
		if c.src.FileType == common.Drive || c.src.FileType == common.Cache || c.src.FileType == common.External {
			srcUri, err := c.src.GetResourceUri()
			if err != nil {
				return err
			}
			srcConfigName = common.Local
			srcPrefixPath = srcUri + common.GetPrefixPath(c.src.Path)
		} else {
			srcConfigName = fmt.Sprintf("%s_%s_%s", c.owner, c.src.FileType, c.src.Extend)
			srcPrefixPath = common.GetPrefixPath(c.src.Path)
		}

		var dstConfigName, dstPrefixPath string
		dstName, _ = common.GetFileNameFromPath(c.dst.Path) // +
		if c.dst.FileType == common.Drive || c.dst.FileType == common.Cache || c.dst.FileType == common.External {
			dstUri, err := c.dst.GetResourceUri()
			if err != nil {
				return err
			}
			dstConfigName = common.Local
			dstPrefixPath = dstUri + common.GetPrefixPath(c.dst.Path)
		} else {
			dstConfigName = fmt.Sprintf("%s_%s_%s", c.owner, c.dst.FileType, c.dst.Extend)
			dstPrefixPath = common.GetPrefixPath(c.dst.Path)
		}

		klog.Infof("cloudTransfer - generate mk empty dir, srcConfig: %s, dstConfig: %s, srcPath: %s, dstPath: %s, srcName: %s, dstName: %s", srcConfigName, dstConfigName, srcPrefixPath, dstPrefixPath, srcName, dstName)

		if err := cmd.GenerateS3EmptyDirectories(c.dst.FileType, srcConfigName, dstConfigName, srcPrefixPath, dstPrefixPath, srcName, dstName); err != nil {
			return err
		}
		// }

		klog.Infof("cloudTransfer - syncCopy, srcFs: %s, dstFs: %s", srcFs, dstFs)

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

			klog.Infof("cloudTransfer - get job data: %s", common.ToJson(data))

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

// copy done
func (c *Handler) clearTarget(isSrc bool) error {
	var err error
	var cmd = rclone.Command
	var owner = c.owner
	var configName string
	var isSrcLocal bool
	var target *models.FileParam

	if isSrc {
		target = c.src
	} else {
		target = c.dst
	}

	srcFileOrDirName, isFile := common.GetFileNameFromPath(target.Path)
	srcPrefixPath := common.GetPrefixPath(target.Path)

	if target.FileType == common.Drive || target.FileType == common.Cache || target.FileType == common.External {
		isSrcLocal = true
	}

	if isSrcLocal {
		configName = common.Local
	} else {
		configName = fmt.Sprintf("%s_%s_%s", c.owner, target.FileType, target.Extend)
	}

	config, err := cmd.GetConfig().GetConfig(configName)
	if err != nil {
		return err
	}

	var srcFsPrefix string
	if isSrcLocal {
		srcUri, err := target.GetResourceUri()
		if err != nil {
			return err
		}
		srcFsPrefix = fmt.Sprintf("%s:%s", configName, srcUri)
	} else {
		srcFsPrefix = fmt.Sprintf("%s:%s", configName, config.Bucket)
	}

	if isFile {
		var fs, remote string
		fs = srcFsPrefix + srcPrefixPath
		remote = srcFileOrDirName

		if err = cmd.GetOperation().Deletefile(fs, remote); err != nil {
			klog.Errorf("cloudTransfer - clear file error: %v, isSrc: %v, user: %s, fs: %s, remote: %s", err, isSrc, owner, fs, remote)
			return err
		}

		cmd.GetOperation().FsCacheClear()

		klog.Infof("cloudTransfer - clear file done! isSrc: %v, user: %s, fs: %s, remote: %s", isSrc, owner, fs, remote)

		return nil
	}

	// purge
	var fs = srcFsPrefix + srcPrefixPath
	var remote = srcFileOrDirName

	if err = cmd.GetOperation().Purge(fs, remote); err != nil {
		klog.Errorf("cloudTransfer - clear directory error: %v, isSrc: %v, user: %s, fs: %s, remote: %s", err, isSrc, owner, fs, remote)
		return err
	}

	cmd.GetOperation().FsCacheClear()

	klog.Infof("cloudTransfer - clear directory done! isSrc: %v, user: %s, fs: %s, remote: %s", isSrc, owner, fs, remote)

	return nil
}
