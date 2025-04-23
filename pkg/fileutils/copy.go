package fileutils

import (
	"context"
	"files/pkg/pool"
	"fmt"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
	"os"
	"path"
	"sync"
	"time"
)

// Copy copies a file or folder from one place to another.
//func Copy(fs afero.Fs, task *pool.Task, src, dst string) error {
//	if src = path.Clean("/" + src); src == "" {
//		return os.ErrNotExist
//	}
//
//	if dst = path.Clean("/" + dst); dst == "" {
//		return os.ErrNotExist
//	}
//
//	if src == "/" || dst == "/" {
//		return os.ErrInvalid
//	}
//
//	if dst == src {
//		return os.ErrInvalid
//	}
//
//	info, err := fs.Stat(src)
//	if err != nil {
//		return err
//	}
//	klog.Infof("copy %v from %s to %s", info, src, dst)
//
//	var progressChan chan int // 假设 progressChan 是 int 类型的通道
//	var errChan chan error    // 用于接收 ExecuteRsyncSimulated 的错误
//
//	// 启动一个 goroutine 来执行 ExecuteRsyncSimulated
//	go func() {
//		var err error
//		progressChan, err = ExecuteRsync("/data"+task.Source, "/data"+task.Dest)
//		if err != nil {
//			errChan <- err
//		}
//	}()
//
//	// 等待 ExecuteRsyncSimulated 完成或出错
//	select {
//	case err := <-errChan:
//		if err != nil {
//			fmt.Printf("Failed to execute rsync: %v\n", err)
//			return err
//		}
//	case <-time.After(5 * time.Second): // 假设等待 5 秒以避免无限等待
//		fmt.Println("ExecuteRsyncSimulated took too long to start, proceeding assuming no initial error.")
//		// 在实际应用中，你可能需要更复杂的逻辑来处理这种情况
//	}
//
//	if progressChan == nil {
//		return fmt.Errorf("progressChan is nil")
//	}
//
//	var wg sync.WaitGroup
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		klog.Infof("~~~Temp log: copy %v from %s to %s, will update progress", info, src, dst)
//		task.UpdateProgressFromRsync(progressChan)
//	}()
//
//	// 等待 goroutine 完成
//	//wg.Wait()
//
//	// 模拟外部获取进度
//	ticker := time.NewTicker(1 * time.Second)
//	defer ticker.Stop()
//
//	done := make(chan bool)
//	go func() {
//		time.Sleep(100 * time.Second) // 模拟一些其他操作
//		done <- true
//	}()
//
//	for {
//		select {
//		case <-ticker.C:
//			if storedTask, ok := pool.TaskManager.Load(task.ID); ok {
//				if t, ok := storedTask.(*pool.Task); ok {
//					klog.Infof("Task %s Infos: %v\n", t.ID, t)
//					fmt.Printf("Task %s Progress: %d%%\n", t.ID, t.GetProgress())
//				}
//			}
//		case <-done:
//			fmt.Println("Operation completed or stopped.")
//			return nil
//		}
//	}
//}

func Copy(ctx context.Context, fs afero.Fs, task *pool.Task, src, dst string) error {
	// 清理路径
	src = path.Clean("/" + src)
	dst = path.Clean("/" + dst)

	// 检查路径合法性
	if src == "" || dst == "" {
		return os.ErrNotExist
	}
	if src == "/" || dst == "/" {
		return os.ErrInvalid
	}
	if src == dst {
		return os.ErrInvalid
	}

	// 检查源文件是否存在
	info, err := fs.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}
	klog.Infof("Copying %v from %s to %s", info, src, dst)

	// 初始化通道
	progressChan := make(chan int, 100)
	errChan := make(chan error, 1)

	// 启动一个 goroutine 来执行 rsync 操作
	go func() {
		progressChan, errChan, err = ExecuteRsyncWithContext(ctx, "/data"+src, "/data"+dst)
		if err != nil {
			errChan <- err
		}
	}()

	// 等待 rsync 操作完成或出错
	select {
	case err = <-errChan:
		if err != nil {
			klog.Errorf("Failed to execute rsync: %v", err)
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	// 检查 progressChan 是否初始化成功
	if progressChan == nil {
		return fmt.Errorf("progressChan is nil")
	}

	// 使用 goroutine 更新任务进度
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		klog.Infof("Starting to update progress for task %s", task.ID)
		task.UpdateProgressFromRsync(progressChan)
	}()

	// 模拟外部获取进度（可选，仅用于演示）
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)
	go func() {
		time.Sleep(100 * time.Second) // 模拟长时间操作
		done <- true
	}()

	for {
		select {
		case <-ticker.C:
			// 打印任务进度（模拟外部获取）
			if storedTask, ok := pool.TaskManager.Load(task.ID); ok {
				if t, ok := storedTask.(*pool.Task); ok {
					klog.Infof("Task %s Infos: %v\n", t.ID, t)
					fmt.Printf("Task %s Progress: %d%%\n", t.ID, t.GetProgress())
				}
			}
		case <-done:
			fmt.Println("Operation completed or stopped.")
			return nil
		}
	}

	// 等待进度更新 goroutine 完成
	wg.Wait()
	return nil
}
