package tasks

import (
	"context"
	"files/pkg/constant"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"fmt"

	"k8s.io/klog/v2"
)

type Task struct {
	taskType TaskType
	id       string
	param    *models.PasteParam
	state    string
	message  string
	progress int

	ctx    context.Context
	cancel context.CancelFunc

	command *commands.Command

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
		var err error

		defer func() {
			klog.Infof("Task Id: %s done, status: %s", t.id, t.state)
		}()

		klog.Infof("Task Id: %s", t.id)
		t.state = constant.Running
		t.command.Update = t.updateProgress

		if err = t.command.Exec(); err != nil {
			klog.Errorf("Task Failed: %v", err)
			t.state = constant.Failed
			t.state = err.Error()
			return
		}

		t.state = constant.Completed

		return
	})

	if !ok {
		return fmt.Errorf("submit worker failed")
	}

	return nil
}

func (t *Task) updateProgress(progress int) {
	t.progress = progress
}
