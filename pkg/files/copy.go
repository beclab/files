package files

import (
	"files/pkg/common"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
	"os"
	"path"
	"sync"
)

// Copy copies a file or folder from one place to another.
func Copy(fs afero.Fs, task *common.Task, src, dst string) error {
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		err = ExecuteRsync(task, "", "", 0, 99)
		if err != nil {
			klog.Errorf("failed to execute rsync: %v\n", err)
			return
		}
		common.CompleteTask(task.ID)
	}()

	return nil
}
