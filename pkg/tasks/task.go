package tasks

import (
	"context"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/models"
	"fmt"
	"time"

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

	executed bool
	finished bool
	suspend  bool

	ctx       context.Context
	ctxCancel context.CancelFunc

	createAt time.Time
	execAt   time.Time
	endAt    time.Time

	funcs   []func() error
	manager *taskManager
}

func (t *Task) Id() string {
	return t.id
}

func (t *Task) SetTotalSize(size int64) {
	t.totalSize = size
}

func (t *Task) Cancel() {
	t.ctxCancel()

	if !t.executed {
		return
	}

	for {
		if !t.finished {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}

	cmd := rclone.Command

	klog.Infof("[Task] Id: %s, Canceled, delete param: %s", t.id, common.ToJson(t.param.Delete))
	if !t.suspend && t.param.Delete != nil {
		if e := cmd.Clear(t.param.Delete); e != nil {
			klog.Errorf("[Task] Id: %s, clear sync dst error: %v", t.id, e)
		}
	}
}

func (t *Task) Execute(fs ...func() error) error {
	userPool := t.manager.getOrCreateUserPool(t.param.Owner)
	_, loaded := userPool.tasks.LoadOrStore(t.id, t)
	_ = loaded

	if t.funcs == nil {
		t.funcs = append(t.funcs, fs...)
	}

	_, ok := userPool.pool.TrySubmit(func() { // ~ enter
		var err error

		defer func() {
			klog.Infof("[Task] Id: %s done! status: %s, progress: %d, size: %d, transfer: %d, elapse: %d, error: %v",
				t.id, t.state, t.progress, t.totalSize, t.transfer, time.Since(t.execAt), err)

			t.endAt = time.Now()
			t.finished = true
		}()

		if t.state == common.Canceled || t.state == common.Paused {
			return
		}

		t.totalPhases = len(fs)
		t.execAt = time.Now()
		t.state = common.Running
		t.executed = true

		for phase, f := range t.funcs {
			t.currentPhase = phase + 1
			// If f() is not the final stage, such as downloadFromCloud, and uploadToSync will be executed afterwards, the src and dst need to be reset. After entering the next phase, src and dst will be extracted again.
			klog.Infof("[Task] Id: %s, exec phase: %d/%d", t.id, t.currentPhase, t.totalPhases)
			err = f()

			if err != nil {
				klog.Errorf("[Task] Id: %s, exec error: %v", t.id, err)
				if err.Error() == TaskCancel {
					if t.suspend {
						t.state = common.Paused
						return
					}

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

func (t *Task) isCancel() (bool, error) {
	select {
	case <-t.ctx.Done():
		return true, t.ctx.Err()
	default:
	}
	return false, nil
}
