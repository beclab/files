package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/alitto/pond/v2"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	SrcType        string             `json:"src_type"` // detailed, only for task_info
	DstType        string             `json:"dst_type"` // detailed, only for task_info
	Action         string             `json:"action"`
	Cancellable    bool               `json:"cancellable"`
	IsDir          bool               `json:"is_dir"`
	FileType       string             `json:"file_type"`
	Filename       string             `json:"filename"`
	DstFilename    string             `json:"dst_filename"`
	Status         string             `json:"status"`
	Progress       int                `json:"progress"`
	TotalFileSize  int64              `json:"total_file_size"`
	Transferred    int64              `json:"transferred"`
	Log            []string           `json:"log"`
	FailedReason   string             `json:"failed_reason"`
	Buffers        []string           `json:"buffers"`
	IsBufferDir    bool               `json:"is_buffer_dir"`
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
	showLog bool
}

func (ft FormattedTask) MarshalJSON() ([]byte, error) {
	type Alias FormattedTask
	if ft.showLog {
		return json.Marshal(struct {
			Alias
			Log interface{} `json:"log,omitempty"`
		}{
			Alias: Alias(ft),
			Log:   ft.Task.Log,
		})
	} else {
		return json.Marshal(struct {
			Alias
		}{
			Alias: Alias(ft),
		})
	}
}

func (ft FormattedTask) WithLogControl(showLog bool) FormattedTask {
	ft.showLog = showLog
	return ft
}

func (ft FormattedTask) String() string {
	return fmt.Sprintf("ID: %s, Source: %s, Dest: %s, SrcType: %s, DstType: %s, Action: %s, Cancellable: %v, IsDir: %v, FileType: %s, Filename: %s, DstFilename: %s, Status: %s, Progress: %d, TotalFileSize: %d, Transferred: %d, Log: %v, FailedReason: %s, RelationTaskID: %s, RelationNode: %s",
		ft.ID, ft.Source, ft.Dest, ft.SrcType, ft.DstType, ft.Action, ft.Cancellable, ft.IsDir, ft.FileType, ft.Filename, ft.DstFilename, ft.Status, ft.Progress, ft.TotalFileSize, ft.Transferred, ft.Log, ft.FailedReason, ft.RelationTaskID, ft.RelationNode)
}

func NewTask(id, source, dest, srcType, dstType, action string, cancellable, isBufferDir bool, isDir bool, fileType, filename string) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	task := &Task{
		ID:             id,
		Source:         source,
		Dest:           dest,
		SrcType:        srcType,
		DstType:        dstType,
		Action:         action,
		Cancellable:    cancellable,
		IsDir:          isDir,
		FileType:       fileType,
		Filename:       filename,
		DstFilename:    path.Base(strings.TrimSuffix(dest, "/")),
		Status:         "pending",
		Progress:       0,
		TotalFileSize:  0,
		Transferred:    0,
		Log:            make([]string, 0),
		FailedReason:   "",
		Buffers:        make([]string, 0),
		IsBufferDir:    isBufferDir,
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

	timeout := time.After(6 * time.Hour)
	//lastHeartbeat := time.Now()
	//heartbeatTicker := time.NewTicker(30 * time.Second)
	//defer heartbeatTicker.Stop()

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

		//case <-heartbeatTicker.C:
		//	if time.Since(lastHeartbeat) > 45*time.Second {
		//		klog.Errorf("Task %s heartbeat lost", t.ID)
		//		return
		//	}

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
			t.FailedReason = err.Error()
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

			//if t.Progress == 100 && processedProgress == 100 {
			if processedProgress == 100 {
				select {
				case <-time.After(1 * time.Second):
					CompleteTask(t.ID)
				case <-t.LogChan:
					// if not err, JUST wait 2 seconds. Because log may flush.
					// CompleteTask(t.ID)
				case <-t.ErrChan:
					FailTask(t.ID)
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
	klog.Infof("TASK LOGGING [%s] %s", t.ID, entry)
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.Log = append(t.Log, entry)
}

// LoggingError 添加错误日志条目
func (t *Task) LoggingError(entry string) {
	klog.Infof("TASK LOGGING ERROR [%s] %s", t.ID, entry)
	//t.Logging("[ERROR] " + entry)
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.Log = append(t.Log, "[ERROR] "+entry)
	t.FailedReason = entry
}

func (t *Task) AddBuffer(entry string) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.Buffers = append(t.Buffers, entry)
}

func (t *Task) RemoveBuffer(entry string) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	for i, buf := range t.Buffers {
		if buf == entry {
			t.Buffers = append(t.Buffers[:i], t.Buffers[i+1:]...)
			break
		}
	}
}

func (t *Task) DeleteBuffers() {
	if len(t.Buffers) > 0 {
		t.Logging("Final clean buffers")
	}
	var err error
	for _, entry := range t.Buffers {
		if t.IsBufferDir {
			dir := filepath.Dir(entry)
			err = os.RemoveAll(dir)
			if err != nil {
				klog.Errorln("failed to delete buffer file dir: ", err)
				t.Logging(fmt.Sprintf("Failed to delete buffer file dir: %v", err))
				return
			}
		} else {
			err = os.RemoveAll(entry)
			if err != nil {
				klog.Errorln("Failed to delete buffer file: ", err)
				t.Logging(fmt.Sprintf("Failed to delete buffer file: %v", err))
				return
			}
		}
		t.Logging(fmt.Sprintf("Buffer file %s deleted.", entry))
	}
	if len(t.Buffers) > 0 {
		t.Logging("Removed all buffers")
		t.Mu.Lock()
		t.Buffers = make([]string, 0)
		t.Mu.Unlock()
	}
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
			if !delete {
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
			}

			if t.Status != "cancelled" {
				t.DeleteBuffers()
				t.Mu.Lock()
				t.Status = "cancelled"
				TaskManager.Store(taskID, t)

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
					if delete && t.timer != nil {
						t.timer.Stop()
					}
				})
				t.Mu.Unlock()
			}

			// after cancel, delete info
			if delete {
				TaskManager.Delete(taskID)
				klog.Infof("Task %s has been deleted", taskID)
			} else {
				klog.Infof("Task %s has been cancelled", taskID)
			}
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
				t.DeleteBuffers()
				t.Mu.Lock()
				// 确保字段可导出
				t.Progress = 100
				t.Status = "completed"
				t.FailedReason = ""
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
			//if t.Status == "cancelled" {
			//	klog.Infof("Task %s has already been cancelled", taskID)
			//	return
			//} else if t.Status == "completed" {
			//	klog.Infof("Task %s has already been completed", taskID)
			//	return
			//} else
			if t.Status == "failed" {
				klog.Infof("Task %s has failed", taskID)
				return
			} // at any condition, fail can be fail

			t.DeleteBuffers()
			t.cancelOnce.Do(func() {
				t.Mu.Lock()
				// 确保字段可导出
				t.Status = "failed"
				TaskManager.Store(taskID, t) // 存储指针副本
				klog.Infof("Task %s has failed with Progress %d", taskID, t.Progress)

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
			klog.Infof("Task %s has failed", taskID)
		}
	} else {
		klog.Infof("Task %s not found", taskID)
	}
}
