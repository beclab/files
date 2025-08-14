package tasks

import (
	"context"
	"files/pkg/common"
	"files/pkg/models"
	"files/pkg/paste/handlers"
	"fmt"

	"k8s.io/klog/v2"
)

type Task struct {
	taskType  TaskType
	id        string
	param     *models.PasteParam
	state     string
	message   string
	progress  int
	transfer  int64
	totalSize int64
	isFile    bool

	ctx      context.Context
	cancel   context.CancelFunc
	canceled bool

	handler *handlers.Handler

	manager *taskManager
}

func (t *Task) Id() string {
	return t.id
}

func (t *Task) Run() error {
	_, loaded := t.manager.task.LoadOrStore(t.id, t)
	if loaded {
		return fmt.Errorf("task %s exists in taskManager", t.id)
	}

	_, ok := t.manager.pool.TrySubmit(func() {
		if t.canceled {
			t.state = common.Cancelled
			return
		}
		var err error

		defer func() {
			klog.Infof("Task Id: %s done, status: %s, progress: %d, size: %d, transfer: %d", t.id, t.state, t.progress, t.totalSize, t.transfer)
		}()

		klog.Infof("Task Id: %s", t.id)
		t.state = common.Running
		t.handler.UpdateProgress = t.updateProgress
		t.handler.UpdateTotalSize = t.updateTotalSize
		t.handler.GetProgress = t.GetProgress

		if err = t.handler.Exec(); err != nil {
			klog.Errorf("Task Failed: %v", err)
			if err.Error() == "context canceled" {
				t.state = common.Cancelled
			} else {
				t.state = common.Failed
			}
			t.message = err.Error()
			return
		}

		t.state = common.Completed
		t.progress = 100

		return
	})

	if !ok {
		return fmt.Errorf("submit worker failed")
	}

	return nil
}

func (t *Task) updateProgress(progress int, transfer int64) {
	t.progress = progress
	t.transfer = transfer
}

func (t *Task) updateTotalSize(totalSize int64) {
	t.totalSize = totalSize
}

func (t *Task) GetProgress() (string, int, int64, int64) {
	return t.state, t.progress, t.transfer, t.totalSize
}
