package pool

import (
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
}

func NewTask(id, source, dest string) *Task {
	return &Task{
		ID:     id,
		Source: source,
		Dest:   dest,
		Status: "pending",
	}
}

func ProcessProgress(progress, progressType int) int {
	// TODO: define progressType
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

	for progress := range progressChan {
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

	t.mu.Lock()
	t.Status = "completed"
	TaskManager.Store(t.ID, t)
	t.mu.Unlock()
}

//func (t *Task) UpdateProgressFromRsync(progressChan chan int) {
//	klog.Infof("~~~Temp log: Update Progress From Rsync [%s] ~~~", t.ID)
//	t.mu.Lock()
//	defer t.mu.Unlock()
//	t.Status = "running"
//	t.Progress = 0
//	t.Log = []string{}
//	klog.Infof("~~~Temp log: %v", t)
//
//	for progress := range progressChan {
//		klog.Infof("[%s] %d", t.ID, progress)
//		// 对 rsync 的进度进行处理，例如求平方根再乘以 10
//		processedProgress := 0
//		//processedProgress := int(float64(progress)*0.1*float64(progress)*0.1*1000 / 10) // 示例处理逻辑
//		// 更简单的逻辑可以是：processedProgress = int(math.Sqrt(float64(progress)) * 10)
//		// 这里使用一个近似的简单整数运算模拟
//		if progress > 0 {
//			processedProgress = ProcessProgress(progress, 0) //int(float64(progress)**0.5 * 10) // 使用平方根乘以10的简化逻辑
//		} else {
//			processedProgress = 0
//		}
//		t.mu.Lock() // 重新锁定以更新进度
//		t.Progress = processedProgress
//		t.mu.Unlock()
//	}
//
//	t.mu.Lock()
//	t.Status = "completed"
//	t.mu.Unlock()
//}

func (t *Task) Logging(entry string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Log = append(t.Log, entry)
}

func (t *Task) LoggingError(entry string) {
	t.Logging("[ERROR] " + entry)
}

func (t *Task) GetProgress() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Progress
}
