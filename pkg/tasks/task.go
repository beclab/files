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
	// Immutable after creation - safe to read without the lock.
	id       string
	param    *models.PasteParam
	isFile   bool
	isShare  bool
	createAt time.Time
	manager  *taskManager

	// ctx/ctxCancel are reset in ResumeTask; that swap is also done
	// under mu, but the worker only ever reads the latest pointer
	// once at the start of a phase.
	ctx       context.Context
	ctxCancel context.CancelFunc

	// funcs is set during Execute under mu and never mutated again.
	funcs []func() error

	// mu guards every mutable field below. Without this, the worker
	// goroutine in Execute and the HTTP handler goroutines (GetTask,
	// GetTasksByStatus, PauseTask, CancelTask, ResumeTask) raced on
	// every status / progress field, with classic data-race symptoms
	// (-race fails, flaky API responses, partial reads).
	mu sync.RWMutex

	state   string
	message string

	currentPhase int // current phase
	totalPhases  int // number of execution phases
	progress     int
	transfer     int64
	totalSize    int64
	tidyDirs     bool

	running bool
	suspend bool

	wasPaused bool

	pausedParam     *models.FileParam // used for dst
	pausedPhase     int
	pausedSyncMkdir bool

	execAt time.Time
	endAt  time.Time

	details []string
}

func (t *Task) Id() string {
	return t.id
}

func (t *Task) SetTotalSize(size int64) {
	t.totalSize = size
}

// taskSnapshot is an immutable snapshot of the mutable Task fields
// taken under t.mu.RLock(). HTTP handlers should always project
// through it instead of reading t.* directly.
type taskSnapshot struct {
	State        string
	Message      string
	CurrentPhase int
	TotalPhases  int
	Progress     int
	Transfer     int64
	TotalSize    int64
	TidyDirs     bool
	Running      bool
	Suspend      bool
	WasPaused    bool
}

func (t *Task) snapshot() taskSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return taskSnapshot{
		State:        t.state,
		Message:      t.message,
		CurrentPhase: t.currentPhase,
		TotalPhases:  t.totalPhases,
		Progress:     t.progress,
		Transfer:     t.transfer,
		TotalSize:    t.totalSize,
		TidyDirs:     t.tidyDirs,
		Running:      t.running,
		Suspend:      t.suspend,
		WasPaused:    t.wasPaused,
	}
}

func (t *Task) getState() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// appendDetail appends one line to t.details under the lock.
//
// Phase functions (rsync / move / checkJobStats / ...) call this
// instead of `t.details = append(t.details, ...)` directly so
// concurrent snapshot() readers do not race the slice header.
//
// History note: an earlier version of this helper landed in
// PR #266 but was lost when PR #267's merge resolved task.go's
// conflict by taking the A.4b side, dropping the A.4a additions.
// task_test.go.TestTask_HelpersUseLock pins the existence of
// this function and setTidyDirs so a future merge that drops
// them again fails CI rather than silently regressing.
func (t *Task) appendDetail(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.details = append(t.details, line)
}

// setTidyDirs flips t.tidyDirs to v under the lock.
func (t *Task) setTidyDirs(v bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tidyDirs = v
}

// pausedSnapshot is a consistent view of the worker's pause /
// resume state. Use Task.pausedSnap() to acquire one rather than
// touching t.wasPaused / t.pausedParam / t.pausedPhase /
// t.pausedSyncMkdir directly: PauseTask writes those fields from
// the HTTP goroutine and the worker reads them across phases, so
// without a mutex on both sides the writes are not guaranteed to
// be visible per the Go memory model.
type pausedSnapshot struct {
	WasPaused bool
	Param     *models.FileParam
	Phase     int
	SyncMkdir bool
}

func (t *Task) pausedSnap() pausedSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return pausedSnapshot{
		WasPaused: t.wasPaused,
		Param:     t.pausedParam,
		Phase:     t.pausedPhase,
		SyncMkdir: t.pausedSyncMkdir,
	}
}

// markPaused records the pause checkpoint when a phase decides to
// stop progressing (typically after isCancel + suspend).
func (t *Task) markPaused(param *models.FileParam, phase int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pausedParam = param
	t.pausedPhase = phase
}

// setPausedSyncMkdir sets t.pausedSyncMkdir under the lock.
func (t *Task) setPausedSyncMkdir(v bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pausedSyncMkdir = v
}

// clearPausedParam sets t.pausedParam to nil under the lock.
func (t *Task) clearPausedParam() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pausedParam = nil
}

// takePausedParam atomically reads and clears t.pausedParam.
func (t *Task) takePausedParam() *models.FileParam {
	t.mu.Lock()
	defer t.mu.Unlock()
	p := t.pausedParam
	t.pausedParam = nil
	return p
}

// setPausedParam writes t.pausedParam under the lock.
func (t *Task) setPausedParam(p *models.FileParam) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pausedParam = p
}

// ~ Cancel
func (t *Task) Cancel() {
	t.ctxCancel()

	for {
		t.mu.RLock()
		stillRunning := t.running
		t.mu.RUnlock()
		if !stillRunning {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.mu.Lock()
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
	t.mu.Unlock()

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

	t.mu.Lock()
	if t.funcs == nil {
		t.funcs = append(t.funcs, fs...)
	}
	currentFuncs := t.funcs
	t.mu.Unlock()

	_, ok := userPool.pool.TrySubmit(func() { // ~ enter
		var err error

		defer func() {
			t.mu.Lock()
			t.endAt = time.Now()
			t.running = false
			state := t.state
			progress := t.progress
			transfer := t.transfer
			totalSize := t.totalSize
			elapsed := time.Since(t.execAt)
			t.mu.Unlock()

			klog.Infof("[Task] Id: %s defer! status: %s, progress: %d, size: %d, transfer: %d, elapse: %d, error: %v",
				t.id, state, progress, totalSize, transfer, elapsed, err)
		}()

		if common.ListContains([]string{common.Canceled, common.Paused, common.Failed, common.Running, common.Completed}, t.getState()) {
			return
		}

		t.mu.Lock()
		t.totalPhases = len(fs)
		t.execAt = time.Now()
		t.state = common.Running
		t.running = true
		t.details = nil
		t.mu.Unlock()

		for phase, f := range currentFuncs {
			t.mu.Lock()
			t.currentPhase = phase + 1
			currentPhase := t.currentPhase
			totalPhases := t.totalPhases
			t.mu.Unlock()

			klog.Infof("[Task] Id: %s, exec phase: %d/%d", t.id, currentPhase, totalPhases)
			err = f()

			if err != nil {
				suspend := func() bool { t.mu.RLock(); defer t.mu.RUnlock(); return t.suspend }()
				klog.Errorf("[Task] Id: %s, exec failed, suspend: %v, error: %s", t.id, suspend, err.Error())

				var errmsg = common.RemoveBlank(err.Error())

				if errmsg == TaskCancel {
					t.mu.Lock()
					t.details = append(t.details, errmsg)
					if t.suspend {
						t.state = common.Paused
						t.mu.Unlock()
						return
					}
					t.state = common.Canceled
					t.mu.Unlock()
				} else {
					t.mu.Lock()
					t.details = append(t.details, errmsg)
					t.message = errmsg
					t.state = common.Failed
					tempParam := t.param.Temp
					pausedParam := t.pausedParam
					t.mu.Unlock()

					klog.Errorf("[Task] Id: %s, exec failed, temp result: %s, pause result: %s", t.id, common.ToJson(tempParam), common.ToJson(pausedParam))

					if tempParam != nil {
						if e := rclone.Command.Clear(tempParam); e != nil {
							klog.Errorf("[Task] Id: %s, exec failed, delete temp result error: %v", t.id, e)
						}

						if strings.Contains(tempParam.Path, t.id) {
							if e := rclone.Command.ClearTaskCaches(tempParam, t.id); e != nil {
								klog.Errorf("[Task] Id: %s, exec failed, delete task cached result error: %v", t.id, e)
							}
						}
					}
					if pausedParam != nil {
						if e := rclone.Command.Clear(pausedParam); e != nil {
							klog.Errorf("[Task] Id: %s, exec failed, delete pause result error: %v", t.id, e)
						}

						if strings.Contains(pausedParam.Path, t.id) {
							if e := rclone.Command.ClearTaskCaches(pausedParam, t.id); e != nil {
								klog.Errorf("[Task] Id: %s, exec failed, delete task cached result error: %v", t.id, e)
							}
						}
					}

					klog.Infof("[Task] Id: %s, exec failed, clear result done!", t.id)
				}
				return
			}
		}

		t.mu.Lock()
		t.state = common.Completed
		t.progress = 100
		t.details = append(t.details, "successed")
		t.mu.Unlock()

		// if t.param.Action == common.ActionMove && !t.isShare {
		// 	share.UpdateMovedSharePaths(t.param.Owner, t.param.Src, t.param.Dst)
		// }

		return
	})

	if !ok {
		return fmt.Errorf("submit worker failed")
	}

	return nil
}

// ExecuteAsync runs the given phase functions in a bare goroutine,
// bypassing the per-user pond pool. This allows upload-finalize tasks
// to run concurrently with paste/copy tasks without pool contention.
func (t *Task) ExecuteAsync(fs ...func() error) {
	userPool := t.manager.getOrCreateUserPool(t.param.Owner)
	userPool.tasks.LoadOrStore(t.id, t)

	t.mu.Lock()
	if t.funcs == nil {
		t.funcs = append(t.funcs, fs...)
	}
	t.mu.Unlock()

	go func() {
		var err error

		defer func() {
			t.mu.Lock()
			t.endAt = time.Now()
			t.running = false
			state := t.state
			progress := t.progress
			transfer := t.transfer
			totalSize := t.totalSize
			elapsed := time.Since(t.execAt)
			t.mu.Unlock()

			klog.Infof("[Task] Id: %s defer! status: %s, progress: %d, size: %d, transfer: %d, elapse: %d, error: %v",
				t.id, state, progress, totalSize, transfer, elapsed, err)
		}()

		if common.ListContains([]string{common.Canceled, common.Paused, common.Failed, common.Running, common.Completed}, t.getState()) {
			return
		}

		t.mu.Lock()
		t.totalPhases = len(fs)
		t.execAt = time.Now()
		t.state = common.Running
		t.running = true
		t.details = nil
		t.mu.Unlock()

		for phase, f := range t.funcs {
			t.mu.Lock()
			t.currentPhase = phase + 1
			currentPhase := t.currentPhase
			totalPhases := t.totalPhases
			t.mu.Unlock()

			klog.Infof("[Task] Id: %s, exec phase: %d/%d", t.id, currentPhase, totalPhases)
			err = f()

			if err != nil {
				klog.Errorf("[Task] Id: %s, exec failed, error: %s", t.id, err.Error())

				t.mu.Lock()
				errmsg := common.RemoveBlank(err.Error())
				t.message = errmsg
				t.state = common.Failed
				t.details = append(t.details, errmsg)
				t.mu.Unlock()
				return
			}
		}

		t.mu.Lock()
		t.state = common.Completed
		t.progress = 100
		t.transfer = t.totalSize
		t.details = append(t.details, "successed")
		t.mu.Unlock()
	}()
}

func (t *Task) updateProgress(progress int, transfer int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progress = progress
	t.transfer += transfer
	t.details = append(t.details, fmt.Sprintf("rsync files progress: %d, transfer: %d", progress, transfer))
}

func (t *Task) updateProgressRsync(progress int, transfer int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progress = progress
	t.transfer = transfer
	t.details = append(t.details, fmt.Sprintf("rsync files progress: %d, transfer: %d", progress, transfer))
}

func (t *Task) resetProgressZero() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progress = 0
	t.transfer = 0
	t.details = append(t.details, fmt.Sprintf("rsync files progress: 0, transfer: 0"))
}

func (t *Task) updateTotalSize(totalSize int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalSize = totalSize
}

func (t *Task) GetProgress() (string, int, int64, int64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state, t.progress, t.transfer, t.totalSize
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

	lower := strings.ToLower(s)
	if strings.Contains(lower, "accessdenied") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "403 forbidden") ||
		strings.Contains(lower, "insufficient_scope") ||
		strings.Contains(lower, "insufficientfilepermissions") ||
		strings.Contains(lower, "invalid_grant") ||
		strings.Contains(lower, "expired_access_token") ||
		strings.Contains(lower, "permission denied") {
		return errors.New(common.ErrorMessagePermissionDenied)
	}

	return errors.New(s)
}
