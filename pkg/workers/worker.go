package workers

import (
	"context"
	"errors"
	"files/pkg/models"
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

func NewTask(rootCtx context.Context, srcFileParam, dstFileParam *models.FileParam, f func(contextCtx context.Context, a, b *models.FileParam) error) (*Task, error) {
	var id = "task-id-1" // generate task id
	var ctx, cancel = context.WithCancel(rootCtx)

	var task = &Task{
		Id:         id,
		Src:        srcFileParam,
		Dst:        dstFileParam,
		Ctx:        ctx,
		CancelFunc: cancel,
		CreateAt:   time.Now(),
	}

	_, ok := WorkerPool.TrySubmitErr(func() (string, error) {
		defer func() {
			// 	todo do sth
		}()

		klog.Infof("Task %s, user: %s, src: %s, dst: %s", id, srcFileParam.Owner, srcFileParam.Path, dstFileParam.Path)

		_, ok := TaskManager.Load(task.Id)
		if ok {
			return "", fmt.Errorf("task exists, id: %s", task.Id)
		} else {
			TaskManager.Store(task.Id, task)
		}

		task.State = "running"

		// long job
		if err := f(ctx, srcFileParam, dstFileParam); err != nil {
			return "", err
		}
		return "", nil
	})

	if !ok {
		return nil, errors.New("task add failed")
	}

	return task, nil
}

func (t *Task) QueryTask(taskId string) {
	// todo
}

func (t *Task) CancelTask(taskId string) {
	// todo
}

func (t *Task) QueryList(latest bool) {
	// todo
}
