package tasks

import (
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/models"
	"fmt"
	"strings"
	"sync"
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
	Src           string `json:"source"`
	CurrentPhase  int    `json:"current_phase"`
	TotalPhases   int    `json:"total_phases"`
	Progress      int    `json:"progress"`
	Transferred   int64  `json:"transferred"`
	TotalFileSize int64  `json:"total_file_size"`
	TidyDirs      bool   `json:"tidy_dirs"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"failed_reason"`
	PauseAble     bool   `json:"pause_able"`
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

	running bool
	suspend bool

	wasPaused bool

	pausedParam     *models.FileParam // used for dst
	pausedPhase     int
	pausedSyncMkdir bool

	ctx       context.Context
	ctxCancel context.CancelFunc

	createAt time.Time
	execAt   time.Time
	endAt    time.Time

	funcs   []func() error
	manager *taskManager

	details []string
	isShare bool

	// done is closed by the worker goroutine in Execute right before
	// it returns, no matter how it returns (success / failure /
	// cancellation / panic via runtime). Cancel waits on it instead
	// of busy-polling t.running, so we can never lock up forever if
	// `running` ends up out of sync with the actual goroutine state.
	// The channel is reset on every Resume, so a cancelled-then-
	// resumed task gets a fresh "done" signal.
	doneMu sync.Mutex
	done   chan struct{}
}

func (t *Task) Id() string {
	return t.id
}

func (t *Task) SetTotalSize(size int64) {
	t.totalSize = size
}

// ensureDone returns the current done channel, creating it lazily.
// It is safe to call from any goroutine.
func (t *Task) ensureDone() chan struct{} {
	t.doneMu.Lock()
	defer t.doneMu.Unlock()
	if t.done == nil {
		t.done = make(chan struct{})
	}
	return t.done
}

// resetDone replaces the done channel; called by Resume so the
// "wait" semantics for the *new* worker run are independent of the
// previous one.
func (t *Task) resetDone() {
	t.doneMu.Lock()
	defer t.doneMu.Unlock()
	t.done = make(chan struct{})
}

// closeDone signals all Cancel waiters that the worker has exited.
// Idempotent: safe to call even if already closed.
func (t *Task) closeDone() {
	t.doneMu.Lock()
	defer t.doneMu.Unlock()
	if t.done == nil {
		t.done = make(chan struct{})
	}
	select {
	case <-t.done:
		// already closed
	default:
		close(t.done)
	}
}

// ~ Cancel
func (t *Task) Cancel() {
	done := t.ensureDone()
	t.ctxCancel()

	// If no worker is currently running, no need to wait - skip the
	// channel select entirely. This also covers tasks that never
	// got to Execute() at all (Cancel right after Create), which
	// would otherwise sit in <-done for 30s before the timeout.
	if !t.running {
		// fall through to state update
	} else {
		// Replace the previous busy-poll on t.running (which could
		// spin forever if `running` ended up out of sync with the
		// actual worker state). Cap the wait at 30s so a stuck
		// worker can't pin a Cancel goroutine indefinitely either.
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			klog.Warningf("[Task] Id: %s, Cancel timed out waiting for worker to exit", t.id)
		}
	}

	t.doneMu.Lock()
	if !t.suspend {
		t.state = common.Canceled
	} else {
		t.state = common.Paused
	}
	finalState := t.state
	pausedParam := t.pausedParam
	suspend := t.suspend
	wasPaused := t.wasPaused
	currentPhase := t.currentPhase
	totalPhases := t.totalPhases
	t.doneMu.Unlock()

	klog.Infof("[Task] Id: %s, Cancel Final, state: %s, suspend: %v, wasPaused: %v, phase: %d/%d, pause: %s, temp: %s",
		t.id, finalState, suspend, wasPaused, currentPhase, totalPhases, common.ToJson(pausedParam), common.ToJson(t.param.Temp))

	if finalState == common.Canceled {
		klog.Infof("[Task] Id: %s, Cancel Final, pause result: %s, temp result: %s", t.id, common.ToJson(pausedParam), common.ToJson(t.param.Temp))

		if pausedParam != nil {
			if pausedParam.FileType != common.Sync {
				if e := rclone.Command.Clear(pausedParam); e != nil {
					klog.Errorf("[Task] Id: %s, Cancel Final, delete pause result error: %v", t.id, e)
				}
			} else {
				if e := seahub.HandleDelete(pausedParam); e != nil {
					klog.Errorf("[Task] Id: %s, Cancel Final, delete seahub pause result error: %v", t.id, e)
				}
			}

		}

		if t.param.Temp != nil {
			if e := rclone.Command.Clear(t.param.Temp); e != nil {
				klog.Errorf("[Task] Id: %s, Cancel Final, delete temp result error: %v", t.id, e)
			}
		}

		klog.Infof("[Task] Id: %s, Canel Final, clear result done!", t.id)
	}
}

// ~ Execute
func (t *Task) Execute(fs ...func() error) error {
	userPool := t.manager.getOrCreateUserPool(t.param.Owner)
	_, loaded := userPool.tasks.LoadOrStore(t.id, t)
	_ = loaded

	if t.funcs == nil {
		t.funcs = append(t.funcs, fs...)
	}

	// Make sure each Execute invocation has its own done channel so
	// any pending Cancel() on the *previous* run doesn't observe an
	// already-closed channel and skip the wait. Resume callers also
	// reset this; doing it here as well keeps the contract simple.
	t.resetDone()

	_, ok := userPool.pool.TrySubmit(func() { // ~ enter
		var err error

		defer func() {

			t.endAt = time.Now()
			t.running = false
			state := t.state
			progress := t.progress
			transfer := t.transfer
			totalSize := t.totalSize
			elapsed := time.Since(t.execAt)
			// Wake any goroutine blocked in Cancel().

			klog.Infof("[Task] Id: %s defer! status: %s, progress: %d, size: %d, transfer: %d, elapse: %d, error: %v",
				t.id, state, progress, totalSize, transfer, elapsed, err)

			t.closeDone()

		}()

		if common.ListContains([]string{common.Canceled, common.Paused, common.Failed, common.Running, common.Completed}, t.getState()) {
			return
		}

		t.totalPhases = len(fs)
		t.execAt = time.Now()
		t.state = common.Running
		t.running = true
		t.details = nil

		for phase, f := range t.funcs {
			t.currentPhase = phase + 1
			currentPhase := t.currentPhase
			totalPhases := t.totalPhases

			klog.Infof("[Task] Id: %s, exec phase: %d/%d", t.id, currentPhase, totalPhases)
			err = f()

			if err != nil {
				klog.Errorf("[Task] Id: %s, exec failed, suspend: %v, error: %s", t.id, t.suspend, err.Error())

				var errmsg = common.RemoveBlank(err.Error())
				t.details = append(t.details, errmsg)

				if errmsg == TaskCancel {
					if t.suspend {
						t.state = common.Paused
						return
					}

					t.state = common.Canceled
				} else {
					t.message = errmsg
					t.state = common.Failed

					klog.Errorf("[Task] Id: %s, exec failed, temp result: %s, pause result: %s", t.id, common.ToJson(t.param.Temp), common.ToJson(t.pausedParam))

					if t.param.Temp != nil {
						if e := rclone.Command.Clear(t.param.Temp); e != nil {
							klog.Errorf("[Task] Id: %s, exec failed, delete temp result error: %v", t.id, e)
						}

						if strings.Contains(t.param.Temp.Path, t.id) {
							if e := rclone.Command.ClearTaskCaches(t.param.Temp, t.id); e != nil {
								klog.Errorf("[Task] Id: %s, exec failed, delete task cached result error: %v", t.id, e)
							}
						}
					}
					if t.pausedParam != nil {
						if e := rclone.Command.Clear(t.pausedParam); e != nil {
							klog.Errorf("[Task] Id: %s, exec failed, delete pause result error: %v", t.id, e)
						}

						if strings.Contains(t.pausedParam.Path, t.id) {
							if e := rclone.Command.ClearTaskCaches(t.pausedParam, t.id); e != nil {
								klog.Errorf("[Task] Id: %s, exec failed, delete task cached result error: %v", t.id, e)
							}
						}
					}

					klog.Infof("[Task] Id: %s, exec failed, clear result done!", t.id)
				}
				return
			}
		}

		t.state = common.Completed
		t.progress = 100
		t.details = append(t.details, "successed")

		// if t.param.Action == common.ActionMove && !t.isShare {
		// 	share.UpdateMovedSharePaths(t.param.Owner, t.param.Src, t.param.Dst)
		// }

		return
	})

	if !ok {
		// The worker won't run, so the deferred closeDone() above
		// will not fire either. Close it now so any concurrent
		// Cancel doesn't sit waiting for a worker that never
		// started.
		t.closeDone()
		return fmt.Errorf("submit worker failed")
	}

	return nil
}

func (t *Task) updateProgress(progress int, transfer int64) {
	t.progress = progress
	t.transfer += transfer
	t.details = append(t.details, fmt.Sprintf("rsync files progress: %d, transfer: %d", progress, transfer))
}

func (t *Task) updateProgressRsync(progress int, transfer int64) {
	t.progress = progress
	t.transfer = transfer
	t.details = append(t.details, fmt.Sprintf("rsync files progress: %d, transfer: %d", progress, transfer))
}

func (t *Task) resetProgressZero() {
	t.progress = 0
	t.transfer = 0
	t.details = append(t.details, fmt.Sprintf("rsync files progress: 0, transfer: 0"))
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

func (t *Task) formatJobStatusError(s string) error {
	var result string
	if strings.Contains(s, "is a directory") {
		var pos = strings.Index(s, t.id)
		result = s[pos+len(t.id):]
		result = strings.TrimSuffix(result, ": is a directory")
		return errors.New(fmt.Sprintf("There may be folders and files with the same name, so the data cannot be copied to the disk. Please check and rename the corresponding folders or files: %s", result))
	}

	if strings.Contains(s, "path/insufficient_space") {
		return errors.New("Storage space is full.")
	}

	return errors.New(s)
}
