package tasks

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
	"path/filepath"
	"sort"
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
	var parentPath = t.param.UploadToCloudParentPath
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

	klog.Infof("[Task] Id: %s, start, downloadFromCloud, phase: %d/%d, paused: %v, user: %s, action: %s, src: %s, dst: %s", t.id, t.currentPhase, t.totalPhases, t.wasPaused, user, action, common.ToJson(src), common.ToJson(dst))

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
		if t.currentPhase == t.totalPhases {
			var newDstPath string
			newDstPath, err = t.manager.GetCloudOrPosixDupNames(t.id, action, parentPath, src, dst, t.param.Src, t.param.Dst)
			if err != nil {
				klog.Errorf("[Task] Id: %s, get dup name by lock error: %v", t.id, err)
				return err
			}

			dst.Path = newDstPath

			klog.Infof("[Task] Id: %s, create dup done! dst path: %s", t.id, dst.Path)
		}
	}

	var dstPath = filepath.Dir(filepath.Join(dstUri, dst.Path))
	if !common.PathExists(dstPath) {
		if err = files.MkdirAllWithChown(nil, dstPath, 0755); err != nil {
			klog.Errorf("[Task] Id: %s, mkdir %s error: %v", t.id, dstPath, err)
			return fmt.Errorf("failed to create parent directories: %v", err)
		}
	}

	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return ctxErr
	}

	var done bool

	if t.param.Src.IsGoogleDrive() {
		var existDups bool
		var driveId string
		existDups, driveId, err = cmd.CheckGoogleDriveDupNames(src)
		if err != nil {
			klog.Errorf("[Task] Id: %s, check google drive dups error: %v", t.id, err)
			return err
		}

		if existDups {
			return errors.New("There is a folder or file with the same name as the current operation. Please log in to Google Drive to rename the object and ensure its name is unique.")
		}

		if err = t.copyId(src, dst, driveId); err != nil {
			klog.Errorf("[Task] Id: %s, copyid error: %v", t.id, err)
			return fmt.Errorf("copyid error: %v, src: %s, dst: %s", err, common.ToJson(src), common.ToJson(dst))
		}
		done = true

	} else {

		jobResp, err := cmd.Copy(src, dst)
		if err != nil {
			klog.Errorf("[Task] Id: %s, copy error: %v", t.id, err)
			return fmt.Errorf("copy error: %v, src: %s, dst: %s", err, common.ToJson(src), common.ToJson(dst))
		}

		if jobResp.JobId == nil {
			klog.Errorf("[Task] Id: %s, job invalid", t.id)
			return fmt.Errorf("job invalid")
		}

		var jobId = *jobResp.JobId

		// check job stats
		done, err = t.checkJobStats(jobId, dst.Path) // todo Continuously monitor the remaining local storage space

		if err != nil && jobId > 0 {
			_, _ = cmd.GetJob().Stop(jobId)
		}
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
	var action = t.param.Action                            // copy, move, upload
	var uploadParentPath = t.param.UploadToCloudParentPath // if not upload ,it's empty

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

	klog.Infof("[Task] Id: %s, start, uploadToCloud, phase: %d/%d, user: %s, action: %s, uploadParentPath: %s, src: %s, dst: %s", t.id, t.currentPhase, t.totalPhases, user, action, uploadParentPath, common.ToJson(src), common.ToJson(dst))

	if action == common.ActionUpload && uploadParentPath == "" {
		return fmt.Errorf("uploaded parent path invalid")
	}

	t.updateProgress(0, 0)

	// get src size
	posixSize, err := cmd.GetFilesSize(src)
	if err != nil {
		klog.Errorf("get posix size error: %v", err)
		return err
	}

	klog.Infof("[Task] Id: %s, totalSize: %d", t.id, posixSize)

	t.updateTotalSize(posixSize)

	if !t.wasPaused || (t.wasPaused && t.pausedPhase != t.currentPhase) {
		// check duplicated names and generate new name
		klog.Infof("[Task] Id: %s, check dup name, wasPaused: %v, pausedPhase: %d", t.id, t.wasPaused, t.pausedPhase)
		var newDstPath string
		newDstPath, err = t.manager.GetCloudOrPosixDupNames(t.id, action, uploadParentPath, src, dst, t.param.Src, t.param.Dst) // uploadToCloud
		if err != nil {
			klog.Errorf("[Task] Id: %s, get dup name by lock error: %v", t.id, err)
			return err
		}
		dst.Path = newDstPath

		klog.Infof("[Task] Id: %s, create dup done! dst path: %s", t.id, dst.Path)

	}

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
	done, err = t.checkJobStats(jobId, dst.Path) // uploadToCloud

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

		if t.param.Action == common.ActionMove || t.param.Action == common.ActionUpload {
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
	var parentPath = t.param.UploadToCloudParentPath
	var src = t.param.Src
	var dst = t.param.Dst

	if t.pausedParam != nil {
		dst = t.pausedParam
		t.pausedParam = nil
	}

	klog.Infof("[Task] Id: %s, start, copyToCloud, user: %s, action: %s, src: %s, dst: %s", t.id, user, action, common.ToJson(src), common.ToJson(dst))

	// get src size
	srcSize, err := cmd.GetFilesSize(src)
	if err != nil {
		klog.Errorf("get posix size error: %v", err)
		return err
	}
	t.updateTotalSize(srcSize)

	if !t.wasPaused {
		klog.Infof("[Task] Id: %s, check dup name, wasPaused: %v, pausedPhase: %d", t.id, t.wasPaused, t.pausedPhase)
		var newDstPath string
		newDstPath, err = t.manager.GetCloudOrPosixDupNames(t.id, action, parentPath, src, dst, t.param.Src, t.param.Dst)
		if err != nil {
			klog.Errorf("[Task] Id: %s, get dup name by lock error: %v", t.id, err)
			return err
		}

		dst.Path = newDstPath

		klog.Infof("[Task] Id: %s, create dup done! dst path: %s", t.id, dst.Path)
	}

	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return ctxErr
	}

	var done bool

	if t.param.Src.IsGoogleDrive() {
		var existDups bool
		var driveId string
		existDups, driveId, err = cmd.CheckGoogleDriveDupNames(src)
		if err != nil {
			klog.Errorf("[Task] Id: %s, check google drive dups error: %v", t.id, err)
			return err
		}

		if existDups {
			return errors.New("There is a folder or file with the same name as the current operation. Please log in to Google Drive to rename the object and ensure its name is unique.")
		}

		if err = t.copyId(src, dst, driveId); err != nil {
			klog.Errorf("[Task] Id: %s, copyid error: %v", t.id, err)
			return fmt.Errorf("copyid error: %v, src: %s, dst: %s", err, common.ToJson(src), common.ToJson(dst))
		}
		done = true
	} else {
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

		var jobId = *jobResp.JobId

		// check job stats
		done, err = t.checkJobStats(jobId, dst.Path)

		if err != nil && jobId > 0 {
			_, _ = cmd.GetJob().Stop(jobId)
		}
	}

	if err != nil {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return err
	}

	// clear files
	if t.param.Action == common.ActionMove {
		if e := cmd.Clear(t.param.Src); e != nil {
			klog.Errorf("[Task] Id: %s, clear src error: %v", t.id, e)
		}
	}

	klog.Infof("[Task] Id: %s done! done: %v, phase: %d, error: %v", t.id, done, t.currentPhase, err)

	return err

}

func (t *Task) checkJobStats(jobId int, dstPath string) (bool, error) {
	var cmd = rclone.Command
	var jobCoreStatsResp []byte
	var jobStatusResp []byte
	var err error
	var transferFinished bool
	var done bool

	var totalSize = t.totalSize
	var preTransfered int64 = 0

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
				klog.Errorf("[Task] Id: %s, job status error: %s", t.id, jobStatusData.Error)
				if (t.param.Dst.IsSync() && t.param.Src.IsCloud()) || (t.param.Src.IsCloud() && t.param.Dst.IsSync()) {
					err = t.formatJobStatusError(jobStatusData.Error)
				}
				break
			}

			var transfers int64
			if preTransfered == 0 {
				preTransfered = data.Bytes
			} else {
				transfers = data.Bytes - preTransfered
				// if transfers == 0 {
				// 	transfers = data.Bytes
				// }
				preTransfered = data.Bytes
			}

			klog.Infof("[Task] Id: %s, get job core stats: %s, status: %s", t.id, common.ToJson(data), common.ToJson(jobStatusData))

			var totalTransfers = t.transfer

			if jobStatusData.Success && jobStatusData.Finished {
				klog.Infof("[Task] Id: %s, job finished: %v, dst: %s", t.id, jobStatusData.Finished, dstPath)
				var progress = (totalTransfers * 100) / totalSize
				transferFinished = true
				t.updateProgress(int(progress), transfers)
				done = true
				err = nil
				break
			}

			if transfers != totalSize {
				var progress = (totalTransfers * 100) / totalSize
				klog.Infof("[Task] Id: %s, dst: %s, progress: %d (%s/%s)", t.id, dstPath, progress, common.FormatBytes(totalTransfers), common.FormatBytes(totalSize))
				t.updateProgress(int(progress), transfers)
				continue
			}

			if transfers == totalSize && data.Transferring == nil && data.Bytes == data.TotalBytes {
				klog.Infof("[Task] Id: %s, upload success, dst: %s", t.id, dstPath)
				var progress = (totalTransfers * 100) / totalSize
				transferFinished = true
				t.updateProgress(int(progress), transfers)
			}

			if !jobStatusData.Finished {
				if transferFinished {
					t.tidyDirs = true
				}
				continue
			} else {
				klog.Infof("[Task] Id: %s, job finished: %v, dst: %s", t.id, jobStatusData.Finished, dstPath)
				t.details = append(t.details, "finished")
				done = true
				err = nil
			}

			break
		case <-t.ctx.Done():
			err = t.ctx.Err()
			klog.Infof("[Task] Id: %s, job %v done, dst: %s", t.id, err, dstPath)
			break
		}

		break
	}

	return done, err
}

// new version
type DriveDir struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Id    string `json:"id"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

func (t *Task) copyId(src, dst *models.FileParam, driveId string) error {
	var cmd = rclone.Command
	var err error
	var fsPrefix, fs string

	fsPrefix, err = cmd.GetFsPrefix(src)
	if err != nil {
		return err
	}

	var dstFsPrefix string
	dstFsPrefix, err = cmd.GetFsPrefix(dst)
	if err != nil {
		return err
	}

	srcName, isSrcFile := files.GetFileNameFromPath(src.Path)

	dstFsPrefix = dstFsPrefix + dst.Path // /xx/xx/

	if isSrcFile {

		var done bool
		var fs = fsPrefix
		var args = []string{
			driveId,
			dstFsPrefix,
		}
		jobCopyAsyncJob, err := cmd.GetOperation().CopyIdAsync(fs, args)
		if err != nil {
			return err
		}
		var jobId = *jobCopyAsyncJob.JobId
		done, err = t.checkJobStats(jobId, dst.Path)
		_ = done
		if err != nil && jobId > 0 {
			_, _ = cmd.GetJob().Stop(jobId)
		}
	} else {
		fs = strings.TrimRight(fsPrefix, ":") + ",root_folder_id=" + driveId + ":"
		if err = t.loopDriveDirs(fs, driveId, srcName, dstFsPrefix, fsPrefix); err != nil {
			return err
		}
	}

	return err
}

func (t *Task) loopDriveDirs(fs string, parentId, parentName, dstFsPrefix string, srcFsPrefix string) error {
	var err error
	var cmd = rclone.Command
	srcDirs, e := cmd.GetOperation().List(fs, &operations.OperationsOpt{
		Recurse:    false,
		NoModTime:  true,
		NoMimeType: true,
		Metadata:   false,
	}, nil)
	if e != nil {
		return e
	}

	if srcDirs == nil || srcDirs.List == nil || len(srcDirs.List) == 0 {
		return nil
	}

	var items []DriveDir
	for _, item := range srcDirs.List {
		var i = DriveDir{
			Path:  item.Path,
			Name:  item.Name,
			Id:    item.ID,
			IsDir: item.IsDir,
			Size:  item.Size,
		}
		items = append(items, i)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	newDriveDriList := t.encodeDuplicateNames(items)

	for _, item := range newDriveDriList {
		if !item.IsDir {
			var fs = strings.TrimRight(srcFsPrefix, ":") + ",root_folder_id=" + parentId + ":"
			var args = []string{
				item.Id,
				dstFsPrefix + item.Name,
			}
			var e error
			var jobStatusResp *operations.OperationsAsyncJobResp
			jobStatusResp, e = cmd.GetOperation().CopyIdAsync(fs, args)
			if e != nil {
				err = e
				break
			}

			var done bool
			var jobId = *jobStatusResp.JobId
			done, e = t.checkJobStats(jobId, dstFsPrefix+item.Name)
			if e != nil && jobId > 0 {
				_, _ = cmd.GetJob().Stop(jobId)
			}
			_ = done

			if e != nil {
				err = e
				return err
			}

			continue
		}

		var dstDirFs = dstFsPrefix
		if err = cmd.GetOperation().Mkdir(dstDirFs, item.Name); err != nil {
			return err
		}

		subFs := strings.TrimRight(srcFsPrefix, ":") + ",root_folder_id=" + item.Id + ":"

		if err = t.loopDriveDirs(subFs, item.Id, dstDirFs, dstDirFs+item.Name+"/", srcFsPrefix); err != nil {
			return err
		}
	}

	return err
}

func (t *Task) encodeDuplicateNames(dirs []DriveDir) []DriveDir {
	parentNameCount := make(map[string]map[string]int)

	result := make([]DriveDir, len(dirs))

	for i, dir := range dirs {
		parent := dir.Path
		name := dir.Name
		if parentNameCount[parent] == nil {
			parentNameCount[parent] = make(map[string]int)
		}
		count := parentNameCount[parent][name]
		if count == 0 {
			result[i] = dir
		} else {
			var createName, createExt string
			if !dir.IsDir {
				createName, createExt = common.SplitNameExt(name)
			} else {
				createName = name
			}

			newName := fmt.Sprintf("%s (%d)%s", createName, count, createExt)
			newDir := dir
			newDir.Name = newName
			result[i] = newDir
		}
		parentNameCount[parent][name]++
	}
	return result
}
