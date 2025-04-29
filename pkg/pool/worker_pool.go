package pool

import (
	"context"
	"fmt"
	"github.com/alitto/pond/v2"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

var (
	TaskManager = sync.Map{} // 存储任务状态
	WorkerPool  pond.Pool
)

type Task struct {
	ID             string   `json:"id"`
	Source         string   `json:"source"`
	Dest           string   `json:"dest"`
	SrcType        string   `json:"src_type"`
	DstType        string   `json:"dst_type"`
	Status         string   `json:"status"`
	Progress       int      `json:"progress"`
	Log            []string `json:"log"`
	RelationTaskID string   `json:"relation_task_id"` // only for same cache now
	RelationNode   string   `json:"relation_node"`    // only for same cache now (for get task progress and cancel task)
	mu             sync.Mutex
	Ctx            context.Context    `json:"-"`
	Cancel         context.CancelFunc `json:"-"`
	LogChan        chan string        `json:"-"`
	ProgressChan   chan int           `json:"-"`
	ErrChan        chan error         `json:"-"`
	timer          *time.Timer        // 新增定时器字段
}

type FormattedTask struct {
	Task
}

func (ft FormattedTask) String() string {
	return fmt.Sprintf("ID: %s, Source: %s, Dest: %s, SrcType: %s, DstType: %s, Status: %s, Progress: %d, Log: %v, RelationTaskID: %s, RelationNode: %s",
		ft.ID, ft.Source, ft.Dest, ft.SrcType, ft.DstType, ft.Status, ft.Progress, ft.Log, ft.RelationTaskID, ft.RelationNode)
}

func NewTask(id, source, dest, srcType, dstType string) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	task := &Task{
		ID:             id,
		Source:         source,
		Dest:           dest,
		SrcType:        srcType,
		DstType:        dstType,
		Status:         "pending",
		Progress:       0,
		Log:            make([]string, 0),
		RelationTaskID: "",
		RelationNode:   "",
		Ctx:            ctx,
		Cancel:         cancel,
		LogChan:        make(chan string, 100),
		ProgressChan:   make(chan int, 100),
		ErrChan:        make(chan error, 10),
	}

	// 新增6小时过期逻辑
	task.timer = time.AfterFunc(6*time.Hour, func() {
		close(task.ErrChan)
		close(task.LogChan)
		close(task.ProgressChan)
		task.Cancel()
		TaskManager.Delete(task.ID) // 从TaskManager删除
	})
	return task
}

func ProcessProgress(progress, lowerBound, upperBound int) int {
	// 确保下限小于等于上限
	if lowerBound > upperBound {
		lowerBound, upperBound = upperBound, lowerBound
	}

	// 确保 progress 在默认范围 0~100 之间
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}

	// 计算缩放比例
	scale := float64(upperBound-lowerBound) / 100.0

	// 计算缩放后的值，并向下取整
	scaledProgress := lowerBound + int(float64(progress)*scale)

	return scaledProgress
}

func (t *Task) UpdateProgress() {
	klog.Infof("~~~Temp log: Update Progress From Rsync [%s] ~~~", t.ID)
	t.mu.Lock()
	t.Status = "running"
	t.Progress = 0
	t.Log = []string{}
	TaskManager.Store(t.ID, t)
	t.mu.Unlock()
	klog.Infof("~~~Temp log: %s", FormattedTask{Task: *t})

	timeout := time.After(24 * time.Hour) // 合理超时时间

	lastHeartbeat := time.Now()
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	var logs []string = []string{}

	for {
		select {
		case <-t.Ctx.Done():
			t.mu.Lock()
			for _, log := range logs {
				klog.Infof("~~~Temp log: logging {%s}", log)
				t.Log = append(t.Log, log)
			}
			if t.Progress < 100 {
				t.Status = "cancelled"
			} else {
				t.Status = "completed"
			}
			TaskManager.Store(t.ID, t)
			t.mu.Unlock()
			klog.Infof("Task %s cancelled", t.ID)
			return
		case <-heartbeatTicker.C:
			if time.Since(lastHeartbeat) > 45*time.Second {
				klog.Errorf("Task %s heartbeat lost", t.ID)
				return
			}
		case <-timeout:
			klog.Errorf("Task %s timeout", t.ID)
			return
		case log, ok := <-t.LogChan:
			if !ok {
				// 通道关闭，可以记录日志或直接退出
				klog.Infof("Task %s log channel closed", t.ID)
				break
			}

			klog.Infof("[%s] %s with log count %d", t.ID, log, len(logs))
			//logs = append(logs, log)
			t.Log = append(t.Log, log)
			//t.mu.Lock()
			//t.Logging(log)
			//TaskManager.Store(t.ID, t)
			//t.mu.Unlock()
			//klog.Infof("[%s] %s", t.ID, FormattedTask{Task: *t})

		case progress, ok := <-t.ProgressChan:
			if !ok {
				// 通道关闭，任务完成
				t.mu.Lock()
				t.Status = "completed"
				for _, log := range logs {
					klog.Infof("~~~Temp log: logging {%s}", log)
					t.Log = append(t.Log, log)
				}
				TaskManager.Store(t.ID, t)
				t.mu.Unlock()
				return
			}
			klog.Infof("[%s] %d", t.ID, progress)
			processedProgress := 0
			if progress > 0 {
				processedProgress = progress //ProcessProgress(progress, 0)
			}

			if t.Progress == 100 && processedProgress == 100 {
				CancelTask(t.ID, false)
			} else {
				t.mu.Lock()
				t.Progress = processedProgress
				TaskManager.Store(t.ID, t)
				t.mu.Unlock()
				klog.Infof("[%s] %s", t.ID, FormattedTask{Task: *t})
			}

			//if t.Progress == 100 {
			//	CancelTask(t.ID, false)
			//}

		default:
			// 避免完全阻塞，可以添加短暂休眠
			time.Sleep(10 * time.Millisecond)
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
func CancelTask(taskID string, delete bool) {
	if task, ok := TaskManager.Load(taskID); ok {
		if t, ok := task.(*Task); ok {
			close(t.ErrChan)
			close(t.LogChan)
			close(t.ProgressChan)
			t.Cancel()
			if t.timer != nil {
				t.timer.Stop() // 停止定时器防止泄漏
			}

			// after cancel, delete info
			if delete {
				TaskManager.Delete(taskID)
				// 可选：等待任务资源清理或执行其他清理逻辑
				klog.Infof("Task %s has been cancelled", taskID)
			} else {
				klog.Infof("Task %s has completed", taskID)
			}
		}
	} else {
		klog.Infof("Task %s not found", taskID)
	}
}
