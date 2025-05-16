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
	ID             string             `json:"id"`
	Source         string             `json:"source"`
	Dest           string             `json:"dest"`
	SrcType        string             `json:"src_type"`
	DstType        string             `json:"dst_type"`
	Cancellable    bool               `json:"cancellable"`
	IsDir          bool               `json:"is_dir"`
	FileType       string             `json:"file_type"`
	Filename       string             `json:"filename"`
	Status         string             `json:"status"`
	Progress       int                `json:"progress"`
	TotalFileSize  int64              `json:"total_file_size"`
	Transferred    int64              `json:"transferred"`
	Log            []string           `json:"log"`
	RelationTaskID string             `json:"relation_task_id"` // only for same cache now
	RelationNode   string             `json:"relation_node"`    // only for same cache now (for get task progress and cancel task)
	Mu             sync.Mutex         `json:"-"`
	Ctx            context.Context    `json:"-"`
	Cancel         context.CancelFunc `json:"-"`
	LogChan        chan string        `json:"-"`
	ProgressChan   chan int           `json:"-"`
	ErrChan        chan error         `json:"-"`
	timer          *time.Timer
	cancelOnce     *sync.Once
}

type FormattedTask struct {
	Task
}

func (ft FormattedTask) String() string {
	return fmt.Sprintf("ID: %s, Source: %s, Dest: %s, SrcType: %s, DstType: %s, Cancellable: %v, IsDir: %v, FileType: %s, Filename: %s, Status: %s, Progress: %d, TotalFileSize: %d, Transferred: %d, Log: %v, RelationTaskID: %s, RelationNode: %s",
		ft.ID, ft.Source, ft.Dest, ft.SrcType, ft.DstType, ft.Cancellable, ft.IsDir, ft.FileType, ft.Filename, ft.Status, ft.Progress, ft.TotalFileSize, ft.Transferred, ft.Log, ft.RelationTaskID, ft.RelationNode)
}

func NewTask(id, source, dest, srcType, dstType string, cancellable bool, isDir bool, fileType, filename string) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	task := &Task{
		ID:             id,
		Source:         source,
		Dest:           dest,
		SrcType:        srcType,
		DstType:        dstType,
		Cancellable:    cancellable,
		IsDir:          isDir,
		FileType:       fileType,
		Filename:       filename,
		Status:         "pending",
		Progress:       0,
		TotalFileSize:  0,
		Transferred:    0,
		Log:            make([]string, 0),
		RelationTaskID: "",
		RelationNode:   "",
		Ctx:            ctx,
		Cancel:         cancel,
		LogChan:        make(chan string, 100),
		ProgressChan:   make(chan int, 100),
		ErrChan:        make(chan error, 10),
		cancelOnce:     new(sync.Once),
	}

	// 新增6小时过期逻辑
	task.timer = time.AfterFunc(6*time.Hour, func() {
		CancelTask(task.ID, true)
		task.cancelOnce.Do(func() {
			close(task.ErrChan)
			close(task.LogChan)
			close(task.ProgressChan)
			task.Cancel()
		})
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

//func (t *Task) UpdateProgress() {
//	if t.Status == "completed" {
//		return
//	}
//
//	klog.Infof("~~~Temp log: Update Progress From Rsync [%s] ~~~", t.ID)
//	t.Mu.Lock()
//	t.Status = "running"
//	t.Progress = 0
//	t.Log = []string{}
//	TaskManager.Store(t.ID, t)
//	t.Mu.Unlock()
//	klog.Infof("~~~Temp log: %s", FormattedTask{Task: *t})
//
//	timeout := time.After(24 * time.Hour) // 合理超时时间
//
//	lastHeartbeat := time.Now()
//	heartbeatTicker := time.NewTicker(30 * time.Second)
//	defer heartbeatTicker.Stop()
//
//	var logs []string = []string{}
//
//	for {
//		select {
//		case <-t.Ctx.Done():
//			t.Mu.Lock()
//			for _, log := range logs {
//				klog.Infof("~~~Temp log: logging {%s}", log)
//				t.Log = append(t.Log, log)
//			}
//			if t.Progress >= 100 {
//				t.Status = "completed"
//				klog.Infof("Task %s completed with progress %d", t.ID, t.Progress)
//			} else {
//				t.Status = "cancelled"
//				klog.Warningf("Task %s cancelled with unexpected progress %d", t.ID, t.Progress)
//			}
//			TaskManager.Store(t.ID, t)
//			t.Mu.Unlock()
//			if t.Status == "cancelled" {
//				klog.Infof("Task %s cancelled with Progress %d", t.ID, t.Progress)
//			} else if t.Status == "completed" {
//				klog.Infof("Task %s completed with Progress %d", t.ID, t.Progress)
//			} else {
//				klog.Infof("Task %s failed with status %s and Progress %d", t.ID, t.Status, t.Progress)
//			}
//
//			return
//		case <-heartbeatTicker.C:
//			if time.Since(lastHeartbeat) > 45*time.Second {
//				klog.Errorf("Task %s heartbeat lost", t.ID)
//				return
//			}
//		case <-timeout:
//			klog.Errorf("Task %s timeout", t.ID)
//			return
//		case err, ok := <-t.ErrChan:
//			if !ok {
//				klog.Infof("Task %s log channel closed", t.ID)
//				break
//			}
//
//			klog.Infof("[%s Error] %v with log count %d", t.ID, err, len(logs))
//			t.Log = append(t.Log, fmt.Sprintf("%v", err))
//		case log, ok := <-t.LogChan:
//			if !ok {
//				klog.Infof("Task %s log channel closed", t.ID)
//				break
//			}
//
//			if log != "" {
//				klog.Infof("[%s] %s with log count %d", t.ID, log, len(logs))
//				//logs = append(logs, log)
//				t.Log = append(t.Log, log)
//			} else {
//				klog.Infof("[%s] %s with log count %d", t.ID, "a discarded empty log", len(logs))
//			}
//			//t.Mu.Lock()
//			//t.Logging(log)
//			//TaskManager.Store(t.ID, t)
//			//t.Mu.Unlock()
//			//klog.Infof("[%s] %s", t.ID, FormattedTask{Task: *t})
//
//		case progress, ok := <-t.ProgressChan:
//			if !ok {
//				// 通道关闭，任务完成
//				t.Mu.Lock()
//				if t.Progress >= 100 {
//					t.Status = "completed"
//				} else {
//					t.Status = "cancelled" // 或根据实际逻辑调整状态
//				}
//				for _, log := range logs {
//					klog.Infof("~~~Temp log: logging {%s}", log)
//					t.Log = append(t.Log, log)
//				}
//				TaskManager.Store(t.ID, t)
//				t.Mu.Unlock()
//				return
//			}
//			klog.Infof("[%s] %d %s %d", t.ID, t.Progress, t.Status, progress)
//
//			if t.Status == "completed" {
//				klog.Infof("task [%s] already completed", t.ID)
//				break
//			}
//
//			processedProgress := 0
//			if progress > 0 {
//				processedProgress = progress //ProcessProgress(progress, 0)
//			}
//
//			if t.Progress == 100 && processedProgress == 100 {
//				select {
//				case <-time.After(time.Second):
//					CompleteTask(t.ID)
//				case <-t.LogChan:
//					CompleteTask(t.ID)
//				case <-t.ErrChan:
//					CompleteTask(t.ID)
//				}
//			} else {
//				t.Mu.Lock()
//				t.Progress = processedProgress
//				TaskManager.Store(t.ID, t)
//				t.Mu.Unlock()
//				klog.Infof("[%s] %s", t.ID, FormattedTask{Task: *t})
//			}
//
//		default:
//			// 避免完全阻塞，可以添加短暂休眠
//			time.Sleep(10 * time.Millisecond)
//		}
//	}
//}

func (t *Task) UpdateProgress() {
	if t.Status == "completed" {
		return
	}

	klog.Infof("~~~Temp log: Update Progress From Rsync [%s] ~~~", t.ID)
	t.Mu.Lock()
	t.Status = "running"
	t.Progress = 0
	t.Log = []string{}
	TaskManager.Store(t.ID, t)
	t.Mu.Unlock()
	klog.Infof("~~~Temp log: %s", FormattedTask{Task: *t})

	// 新增：启动独立日志收集协程
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case log, ok := <-t.LogChan:
				if !ok {
					klog.Infof("Task %s log channel closed", t.ID)
					return
				}
				if log != "" {
					t.Mu.Lock()
					t.Log = append(t.Log, log)
					t.Mu.Unlock()
					klog.Infof("[%s] Log collected: %s (total: %d)", t.ID, log, len(t.Log))
				}
			case <-t.Ctx.Done(): // 收到退出信号时退出循环
				klog.Infof("Task %s log collector exiting gracefully", t.ID)
				return
			}
		}
	}()

	timeout := time.After(24 * time.Hour)
	lastHeartbeat := time.Now()
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-t.Ctx.Done():
			// 关闭日志通道并等待收集完成
			//close(t.LogChan)
			wg.Wait()

			t.Mu.Lock()
			//if t.Status == "completed" {
			//	t.Progress = 100
			//} else {
			if t.Progress < 100 {
				if t.Status != "failed" {
					t.Status = "cancelled"
				}
			} else {
				t.Status = "completed"
			}
			//}
			TaskManager.Store(t.ID, t)
			t.Mu.Unlock()

			if t.Status == "cancelled" {
				klog.Infof("Task %s cancelled with Progress %d", t.ID, t.Progress)
			} else if t.Status == "completed" {
				klog.Infof("Task %s completed with Progress %d", t.ID, t.Progress)
			} else {
				klog.Infof("Task %s failed with status %s and Progress %d", t.ID, t.Status, t.Progress)
			}
			return

		case <-heartbeatTicker.C:
			if time.Since(lastHeartbeat) > 45*time.Second {
				klog.Errorf("Task %s heartbeat lost", t.ID)
				return
			}

		case <-timeout:
			klog.Errorf("Task %s timeout", t.ID)
			return

		case err, ok := <-t.ErrChan:
			if !ok {
				klog.Infof("Task %s error channel closed", t.ID)
				break
			}
			klog.Errorf("[%s Error] %v", t.ID, err)
			t.Mu.Lock()
			t.Log = append(t.Log, fmt.Sprintf("ERROR: %v", err))
			t.Mu.Unlock()

		case progress, ok := <-t.ProgressChan:
			if !ok {
				// 通道关闭处理
				//close(t.LogChan) // 关闭日志通道
				wg.Wait() // 等待日志收集完成

				//t.Mu.Lock()
				//t.Status = "completed"
				//t.Progress = 100
				//TaskManager.Store(t.ID, t)
				//t.Mu.Unlock()
				t.Mu.Lock()
				if t.Progress < 100 {
					if t.Status != "failed" {
						t.Status = "cancelled"
					}
				} else {
					t.Status = "completed"
				}
				TaskManager.Store(t.ID, t)
				t.Mu.Unlock()
				return
			}

			if t.Status == "completed" {
				klog.Infof("Task [%s] already completed", t.ID)
				break
			}

			processedProgress := 0
			if progress > 0 {
				processedProgress = progress
			}

			if t.Progress == 100 && processedProgress == 100 {
				select {
				case <-time.After(time.Second):
					CompleteTask(t.ID)
				case <-t.LogChan:
					CompleteTask(t.ID)
				case <-t.ErrChan:
					CompleteTask(t.ID)
				}
			} else {
				t.Mu.Lock()
				t.Progress = processedProgress
				TaskManager.Store(t.ID, t)
				t.Mu.Unlock()
				klog.Infof("[%s] %s", t.ID, FormattedTask{Task: *t})
			}

		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// Logging 添加日志条目
func (t *Task) Logging(entry string) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.Log = append(t.Log, entry)
}

// LoggingError 添加错误日志条目
func (t *Task) LoggingError(entry string) {
	t.Logging("[ERROR] " + entry)
}

// GetProgress 获取任务进度
func (t *Task) GetProgress() int {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	return t.Progress
}

// CancelTask 取消任务
func CancelTask(taskID string, delete bool) {
	if task, ok := TaskManager.Load(taskID); ok {
		if t, ok := task.(*Task); ok {
			if !t.Cancellable {
				klog.Infof("[%s] is not cancellable", taskID)
				return
			}

			if t.Status == "cancelled" {
				klog.Infof("Task %s has already been cancelled, cannot be cancalled again", taskID)
				return
			} else if t.Status == "completed" {
				klog.Infof("Task %s has already completed, cannot be cancelled any more", taskID)
				return
			} else if t.Status == "failed" {
				klog.Infof("Task %s has failed, cannot be cancelled any more", taskID)
				return
			}

			t.cancelOnce.Do(func() {
				if t.ErrChan != nil {
					close(t.ErrChan)
				}
				if t.LogChan != nil {
					close(t.LogChan)
				}
				if t.ProgressChan != nil {
					close(t.ProgressChan)
				}
				t.Cancel()
				//if t.timer != nil {
				//	t.timer.Stop()
				//}
			})

			// after cancel, delete info
			if delete {
				TaskManager.Delete(taskID)
			}
			klog.Infof("Task %s has been cancelled", taskID)
		}
	} else {
		klog.Infof("Task %s not found", taskID)
	}
}

func CompleteTask(taskID string) {
	klog.Infof("~~~Debug Log: Try to complete Task %s", taskID)
	time.Sleep(1 * time.Second)
	if task, ok := TaskManager.Load(taskID); ok {
		if t, ok := task.(*Task); ok {
			if t.Status == "cancelled" {
				klog.Infof("Task %s has already been cancelled, cannot complete it", taskID)
				return
			} else if t.Status == "completed" {
				klog.Infof("Task %s has already been completed, cannot complete it again", taskID)
				return
			} else if t.Status == "failed" {
				klog.Infof("Task %s has failed, cannot complete it", taskID)
				return
			}

			t.cancelOnce.Do(func() {
				t.Mu.Lock()
				// 确保字段可导出
				t.Progress = 100
				t.Status = "completed"
				TaskManager.Store(taskID, t) // 存储指针副本
				klog.Infof("Task %s has been completed with Progress %d", taskID, t.Progress)

				// 锁外执行非关键操作
				if t.ErrChan != nil {
					close(t.ErrChan)
				}
				if t.LogChan != nil {
					close(t.LogChan)
				}
				if t.ProgressChan != nil {
					close(t.ProgressChan)
				}
				t.Cancel()
				//if t.timer != nil {
				//	t.timer.Stop()
				//}
				t.Mu.Unlock()
			})
			klog.Infof("Task %s has completed", taskID)
		}
	} else {
		klog.Infof("Task %s not found", taskID)
	}
}

func FailTask(taskID string) {
	klog.Infof("~~~Debug Log: Try to fail Task %s", taskID)
	time.Sleep(1 * time.Second)
	if task, ok := TaskManager.Load(taskID); ok {
		if t, ok := task.(*Task); ok {
			if t.Status == "cancelled" {
				klog.Infof("Task %s has already been cancelled", taskID)
				return
			} else if t.Status == "completed" {
				klog.Infof("Task %s has already been completed", taskID)
				return
			} else if t.Status == "failed" {
				klog.Infof("Task %s has failed", taskID)
				return
			}

			t.cancelOnce.Do(func() {
				t.Mu.Lock()
				// 确保字段可导出
				t.Status = "failed"
				TaskManager.Store(taskID, t) // 存储指针副本
				klog.Infof("Task %s has been completed with Progress %d", taskID, t.Progress)

				// 锁外执行非关键操作
				if t.ErrChan != nil {
					close(t.ErrChan)
				}
				if t.LogChan != nil {
					close(t.LogChan)
				}
				if t.ProgressChan != nil {
					close(t.ProgressChan)
				}
				t.Cancel()
				//if t.timer != nil {
				//	t.timer.Stop()
				//}
				t.Mu.Unlock()
			})
			klog.Infof("Task %s has completed", taskID)
		}
	} else {
		klog.Infof("Task %s not found", taskID)
	}
}
