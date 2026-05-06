package tasks

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestTask_SnapshotUnderConcurrentUpdates pins the property that
// PR #241 (B2) added: concurrent worker-side state updates and
// HTTP-side snapshot reads must not race and must produce
// internally consistent views.
//
// The HTTP path is modelled by snapshot(); the worker path is
// modelled by updateProgress and updateTotalSize (the locked
// accessors that already exist on main today; helpers added by
// PR #266 / #267 such as appendDetail / setTidyDirs / pausedSnap
// will get their own follow-up tests once those PRs land).
//
// Run with -race to catch any future regression that drops the
// mutex on either side.
func TestTask_SnapshotUnderConcurrentUpdates(t *testing.T) {
	const (
		updaters = 4
		readers  = 4
		duration = 200 * time.Millisecond
	)

	task := &Task{id: "test"}
	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup

	// Updater goroutines: model the worker phase doing progress
	// updates and total-size resets.
	for i := 0; i < updaters; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			n := int64(seed)
			for time.Now().Before(deadline) {
				n++
				task.updateProgress(int(n%100), n)
				if n%3 == 0 {
					task.updateTotalSize(n * 1024)
				}
			}
		}(i)
	}

	// Reader goroutines: model HTTP GetTask / GetTasksByStatus
	// projecting through snapshot(). The only invariant we can pin
	// down without coordinating with updaters is: Progress is in
	// [0, 100] and TotalSize is non-negative. Any tear of multi-word
	// fields (string `state`, int64 sizes) would be flagged by the
	// race detector.
	var snapshots int64
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				snap := task.snapshot()
				atomic.AddInt64(&snapshots, 1)
				if snap.Progress < 0 || snap.Progress >= 100 {
					t.Errorf("snapshot Progress out of range: %d", snap.Progress)
					return
				}
				if snap.TotalSize < 0 {
					t.Errorf("snapshot TotalSize negative: %d", snap.TotalSize)
					return
				}
				_ = snap.State
				_ = snap.Message
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt64(&snapshots) == 0 {
		t.Fatalf("no snapshots taken; reader goroutines never ran")
	}
}

// TestTask_GetStateConsistent makes sure getState() is callable
// concurrently with updateProgress (which writes other fields under
// the same mutex). Race detector is the real assertion here; the
// returned value just has to be the empty string we never set.
func TestTask_GetStateConsistent(t *testing.T) {
	task := &Task{id: "test"}

	const duration = 100 * time.Millisecond
	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := int64(0); time.Now().Before(deadline); n++ {
			task.updateProgress(int(n%100), n)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for time.Now().Before(deadline) {
			_ = task.getState()
		}
	}()

	wg.Wait()
}
