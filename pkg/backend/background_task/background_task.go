package background_task

import (
	"context"
	"files/pkg/backend/rpc"
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
		ticker := time.NewTicker(task.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
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
	})

	manager.Start(ctx)
}
