package tasks

import (
	"context"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
	"k8s.io/klog/v2"
)

var TaskManager *taskManager

type TaskType int

const (
	TypeRsync TaskType = iota

	TypeDownloadFromFiles
	TypeDownloadFromSync
	TypeDownloadFromCloud

	TypeUploadToSync
	TypeUploadToCloud

	TypeSyncCopy
	TypeCloudCopy
)

var TaskCancel = "context canceled"

type TaskManagerInterface interface {
	CreateTask(taskType TaskType, param *models.PasteParam) *Task
}

type taskManager struct {
	userPools sync.Map
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
		id:        t.generateTaskId(),
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

	if task.state != common.Paused { // todo
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

	var res = &TaskInfo{
		Id:            task.id,
		Action:        task.param.Action,
		IsDir:         !task.isFile,
		FileName:      srcFileName,
		Dst:           dstUri,
		DstPath:       dstFileName,
		DstFileType:   dst.FileType,
		Src:           srcUri,
		SrcFileType:   src.FileType,
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
			DstFileType:   dst.FileType,
			Src:           srcUri,
			SrcFileType:   src.FileType,
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

func (t *taskManager) generateTaskId() string {
	var n = time.Now()
	var id = n.UnixNano()
	return fmt.Sprintf("task%d", id)
}
