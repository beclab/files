package fileutils

import (
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
func Copy(fs afero.Fs, task *pool.Task, src, dst string) error {
	if src = path.Clean("/" + src); src == "" {
		return os.ErrNotExist
	}

	if dst = path.Clean("/" + dst); dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		return os.ErrInvalid
	}

	if dst == src {
		return os.ErrInvalid
	}

	info, err := fs.Stat(src)
	if err != nil {
		return err
	}
	klog.Infof("copy %v from %s to %s", info, src, dst)

	var progressChan chan int // 假设 progressChan 是 int 类型的通道
	var errChan chan error    // 用于接收 ExecuteRsyncSimulated 的错误

	// 启动一个 goroutine 来执行 ExecuteRsyncSimulated
	go func() {
		var err error
		progressChan, err = ExecuteRsyncSimulated("/data"+task.Source, "/data"+task.Dest)
		if err != nil {
			errChan <- err
		}
	}()

	// 等待 ExecuteRsyncSimulated 完成或出错
	select {
	case err := <-errChan:
		if err != nil {
			fmt.Printf("Failed to execute rsync: %v\n", err)
			return err
		}
	case <-time.After(5 * time.Second): // 假设等待 5 秒以避免无限等待
		fmt.Println("ExecuteRsyncSimulated took too long to start, proceeding assuming no initial error.")
		// 在实际应用中，你可能需要更复杂的逻辑来处理这种情况
	}

	if progressChan == nil {
		return fmt.Errorf("progressChan is nil")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		klog.Infof("~~~Temp log: copy %v from %s to %s, will update progress", info, src, dst)
		task.UpdateProgressFromRsync(progressChan)
	}()

	// 等待 goroutine 完成
	wg.Wait()

	// 模拟外部获取进度
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)
	go func() {
		time.Sleep(10 * time.Second) // 模拟一些其他操作
		done <- true
	}()

	for {
		select {
		case <-ticker.C:
			if storedTask, ok := pool.TaskManager.Load(task.ID); ok {
				if t, ok := storedTask.(*pool.Task); ok {
					fmt.Printf("Task %s Progress: %d%%\n", t.ID, t.GetProgress())
				}
			}
		case <-done:
			fmt.Println("Operation completed or stopped.")
			return nil
		}
	}
}
