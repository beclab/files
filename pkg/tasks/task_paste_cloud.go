package tasks

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

/**
 * ~ DownloadFromCloud
 */
func (t *Task) DownloadFromCloud() error {
	// cloud > posix; cloud > sync
	var cmd = rclone.Command
	var user = t.param.Owner
	var action = t.param.Action
	var src = t.param.Src
	var dst *models.FileParam

	if t.param.Dst.IsSync() { // cloud->sync, if phase 1 complete
		if t.pausedPhase == t.totalPhases {
			klog.Infof("[Task] Id: %s, resume phase: %d", t.id, t.pausedPhase)
			return nil
		}
	}

	if t.wasPaused && t.pausedParam == nil {
		klog.Errorf("[Task] Id: %s, paused param invalid", t.id)
		return errors.New("pause param invalid")
	}

	if !t.param.Dst.IsSync() && t.wasPaused {
		t.param.Dst = t.pausedParam
		t.pausedParam = nil
	}

	if t.param.Dst.IsSync() {
		if !t.wasPaused {
			// copying files to Seahub, the files will first be downloaded to the local
			srcName, isFile := files.GetFileNameFromPath(src.Path)
			srcPath := srcName
			if !isFile {
				srcPath += "/"
			}

			var cacheParam = &models.FileParam{
				Owner:    user,
				FileType: common.Cache,
				Extend:   global.CurrentNodeName,
				Path:     common.DefaultSyncUploadToCloudTempPath + "/" + t.id + "/" + srcPath,
			}
			dst = cacheParam
		} else {
			dst = t.pausedParam
			t.pausedParam = nil
		}
	} else {
		dst = t.param.Dst // posix
	}

	klog.Infof("[Task] Id: %s, start, downloadFromCloud, phase: %d/%d, user: %s, action: %s, src: %s, dst: %s", t.id, t.currentPhase, t.totalPhases, user, action, common.ToJson(src), common.ToJson(dst))

	// check local free space
	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return err
	}

	cloudSize, err := cmd.GetFilesSize(src)
	if err != nil {
		klog.Errorf("get cloud size error: %v", err)
		return err
	}

	posixSize, err := cmd.GetSpaceSize(dst)
	if err != nil {
		klog.Errorf("get posix free space size error: %v", err)
		return err
	}

	klog.Infof("[Task] Id: %s, check posix space, cloudSize: %d, posixSize: %d", t.id, cloudSize, posixSize)

	requiredSpace, ok := common.IsDiskSpaceEnough(posixSize, cloudSize)
	if ok {
		return fmt.Errorf("not enough free space on disk, required: %s, available: %s", common.FormatBytes(requiredSpace), common.FormatBytes(posixSize))
	}

	t.updateTotalSize(cloudSize)

	if !t.wasPaused {
		// If the task has not been paused, it is necessary to check for name conflicts.
		cloudFileName, isFile := files.GetFileNameFromPath(src.Path)
		posixPrefixPath := files.GetPrefixPath(dst.Path)

		// check duplicated names and generate new name
		localItems, err := cmd.GetFilesList(dst, true)
		if err != nil {
			if !strings.Contains(err.Error(), "directory not found") {
				return fmt.Errorf("get local files list error: %v", err)
			}

		}
		var dupNames []string
		if localItems != nil && localItems.List != nil && len(localItems.List) > 0 {
			for _, item := range localItems.List {
				dupNames = append(dupNames, item.Name)
			}
		}

		newPosixName := files.GenerateDupName(dupNames, cloudFileName, t.isFile)
		klog.Infof("[Task] Id: %s, newPosixName: %s", t.id, newPosixName)

		dst.Path = posixPrefixPath + newPosixName
		if !isFile {
			dst.Path += "/"
		}

	}

	if t.param.Dst.IsSync() {
		var dstPath = filepath.Dir(filepath.Join(dstUri, dst.Path))
		if !common.PathExists(dstPath) {
			if err = files.MkdirAllWithChown(nil, dstPath, 0755); err != nil {
				klog.Errorf("[Task] Id: %s, mkdir %s error: %v", t.id, dstPath, err)
				return fmt.Errorf("failed to create parent directories: %v", err)
			}
		}
	}

	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return ctxErr
	}

	// create download job
	jobResp, err := cmd.Copy(src, dst)
	if err != nil {
		klog.Errorf("[Task] Id: %s, copy error: %v", t.id, err)
		return fmt.Errorf("copy error: %v, src: %s, dst: %s", err, common.ToJson(src), common.ToJson(dst))
	}

	if jobResp.JobId == nil {
		klog.Errorf("[Task] Id: %s, job invalid", t.id)
		return fmt.Errorf("job invalid")
	}

	var done bool
	var jobId = *jobResp.JobId

	// check job stats
	done, err = t.checkJobStats(jobId) // todo Continuously monitor the remaining local storage space

	if err != nil && jobId > 0 {
		_, _ = cmd.GetJob().Stop(jobId)
	}

	if err != nil {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return err
	}

	// clear files
	if !t.param.Dst.IsSync() {

		if t.param.Action == common.ActionMove {
			if e := cmd.Clear(t.param.Src); e != nil {
				klog.Errorf("[Task] Id: %s, clear src error: %v", t.id, e)
			}
		}

	} else { // > sync

		var nextPhaseParam = &models.FileParam{
			Owner:    t.param.Owner,
			FileType: common.Cache,
			Extend:   global.CurrentNodeName,
			Path:     dst.Path,
		}
		t.param.Temp = nextPhaseParam

	}

	klog.Infof("[Task] Id: %s done! done: %v, phase: %d, error: %v", t.id, done, t.currentPhase, err)

	return err
}

/**
 * ~ UploadToCloud
 */
func (t *Task) UploadToCloud() error {

	// sync > cloud; posix > cloud; upload to cloud
	var err error
	var cmd = rclone.Command
	var user = t.param.Owner
	var action = t.param.Action // copy, move, upload

	var jobId int

	var src *models.FileParam
	var dst *models.FileParam

	if t.pausedParam != nil {
		dst = t.pausedParam
		t.pausedParam = nil
	} else {
		dst = t.param.Dst
	}

	if t.param.Src.IsSync() { // sync->cloud
		if t.param.Temp == nil {
			return fmt.Errorf("[Task] Id: %s, temp param invalid", t.id)
		} else {
			src = t.param.Temp
		}
	} else {
		src = t.param.Src
	}

	klog.Infof("[Task] Id: %s, start, uploadToCloud, phase: %d/%d, user: %s, action: %s, src: %s, dst: %s", t.id, t.currentPhase, t.totalPhases, user, action, common.ToJson(src), common.ToJson(dst))

	t.updateProgress(0, 0)

	if !t.wasPaused {
		// check duplicated names and generate new name
		posixPathPrefix := files.GetPrefixPath(src.Path)
		posixFileName, _ := files.GetFileNameFromPath(src.Path)
		_, _ = posixPathPrefix, posixFileName
		cloudFileName, isFile := files.GetFileNameFromPath(dst.Path)
		cloudPrefixPath := files.GetPrefixPath(dst.Path)

		cloudItems, err := cmd.GetFilesList(dst, true)
		if err != nil {
			if !strings.Contains(err.Error(), "directory not found") {
				return fmt.Errorf("get local files list error: %v", err)
			}

		}
		var dupNames []string
		if cloudItems != nil && cloudItems.List != nil && len(cloudItems.List) > 0 {
			for _, item := range cloudItems.List {
				dupNames = append(dupNames, item.Name)
			}
		}

		newCloudName := files.GenerateDupName(dupNames, cloudFileName, t.isFile) // posixFileName
		klog.Infof("[Task] Id: %s, newCloudName: %s", t.id, newCloudName)

		dst.Path = cloudPrefixPath + newCloudName

		if !isFile {
			dst.Path += "/"
		}
	}

	// get src size
	posixSize, err := cmd.GetFilesSize(src)
	if err != nil {
		klog.Errorf("get posix size error: %v", err)
		return err
	}

	klog.Infof("[Task] Id: %s, totalSize: %d", t.id, posixSize)

	t.updateTotalSize(posixSize)

	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return ctxErr
	}

	// upload to cloud job
	jobResp, err := cmd.Copy(src, dst)
	if err != nil {
		klog.Errorf("[Task] Id: %s, copy error: %v", t.id, err)
		return fmt.Errorf("copy error: %v, src: %s, dst: %s", err, common.ToJson(src), common.ToJson(dst))
	}

	if jobResp.JobId == nil {
		klog.Errorf("[Task] Id: %s, job invalid", t.id)
		return fmt.Errorf("job invalid")
	}

	var done bool
	jobId = *jobResp.JobId

	// check job stats
	done, err = t.checkJobStats(jobId) // uploadToCloud

	if err != nil && jobId > 0 {
		_, _ = cmd.GetJob().Stop(jobId)
	}

	if err != nil {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return err
	}

	if t.param.Src.IsSync() {
		if e := cmd.ClearTaskCaches(src, t.id); e != nil {
			klog.Errorf("[Task] Id: %s, clear src error: %v", t.id, e)
		}

		if t.param.Action == common.ActionMove {
			if e := seahub.HandleDelete(t.param.Src); e != nil {
				klog.Errorf("[Task] Id: %s, clear sync src error: %v", t.id, e)
			}
		}

	} else {

		if t.param.Action == common.ActionUpload {
			var srcCacheInfoParam = &models.FileParam{
				Owner:    t.param.Src.Owner,
				FileType: t.param.Src.FileType,
				Extend:   t.param.Src.Extend,
				Path:     t.param.Src.Path + ".info",
			}
			if e := cmd.Clear(srcCacheInfoParam); e != nil {
				klog.Errorf("[Task] Id: %s, clear upload cache file error: %v", t.id, e)
			}
		}

		if t.param.Action == common.ActionMove {
			if e := cmd.Clear(t.param.Src); e != nil {
				klog.Errorf("[Task] Id: %s, clear src error: %v", t.id, e)
			}
		}
	}

	klog.Infof("[Task] Id: %s done! done: %v, phase: %d, error: %v", t.id, done, t.currentPhase, err)

	return err
}

/**
 * ~ CopyToCloud
 */
func (t *Task) CopyToCloud() error {
	var cmd = rclone.Command
	var user = t.param.Owner
	var action = t.param.Action
	var src = t.param.Src
	var dst = t.param.Dst

	if t.pausedParam != nil {
		dst = t.pausedParam
		t.pausedParam = nil
	}

	klog.Infof("[Task] Id: %s, start, copyToCloud, user: %s, action: %s, src: %s, dst: %s", t.id, user, action, common.ToJson(src), common.ToJson(dst))

	cloudSrcFileName, isFile := files.GetFileNameFromPath(src.Path)
	cloudDstPrefixPath := files.GetPrefixPath(dst.Path)

	// check duplicated names and generate new name
	cloudDstItems, err := cmd.GetFilesList(dst, true)
	if err != nil {
		return fmt.Errorf("get local files list error: %v", err)
	}
	var dupNames []string
	if cloudDstItems != nil && cloudDstItems.List != nil && len(cloudDstItems.List) > 0 {
		for _, item := range cloudDstItems.List {
			dupNames = append(dupNames, item.Name)
		}
	}

	if !t.wasPaused {
		newCloudDstName := files.GenerateDupName(dupNames, cloudSrcFileName, t.isFile)
		klog.Infof("[Task] Id: %s, newCloudDstName: %s", t.id, newCloudDstName)

		dst.Path = cloudDstPrefixPath + newCloudDstName
		if !isFile {
			dst.Path += "/"
		}
	}

	// get src size
	srcSize, err := cmd.GetFilesSize(src)
	if err != nil {
		klog.Errorf("get posix size error: %v", err)
		return err
	}
	t.updateTotalSize(srcSize)

	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return ctxErr
	}

	// create download job
	jobResp, err := cmd.Copy(src, dst)
	if err != nil {
		klog.Errorf("[Task] Id: %s, copy error: %v", t.id, err)
		return fmt.Errorf("copy error: %v, src: %s, dst: %s", err, common.ToJson(src), common.ToJson(dst))
	}

	if jobResp.JobId == nil {
		klog.Errorf("[Task] Id: %s, job invalid", t.id)
		return fmt.Errorf("job invalid")
	}

	var done bool
	var jobId = *jobResp.JobId

	// check job stats
	done, err = t.checkJobStats(jobId)

	if err != nil && jobId > 0 {
		_, _ = cmd.GetJob().Stop(jobId)
	}

	// clear files
	if err == nil {

		if t.param.Action == common.ActionMove {
			if e := cmd.Clear(t.param.Src); e != nil {
				klog.Errorf("[Task] Id: %s, clear src error: %v", t.id, e)
			}
		}

	} else {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
	}

	klog.Infof("[Task] Id: %s done! done: %v, phase: %d, error: %v", t.id, done, t.currentPhase, err)

	return err

}

func (t *Task) checkJobStats(jobId int) (bool, error) {
	var cmd = rclone.Command
	var jobCoreStatsResp []byte
	var jobStatusResp []byte
	var err error
	var transferFinished bool
	var done bool
	var ticker = time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			jobCoreStatsResp, err = cmd.GetJob().Stats(jobId)
			if err != nil {
				if err.Error() == "job not found" {
					err = nil
					done = true
					break
				}
				err = fmt.Errorf("get job core stats error: %v", err)
				break
			}

			var data *job.CoreStatsResp
			if err = json.Unmarshal(jobCoreStatsResp, &data); err != nil {
				err = fmt.Errorf("unmarshal job core stats error: %v", err)
				break
			}

			jobStatusResp, err = cmd.GetJob().Status(jobId)
			if err != nil {
				if err.Error() == "job not found" {
					err = nil
					done = true
					break
				}
				err = fmt.Errorf("get job status error: %v", err)
				break
			}

			var jobStatusData *job.JobStatusResp
			if err = json.Unmarshal(jobStatusResp, &jobStatusData); err != nil {
				err = fmt.Errorf("unmarshal job status error: %v", err)
				break
			}

			if jobStatusData.Error != "" {
				err = errors.New(jobStatusData.Error)
				break
			}

			// klog.Infof("[Task] Id: %s, get job core stats: %s, status: %s", t.id, common.ToJson(data), common.ToJson(jobStatusData))
			klog.Infof("[Task] Id: %s, get job core stats", t.id)

			var totalTransfers = t.totalSize
			var transfers = data.Bytes

			if transfers != totalTransfers {
				var progress = transfers * 100 / totalTransfers
				klog.Infof("[Task] Id: %s, jobId: %d, progress: %d, transfers: %d, totals: %d", t.id, jobId, progress, transfers, totalTransfers)
				t.updateProgress(int(progress), data.Bytes)
				continue
			}

			if transfers == totalTransfers && data.Transferring == nil && data.Bytes == data.TotalBytes {
				klog.Infof("[Task] Id: %s, upload success, jobId: %d", t.id, jobId)
				var progress = 100
				transferFinished = true
				t.updateProgress(int(progress), data.TotalBytes)
			}

			if !jobStatusData.Finished {
				if transferFinished {
					t.tidyDirs = true
				}
				continue
			} else {
				klog.Infof("[Task] Id: %s, job finished: %v", t.id, jobStatusData.Finished)
				t.details = append(t.details, "finished")
				done = true
				err = nil
			}

			break
		case <-t.ctx.Done():
			err = t.ctx.Err()
			klog.Infof("[Task] Id: %s, job %v, jobId: %d", t.id, err, jobId)
			break
		}

		break
	}

	return done, err
}
