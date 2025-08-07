package tasks

import (
	"context"
	"files/pkg/constant"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/paste/handlers"
	"fmt"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
)

var TaskManager *taskManager

type TaskType int

const (
	Rsync TaskType = iota

	DownloadFromFiles
	DownloadFromSync
	DownloadFromCloud

	UploadToSync
	UploadToCloud

	SyncCopy
	CloudCopy
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

func (t *taskManager) CreateTask(taskType TaskType, param *models.PasteParam) *Task {

	var ctx, cancel = context.WithCancel(context.Background())
	var _, isFile = param.Src.IsFile()

	return &Task{
		taskType: taskType,
		id:       t.generateTaskId(),
		param:    param,
		state:    constant.Pending,
		ctx:      ctx,
		cancel:   cancel,
		manager:  t,
		isFile:   isFile,
		handler:  t.buildHandler(ctx, taskType, param),
	}
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
	var dstFileName = fileutils.GetPathName(dst.Path)
	var srcFileName = fileutils.GetPathName(src.Path)

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
		Status:        task.state,
		ErrorMessage:  task.message,
	}

	return res
}

func (t *taskManager) buildHandler(ctx context.Context, taskType TaskType, param *models.PasteParam) *handlers.Handler {
	var handler = handlers.NewHandler(ctx, param)

	var f func() error

	switch taskType {

	case Rsync:
		f = handler.Rsync
	case DownloadFromFiles:
		f = handler.DownloadFromFiles
	case DownloadFromSync:
		f = handler.DownloadFromSync
	case DownloadFromCloud:
		f = handler.DownloadFromCloud
	case UploadToSync:
		f = handler.UploadToSync
	case UploadToCloud:
		f = handler.UploadToCloud
	case SyncCopy:
		f = handler.SyncCopy
	case CloudCopy:
		f = handler.CloudCopy
	}

	handler.Exec = f

	return handler
}

func (t *taskManager) generateTaskId() string {
	var n = time.Now()
	var id = n.UnixNano()
	return fmt.Sprintf("task%d", id)
}
