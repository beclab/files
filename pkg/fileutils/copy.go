package fileutils

import (
	"files/pkg/pool"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
	"os"
	"path"
	"sync"
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

	if task == nil {
		if info.IsDir() {
			return CopyDir(fs, src, dst)
		}

		return CopyFile(fs, src, dst)
	}

	// 启动一个 goroutine 来执行 ExecuteRsyncSimulated
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		//progressChan, logChan, errChan, err = ExecuteRsyncWithContext(task.Ctx, "/data"+task.Source, "/data"+task.Dest)
		err = ExecuteRsync(task, "", "", 0, 99)
		if err != nil {
			klog.Errorf("failed to execute rsync: %v\n", err)
			return
		}
		pool.CompleteTask(task.ID)
	}()

	return nil
}
