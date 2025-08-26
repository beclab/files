package tasks

import (
	"context"
	"files/pkg/common"
	"files/pkg/models"
	"fmt"

	"k8s.io/klog/v2"
)

type TaskInfo struct {
	Id            string `json:"id"`
	Action        string `json:"action"`
	IsDir         bool   `json:"is_dir"`
	FileName      string `json:"filename"`
	Dst           string `json:"dest"`
	DstPath       string `json:"dst_filename"`
	DstFileType   string `json:"dst_type"`
	Src           string `json:"source"`
	SrcFileType   string `json:"src_type"`
	CurrentPhase  int    `json:"current_phase"`
	TotalPhases   int    `json:"total_phases"`
	Progress      int    `json:"progress"`
	Transferred   int64  `json:"transferred"`
	TotalFileSize int64  `json:"total_file_size"`
	TidyDirs      bool   `json:"tidy_dirs"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"failed_reason"`
}

type Task struct {
	id      string
	param   *models.PasteParam
	state   string
	message string

	currentPhase int // current phase
	totalPhases  int // number of execution phases
	progress     int
	transfer     int64
	totalSize    int64
	tidyDirs     bool
	isFile       bool

	ctx      context.Context
	cancel   context.CancelFunc
	canceled bool

	manager *taskManager
}

func (t *Task) Id() string {
	return t.id
}

func (t *Task) Execute(fs ...func() error) error {
	_, loaded := t.manager.task.LoadOrStore(t.id, t)
	if loaded {
		return fmt.Errorf("task %s exists in taskManager", t.id)
	}

	_, ok := t.manager.pool.TrySubmit(func() {
		if t.canceled {
			t.state = common.Canceled
			return
		}

		var err error
		_ = err

		defer func() {
			klog.Infof("[Task] Id: %s done! status: %s, progress: %d, size: %d, transfer: %d", t.id, t.state, t.progress, t.totalSize, t.transfer)
		}()

		t.totalPhases = len(fs)

		t.state = common.Running

		for phase, f := range fs { //
			t.currentPhase = phase + 1
			// If f() is not the final stage, such as downloadFromCloud, and uploadToSync will be executed afterwards, the src and dst need to be reset. After entering the next phase, src and dst will be extracted again.
			klog.Infof("[Task] Id: %s, exec phase: %d/%d", t.id, t.currentPhase, t.totalPhases)
			err := f()
			if err != nil {
				klog.Errorf("[Task] Id: %s, exec error: %v", t.id, err)
				if err.Error() == "context canceled" {
					t.state = common.Canceled
				} else {
					t.state = common.Failed
				}
				t.message = err.Error()
				return
			}
		}

		t.state = common.Completed
		t.progress = 100

		return
	})

	if !ok {
		return fmt.Errorf("submit worker failed")
	}

	return nil
}

func (t *Task) updateProgress(progress int, transfer int64) {
	t.progress = progress
	t.transfer = transfer
}

func (t *Task) updateTotalSize(totalSize int64) {
	t.totalSize = totalSize
}

func (t *Task) GetProgress() (string, int, int64, int64) {
	return t.state, t.progress, t.transfer, t.totalSize
}

func (t *Task) isLastPhase() bool {
	return t.currentPhase == t.totalPhases
}

func (t *Task) isCanceled() (bool, error) {
	select {
	case <-t.ctx.Done():
		return true, t.ctx.Err()
	default:
	}
	return false, nil
}
