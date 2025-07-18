package workers

import (
	"context"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"files/pkg/utils"
	"fmt"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
	"k8s.io/klog/v2"
)

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

var (
	TaskManager = sync.Map{}
	WorkerPool  pond.ResultPool[string]
)

func init() {
	WorkerPool = pond.NewResultPool[string](1)
}

type exec func(f func() error) error

type Task struct {
	Id     string `json:"id"`
	Owner  string `json:"owner"`
	Action string `json:"action"`

	Src *models.FileParam
	Dst *models.FileParam

	State string `json:"state"` // running pending failed

	CreateAt time.Time `json:"createAt"`

	Ctx        context.Context    `json:"-"`
	CancelFunc context.CancelFunc `json:"-"`
}

func NewTaskId() string {
	return utils.NewUUID()
}

// command commands.CommandInterface
func SubmitTask(taskId string, taskType TaskType, param *models.PasteParam) (*Task, error) {

	var task = &Task{

		Id:     taskId,
		Owner:  param.Owner,
		Action: param.Action,
		Src:    param.Src,
		Dst:    param.Dst,
	}

	var command = commands.NewCommand(param)

	var f func() error

	switch taskType {

	case Rsync:
		f = command.Rsync
		if param.Action == "move" {
			f = command.Move
		}
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

	if err := addQueue(task, f); err != nil {
		return nil, err
	}

	return task, nil
}

func addQueue(task *Task, f func() error) error {
	_, loaded := TaskManager.LoadOrStore(task.Id, task)
	if loaded {
		return fmt.Errorf("task exists, id: %s", task.Id)
	}

	_, ok := WorkerPool.TrySubmitErr(func() (string, error) {
		var err error
		defer func() {
			//
		}()

		klog.Infof("Task %s", task.Id)

		task.State = "running"
		if err = f(); err != nil {
			task.State = "failed"
			return "", err
		}

		task.State = "complete"
		return "", nil
	})

	if !ok {
		return fmt.Errorf("submit worker failed")
	}

	return nil
}
