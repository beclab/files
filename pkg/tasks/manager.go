package tasks

import (
	"context"
	"files/pkg/constant"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/paste/commands"
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

	return &Task{
		taskType: taskType,
		id:       t.generateTaskId(),
		param:    param,
		state:    constant.Pending,
		ctx:      ctx,
		cancel:   cancel,
		manager:  t,
		command:  t.buildCommand(ctx, taskType, param),
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
		IsDir:         true,
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

func (t *taskManager) buildCommand(ctx context.Context, taskType TaskType, param *models.PasteParam) *commands.Command {
	var command = commands.NewCommand(ctx, param)

	var f func() error

	switch taskType {

	case Rsync:
		f = command.Rsync
	case DownloadFromFiles:
		f = command.DownloadFromFiles
	case DownloadFromSync:
		f = command.DownloadFromSync
	case DownloadFromCloud:
		f = command.DownloadFromCloud
	case UploadToSync:
		f = command.UploadToSync
	case UploadToCloud:
		f = command.UploadToCloud
	case SyncCopy:
		f = command.SyncCopy
	case CloudCopy:
		f = command.CloudCopy
	}

	command.Exec = f

	return command
}

func (t *taskManager) generateTaskId() string {
	var n = time.Now()
	var id = n.UnixNano()
	return fmt.Sprintf("task%d", id)
}
