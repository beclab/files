package background_task

import (
	"context"
	"files/pkg/drives"
	"files/pkg/rpc"
	"sync"
	"time"
)

type TaskType int

const (
	OnceTask TaskType = iota
	PeriodicTask
)

type Task struct {
	name     string
	taskFunc func(context.Context)
	taskType TaskType
	interval time.Duration // only for PeriodicTask
	ticker   *time.Ticker
}

type TaskManager struct {
	tasks []Task
	wg    sync.WaitGroup
}

func NewTaskManager() *TaskManager {
	return &TaskManager{}
}

func (tm *TaskManager) RegisterTask(task Task) {
	tm.tasks = append(tm.tasks, task)
}

func (tm *TaskManager) Start(ctx context.Context) {
	for _, task := range tm.tasks {
		tm.wg.Add(1)
		go tm.runTask(ctx, task)
	}
	tm.wg.Wait()
}

func (tm *TaskManager) runTask(ctx context.Context, task Task) {
	defer tm.wg.Done()
	switch task.taskType {
	case OnceTask:
		task.taskFunc(ctx)
	case PeriodicTask:
		if task.ticker == nil {
			task.ticker = time.NewTicker(task.interval)
		}
		defer task.ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-task.ticker.C:
				task.taskFunc(ctx)
			}
		}
	}
}

func InitBackgroundTaskManager(ctx context.Context) {
	manager := NewTaskManager()

	manager.RegisterTask(Task{
		name:     "OnceTask",
		taskFunc: rpc.InitRpcService,
		taskType: OnceTask,
		interval: 0,
		ticker:   nil,
	})

	manager.RegisterTask(Task{
		name:     "PeriodicTask",
		taskFunc: drives.GetMountedData,
	})

	manager.Start(ctx)
}
