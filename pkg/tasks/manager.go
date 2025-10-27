package tasks

import (
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/redisutils"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
	"k8s.io/klog/v2"
)

var TaskManager *taskManager

var (
	lockTTL      = 10 * time.Second
	maxWait      = 15 * time.Second
	waitInterval = 100 * time.Millisecond
)

var TaskCancel = "context canceled"

type taskManager struct {
	userPools sync.Map
	sync.RWMutex
}

type userPool struct {
	owner string
	pool  pond.Pool
	tasks sync.Map
}

func NewTaskManager() {
	TaskManager = &taskManager{
		userPools: sync.Map{},
	}
}

func (t *taskManager) getOrCreateUserPool(owner string) *userPool {
	if pool, ok := t.userPools.Load(owner); ok {
		userPool := pool.(*userPool)
		return userPool
	}

	userPool := &userPool{
		owner: owner,
		tasks: sync.Map{},
		pool:  pond.NewPool(1, pond.WithContext(context.Background()), pond.WithNonBlocking(true)),
	}

	t.userPools.Store(owner, userPool)

	return userPool
}

// create
func (t *taskManager) CreateTask(param *models.PasteParam) *Task {

	var ctx, cancel = context.WithCancel(context.Background())
	var _, isFile = param.Src.IsFile()

	var task = &Task{
		id:        common.GenerateTaskId(),
		param:     param,
		state:     common.Pending,
		ctx:       ctx,
		ctxCancel: cancel,
		manager:   t,
		isFile:    isFile,
		createAt:  time.Now(),
	}

	return task

}

// resume
func (t *taskManager) ResumeTask(owner, taskId string) error {
	klog.Infof("[Task] Id: %s, Resume, user: %s", taskId, owner)
	userPool := t.getOrCreateUserPool(owner)

	val, ok := userPool.tasks.Load(taskId)
	if !ok {
		return fmt.Errorf("task %s not found", taskId)
	}

	var task = val.(*Task)

	if task.state != common.Paused {
		return fmt.Errorf("task is not paused")
	}

	var ctx, cancel = context.WithCancel(context.Background())

	task.ctx = ctx
	task.ctxCancel = cancel
	task.suspend = false
	task.state = common.Pending

	return task.Execute(task.funcs...)
}

// pause
func (t *taskManager) PauseTask(owner, taskId string) error {
	klog.Infof("[Task] Id: %s, Pause, user: %s", taskId, owner)
	userPool := t.getOrCreateUserPool(owner)

	val, ok := userPool.tasks.Load(taskId)
	if !ok {
		return fmt.Errorf("task %s not found", taskId)
	}

	var task = val.(*Task)

	if task.state != common.Pending && task.state != common.Running {
		return fmt.Errorf("task is not pending or running")
	}

	task.suspend = true
	task.wasPaused = true
	go task.Cancel()

	return nil
}

// cancel
func (t *taskManager) CancelTask(owner, taskId string, all string) {
	klog.Infof("[Task] Id: %s, Cancel, user: %s", taskId, owner)
	userPool := t.getOrCreateUserPool(owner)

	if all == "1" {
		userPool.tasks.Range(func(key, value any) bool {
			task := value.(*Task)
			go task.Cancel()
			return true
		})
		return
	}

	val, ok := userPool.tasks.Load(taskId)
	if !ok {
		return
	}

	task := val.(*Task)
	task.suspend = false
	go task.Cancel()
}

func (t *taskManager) GetTask(owner string, taskId string, status string) []*TaskInfo {
	userPool := t.getOrCreateUserPool(owner)

	if status != "" {
		return t.GetTasksByStatus(owner, status)
	}

	var tasks []*TaskInfo
	val, ok := userPool.tasks.Load(taskId)
	if !ok {
		return tasks
	}

	task := val.(*Task)

	var src = task.param.Src
	var dst = task.param.Dst

	var srcUri = "/" + src.FileType + "/" + src.Extend + src.Path
	var dstUri = "/" + dst.FileType + "/" + dst.Extend + dst.Path
	var dstFileName = files.GetPathName(dst.Path)
	var srcFileName = files.GetPathName(src.Path)

	var pauseAble bool = true

	if src.IsSync() && dst.IsSync() {
		pauseAble = false
	}

	var res = &TaskInfo{
		Id:            task.id,
		Action:        task.param.Action,
		IsDir:         !task.isFile,
		FileName:      srcFileName,
		Dst:           dstUri,
		DstPath:       dstFileName,
		Src:           srcUri,
		CurrentPhase:  task.currentPhase,
		TotalPhases:   task.totalPhases,
		Progress:      task.progress,
		Transferred:   task.transfer,
		TotalFileSize: task.totalSize,
		TidyDirs:      task.tidyDirs,
		Status:        task.state,
		ErrorMessage:  task.message,
		PauseAble:     pauseAble,
	}

	tasks = append(tasks, res)

	return tasks
}

func (t *taskManager) GetTasksByStatus(owner, status string) []*TaskInfo {
	userPool := t.getOrCreateUserPool(owner)

	var tasks []*Task
	var result []*TaskInfo
	userPool.tasks.Range(func(key, value any) bool {
		task := value.(*Task)
		if strings.Contains(status, task.state) {
			tasks = append(tasks, task)
		}

		return true
	})

	if tasks == nil || len(tasks) == 0 {
		return result
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].createAt.Before(tasks[j].createAt) // asc
	})

	for _, task := range tasks {
		var src = task.param.Src
		var dst = task.param.Dst

		var srcUri = "/" + src.FileType + "/" + src.Extend + src.Path
		var dstUri = "/" + dst.FileType + "/" + dst.Extend + dst.Path
		var dstFileName = files.GetPathName(dst.Path)
		var srcFileName = files.GetPathName(src.Path)

		var res = &TaskInfo{
			Id:            task.id,
			Action:        task.param.Action,
			IsDir:         !task.isFile,
			FileName:      srcFileName,
			Dst:           dstUri,
			DstPath:       dstFileName,
			Src:           srcUri,
			CurrentPhase:  task.currentPhase,
			TotalPhases:   task.totalPhases,
			Progress:      task.progress,
			Transferred:   task.transfer,
			TotalFileSize: task.totalSize,
			TidyDirs:      task.tidyDirs,
			Status:        task.state,
			ErrorMessage:  task.message,
		}

		result = append(result, res)
	}

	return result
}

func (t *taskManager) ClearTasks() {
	klog.Info("Task remove finished tasks")
	t.userPools.Range(func(poolUser, poolValue any) bool {
		user := poolUser.(string)
		var tasks []*Task
		userPool := poolValue.(*userPool)

		userPool.tasks.Range(func(_, taskValue any) bool {

			task := taskValue.(*Task)

			if common.ListContains([]string{common.Completed, common.Failed, common.Canceled}, task.state) {
				if task.createAt.Before(time.Now().Add(-12 * time.Hour)) {
					tasks = append(tasks, task)
				}

			}
			return true
		})

		if len(tasks) > 0 {
			klog.Infof("Task remove %s tasks: %d", user, len(tasks))
			for _, t := range tasks {
				userPool.tasks.Delete(t)
			}
		}

		return true
	})
}

func (t *taskManager) GetCloudOrPosixDupNames(taskId string, action string, uploadParentPath string, src, dst, orgSrc, orgDst *models.FileParam) (string, error) {
	t.Lock()
	defer t.Unlock()
	var ok bool

	var err error
	var newDstPath string

	var cmd = rclone.Command
	var orgSrcName string
	var isOrgFile bool
	var orgSrcExt, orgSrcNamePrefix string

	var uploadFullPathFirstDir, uploadFullPathSuffix string
	_ = uploadFullPathFirstDir
	_ = uploadFullPathSuffix

	var dstTemp *models.FileParam

	if action == common.ActionUpload { // upload file to Cloud
		dstTemp = dst
		// dstUri, _ := dstTemp.GetResourceUri()
		// // /google/ACCOUNT
		// var uploadSrcRootPath = strings.TrimPrefix(uploadParentPath, dstUri) // /google/ACCOUNT/RootPath/ , /google/ACCOUNT  ==> /RootPath/

		// var uploadSplitPath = strings.TrimPrefix(dstTemp.Path, uploadSrcRootPath) // RootPath/s1/s2/file
		// uploadSplitPath = strings.TrimPrefix(uploadSplitPath, "/")

		// if strings.Contains(uploadSplitPath, "/") {
		// 	var splitPos = strings.Index(uploadSplitPath, "/")
		// 	uploadFullPathFirstDir = uploadSplitPath[:splitPos]       // firstdir
		// 	uploadFullPathSuffix = uploadSplitPath[splitPos:]         // /dir1/dir2/file.suf
		// 	dstTemp.Path = uploadSrcRootPath + uploadFullPathFirstDir // short path, like  /google/ACCOUNT/RootPath/first/
		// 	if !strings.HasSuffix(dstTemp.Path, "/") {
		// 		dstTemp.Path += "/"
		// 	}
		// } else {
		// 	dstTemp = dst
		// }
	} else {
		dstTemp = dst
	}

	var dstPrefix = files.GetPrefixPath(dstTemp.Path)

	if action == common.ActionUpload {
		orgSrcName, isOrgFile = files.GetFileNameFromPath(dstTemp.Path)
	} else {
		orgSrcName, isOrgFile = files.GetFileNameFromPath(orgSrc.Path)
	}

	var keyPath = fmt.Sprintf("%s_%s_%s_%s", dstTemp.Owner, dstTemp.FileType, dstTemp.Extend, dstTemp.Path)
	var key = common.Md5String(keyPath)

	if isOrgFile {
		_, orgSrcExt = common.SplitNameExt(orgSrcName)
	}

	orgSrcNamePrefix = strings.TrimSuffix(orgSrcName, orgSrcExt)

	klog.Infof("[Task] Id: %s, get lock prepare, srcName: %s, dstPrefix: %s, dst: %s", taskId, orgSrcName, dstPrefix, common.ToJson(dstTemp))

	start := time.Now()
	for {
		if time.Since(start) > maxWait {
			err = errors.New("get lock timeout")
			break
		}

		ok, err = redisutils.RedisClient.SetNX(key, 1, lockTTL).Result()
		if err != nil {
			err = fmt.Errorf("get lock error: %v", err)
			break
		}

		if ok {
			var fsPrefix string
			klog.Infof("[Task] Id: %s, get the lock succeed", taskId)

			fsPrefix, err = cmd.GetFsPrefix(dstTemp)
			if err != nil {
				break
			}
			var fs = fsPrefix + dstPrefix
			var opts = &operations.OperationsOpt{
				Recurse:    false,
				NoModTime:  true,
				NoMimeType: true,
				Metadata:   false,
			}
			var orgSrcNamePrefixTmp = common.EscapeGlob(orgSrcNamePrefix)
			var filterRules = cmd.FormatFilter(orgSrcNamePrefixTmp, true, true, true)
			var filter = &operations.OperationsFilter{}
			if filterRules != nil {
				klog.Infof("[Task] Id: %s, lock, list filter: %v", taskId, filterRules)
				filter.FilterRule = filterRules
			}

			var lists *operations.OperationsList

			lists, err = cmd.GetOperation().List(fs, opts, filter)
			if err != nil {
				if !strings.Contains(err.Error(), "directory not found") {
					break
				}
			}

			var dupNames []string
			if lists != nil && lists.List != nil && len(lists.List) > 0 {
				for _, i := range lists.List {
					var tmpName = i.Name
					if isOrgFile {
						tmpName = strings.TrimSuffix(tmpName, orgSrcExt)
					}
					if strings.Contains(tmpName, orgSrcNamePrefix) {
						dupNames = append(dupNames, i.Name)
					}
				}
			}

			newDstName := files.GenerateDupName(dupNames, orgSrcName, isOrgFile)

			klog.Infof("[Task] Id: %s, lock, new dup dst name: %s", taskId, newDstName)

			newDstPath = dstPrefix + newDstName

			if action == common.ActionUpload {
				// /google/ACCOUNT/RootDir/first (1)/s1/s2/10M.jpg
				newDstPath += uploadFullPathSuffix
			} else {
				if !isOrgFile {
					if !strings.HasSuffix(newDstPath, "/") {
						newDstPath += "/"
					}
				}
			}

			var newDst = &models.FileParam{
				Owner:    dstTemp.Owner,
				FileType: dstTemp.FileType,
				Extend:   dstTemp.Extend,
				Path:     newDstPath,
			}

			klog.Infof("[Task] Id: %s, lock, create placeholder param: %s", taskId, common.ToJson(newDst))

			if err = rclone.Command.CreatePlaceHolder(newDst); err != nil {
				err = fmt.Errorf("[Task] Id: %s, lock, create placeholder error: %v", taskId, err)
				break
			}

			break
		}

		time.Sleep(waitInterval)
	}

	if ok {
		released, e := redisutils.RedisClient.Del(key).Result()
		klog.Infof("[Task] Id: %s, released lock result: %d, error: %v", taskId, released, e)
	}

	if err != nil {
		return "", err
	}

	return newDstPath, nil
}

// sync
func (t *taskManager) GetSyncDupName(taskId string, src, dst, orgSrc, orgDst *models.FileParam) (string, error) {
	t.Lock()
	defer t.Unlock()

	var ok bool
	var err error

	var newDstPath string

	var orgSrcName, isOrgFile = files.GetFileNameFromPath(orgSrc.Path)
	var orgSrcExt, orgSrcNamePrefix string
	var dstPrefix = files.GetPrefixPath(dst.Path)

	var keyPath = fmt.Sprintf("%s_%s_%s_%s", dst.Owner, dst.FileType, dst.Extend, dst.Path)
	var key = common.Md5String(keyPath)

	if isOrgFile {
		_, orgSrcExt = common.SplitNameExt(orgSrcName)
	}

	orgSrcNamePrefix = strings.TrimSuffix(orgSrcName, orgSrcExt)

	klog.Infof("[Task] Id: %s, get lock prepare, srcName: %s, dstPrefix: %s, dst: %s", taskId, orgSrcName, dstPrefix, common.ToJson(dst))

	start := time.Now()
	for {
		if time.Since(start) > maxWait {
			err = errors.New("get lock timeout")
			break
		}

		ok, err = redisutils.RedisClient.SetNX(key, 1, lockTTL).Result()
		if err != nil {
			err = fmt.Errorf("get lock error: %v", err)
			break
		}
		if ok {

			klog.Infof("[Task] Id: %s, get the lock succeed", taskId)

			var checkName string = orgSrcName
			counter := 0
			for {
				var exists bool

				if counter > 0 {
					checkName = fmt.Sprintf("%s (%d)%s", orgSrcNamePrefix, counter, orgSrcExt)
				}

				if isOrgFile {
					uploadedParam := &models.FileParam{
						Owner:    dst.Owner,
						FileType: dst.FileType,
						Extend:   dst.Extend,
						Path:     filepath.Dir(dst.Path),
					}
					var resp []byte
					resp, err = seahub.GetUploadedBytes(uploadedParam, checkName)
					klog.Infof("[Task] Id: %s, check sync file %s, resp: %s, err: %v", taskId, checkName, string(resp), err)
					if err == nil {
						if string(resp) == "{\"uploadedBytes\":0}" {
							exists = false
						} else {
							exists = true
						}
					}
					var syncPrefixPath = files.GetPrefixPath(dst.Path)
					fileInfo := seahub.GetFileInfo(dst.Extend, syncPrefixPath+checkName)
					var syncObjId = fileInfo["obj_id"]
					if syncObjId != nil && syncObjId.(string) != "" {
						exists = true
					}
				}

				if !exists {
					break
				}
				counter++
			}

			if err != nil {
				break
			}

			newDstPath = dstPrefix + checkName
			if !isOrgFile {
				if !strings.HasSuffix(newDstPath, "/") {
					newDstPath += "/"
				}
			}

			break
		}

		time.Sleep(waitInterval)
	}

	if ok {
		released, e := redisutils.RedisClient.Del(key).Result()
		klog.Infof("[Task] Id: %s, released lock result: %d, error: %v", taskId, released, e)
	}

	if err != nil {
		return "", err
	}

	return newDstPath, nil

}

func (t *taskManager) ClearCacheFiles() {
	klog.Infof("Task remove cache files")
	users := global.GlobalData.GetGlobalUsers()

	var files []string
	var uploadCacheExpired = time.Now().AddDate(0, 0, -2)
	var downloadCacheExpired = time.Now().AddDate(0, 0, -2)
	var thumbCacheExpired = time.Now().AddDate(0, 0, -30)
	var bufferCacheExpired = time.Now().AddDate(0, 0, -5)

	for _, user := range users {

		// clear expired upload
		files = nil
		var pvcname = global.GlobalData.GetPvcCache(user)
		var uploadPath = fmt.Sprintf("%s/%s%s", common.CACHE_PREFIX, pvcname, common.DefaultUploadToCloudTempPath)
		filepath.Walk(uploadPath, func(path string, info fs.FileInfo, err error) error {
			if info != nil && info.ModTime().Before(uploadCacheExpired) {
				files = append(files, path)
			}
			return nil
		})

		if len(files) > 0 {
			for _, f := range files {
				if err := os.Remove(f); err != nil {
					klog.Errorf("remove file %s error: %v", err, f)
				}
			}
		}

		// clear expired download
		files = nil
		var downloadPath = fmt.Sprintf("%s/%s%s", common.CACHE_PREFIX, pvcname, common.DefaultSyncUploadToCloudTempPath)
		filepath.Walk(downloadPath, func(path string, info fs.FileInfo, err error) error {
			if info != nil && info.IsDir() {
				if info.ModTime().Before(downloadCacheExpired) {
					files = append(files, path)
				}
			}
			return nil
		})

		if len(files) > 0 {
			for _, f := range files {
				if err := os.RemoveAll(f); err != nil {
					klog.Errorf("remove dir %s error: %v", err, f)
				}
			}
		}

		// clear expired thumb
		files = nil
		var thumbPath = fmt.Sprintf("%s/%s%s%s", common.CACHE_PREFIX, pvcname, common.DefaultLocalFileCachePath, common.CacheThumb)
		filepath.Walk(thumbPath, func(path string, info fs.FileInfo, err error) error {
			if info != nil && info.ModTime().Before(thumbCacheExpired) {
				files = append(files, path)
			}
			return nil
		})

		if len(files) > 0 {
			for _, f := range files {
				if err := os.Remove(f); err != nil {
					klog.Errorf("remove file %s error: %v", err, f)
				}
			}
		}

		// clear expired buffer
		files = nil
		var bufferPath = fmt.Sprintf("%s/%s%s%s", common.CACHE_PREFIX, pvcname, common.DefaultLocalFileCachePath, common.CacheBuffer)
		filepath.Walk(bufferPath, func(path string, info fs.FileInfo, err error) error {
			if info != nil && info.IsDir() {
				if info.ModTime().Before(bufferCacheExpired) {
					files = append(files, path)
				}
			}
			return nil
		})

		if len(files) > 0 {
			for _, f := range files {
				if err := os.RemoveAll(f); err != nil {
					klog.Errorf("remove dir %s error: %v", err, f)
				}
			}
		}

	}

}

func (t *taskManager) GenerateKeepFile() {
	var keepFilePath = common.DefaultLocalRootPath + common.DefaultKeepFileName
	if err := files.CheckKeepFile(keepFilePath); err != nil {
		klog.Errorf("generate keep file error: %v", err)
	}
}

func (t *taskManager) formatFilePathWithoutTask(s string, taskId string) string {
	if !strings.Contains(s, taskId) {
		return s
	}

	var pos = strings.Index(s, taskId)
	return s[pos+len(taskId):]
}
