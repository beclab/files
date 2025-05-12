package background_task

import (
	"context"
	"files/pkg/drives"
	"files/pkg/rpc"
	"k8s.io/klog/v2"
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
	tasks        []Task
	wg           sync.WaitGroup
	periodicWg   sync.WaitGroup
	periodicStop chan struct{}
	isRunning    bool
	mu           sync.Mutex
	stopOnce     sync.Once
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
		klog.Infoln("run once task", task.name)
		task.taskFunc(ctx)
	case PeriodicTask:
		tm.periodicWg.Add(1)
		go func() {
			defer tm.periodicWg.Done()
			tm.runPeriodicTask(ctx, task)
		}()
	}
}

func (tm *TaskManager) runPeriodicTask(ctx context.Context, task Task) {
	klog.Infoln("run periodic task", task.name)
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if task.ticker == nil {
		task.ticker = time.NewTicker(task.interval)
	}
	defer task.ticker.Stop()

	for {
		select {
		case <-tm.periodicStop:
			klog.Infoln("Periodic task stopped:", task.name)
			return
		case <-task.ticker.C:
			task.taskFunc(taskCtx)
		case <-ctx.Done():
			klog.Infoln("Periodic task canceled by context:", task.name)
			return
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

	manager.RegisterTask(Task{
		name:     "GetMountedData",
		taskFunc: drives.GetMountedData,
		taskType: PeriodicTask,
		interval: 2 * time.Minute,
		ticker:   drives.MountedTicker,
	})

	go manager.Start(ctx)
}
