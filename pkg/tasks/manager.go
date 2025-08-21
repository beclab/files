package tasks

import (
	"context"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
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

type TaskManagerInterface interface {
	CreateTask(taskType TaskType, param *models.PasteParam) *Task
}

type taskManager struct {
	task sync.Map
	pool pond.Pool //pond.ResultPool[string]
}

func NewTaskManager() {
	TaskManager = &taskManager{
		task: sync.Map{},
		pool: pond.NewPool(1, pond.WithContext(context.Background()), pond.WithNonBlocking(true)),
	}
}

func (t *taskManager) CreateTask(param *models.PasteParam) *Task {

	var ctx, cancel = context.WithCancel(context.Background())
	var _, isFile = param.Src.IsFile()

	var task = &Task{
		id:      t.generateTaskId(),
		param:   param,
		state:   common.Pending,
		ctx:     ctx,
		cancel:  cancel,
		manager: t,
		isFile:  isFile,
	}

	return task

}

func (t *taskManager) CancelTask(taskId string) {
	val, ok := t.task.Load(taskId)
	if !ok {
		return
	}

	task := val.(*Task)
	task.cancel()
	task.canceled = true
}

func (t *taskManager) GetTask(taskId string) *TaskInfo {
	val, ok := t.task.Load(taskId)
	if !ok {
		return nil
	}

	task := val.(*Task)

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
		Progress:      task.progress,
		Transferred:   task.transfer,
		TotalFileSize: task.totalSize,
		TidyDirs:      task.tidyDirs,
		Status:        task.state,
		ErrorMessage:  task.message,
	}

	return res
}

func (t *taskManager) generateTaskId() string {
	var n = time.Now()
	var id = n.UnixNano()
	return fmt.Sprintf("task%d", id)
}
