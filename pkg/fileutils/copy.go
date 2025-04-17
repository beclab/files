package fileutils

import (
	"files/pkg/pool"
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"path"
	"time"

	"github.com/spf13/afero"
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
		// Prohibit copying from or to the virtual root directory.
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

	//if info.IsDir() {
	//	return CopyDir(fs, src, dst)
	//}
	//
	//return CopyFile(fs, src, dst)
	progressChan, err := ExecuteRsync(task.Source, task.Dest)
	if err != nil {
		fmt.Printf("Failed to execute rsync: %v\n", err)
		return err
	}

	go task.UpdateProgressFromRsync(progressChan)

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
