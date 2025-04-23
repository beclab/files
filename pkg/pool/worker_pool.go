package pool

import (
	"context"
	"github.com/alitto/pond/v2"
	"k8s.io/klog/v2"
	"sync"
)

var (
	TaskManager = sync.Map{} // 存储任务状态
	WorkerPool  pond.Pool
)

type Task struct {
	ID       string
	Source   string
	Dest     string
	Status   string
	Progress int
	Log      []string
	mu       sync.Mutex
	Ctx      context.Context
	Cancel   context.CancelFunc
}

func NewTask(id, source, dest string) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	return &Task{
		ID:     id,
		Source: source,
		Dest:   dest,
		Status: "pending",
		Ctx:    ctx,
		Cancel: cancel,
	}
}

func ProcessProgress(progress, progressType int) int {
	// TODO: 根据 progressType 处理进度
	return progress
}

func (t *Task) UpdateProgressFromRsync(progressChan chan int) {
	klog.Infof("~~~Temp log: Update Progress From Rsync [%s] ~~~", t.ID)
	t.mu.Lock()
	t.Status = "running"
	t.Progress = 0
	t.Log = []string{}
	TaskManager.Store(t.ID, t)
	t.mu.Unlock()
	klog.Infof("~~~Temp log: %v", t)

	for {
		select {
		case <-t.Ctx.Done():
			klog.Infof("Task %s cancelled", t.ID)
			return
		case progress, ok := <-progressChan:
			if !ok {
				// 通道关闭，任务完成
				t.mu.Lock()
				t.Status = "completed"
				TaskManager.Store(t.ID, t)
				t.mu.Unlock()
				return
			}
			klog.Infof("[%s] %d", t.ID, progress)
			processedProgress := 0
			if progress > 0 {
				processedProgress = ProcessProgress(progress, 0)
			}
			t.mu.Lock()
			t.Progress = processedProgress
			TaskManager.Store(t.ID, t)
			t.mu.Unlock()
			klog.Infof("[%s] %v", t.ID, t)
		}
	}
}

// Logging 添加日志条目
func (t *Task) Logging(entry string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Log = append(t.Log, entry)
}

// LoggingError 添加错误日志条目
func (t *Task) LoggingError(entry string) {
	t.Logging("[ERROR] " + entry)
}

// GetProgress 获取任务进度
func (t *Task) GetProgress() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Progress
}

// CancelTask 取消任务
func CancelTask(taskID string) {
	if task, ok := TaskManager.Load(taskID); ok {
		if t, ok := task.(*Task); ok {
			t.Cancel()
			// 可选：等待任务资源清理或执行其他清理逻辑
			klog.Infof("Task %s has been cancelled", taskID)
		}
	} else {
		klog.Infof("Task %s not found", taskID)
	}
}
