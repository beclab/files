package tasks

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain wraps every test in this package with goleak so that any
// task worker / pond pool / ticker / channel that fails to clean up
// is flagged as a leaked goroutine. This is the regression guard for
// the recent ticker fix (PR #265, SimulateProgress) and the
// goroutine-Cancel design in PR #243.
//
// Notes on ignored functions:
//
//   - go.uber.org/atomic isn't used here, but pond v2 spins up a
//     scheduler / dispatcher goroutine when a pool is created. The
//     manager_test.go tests do create *userPool values, and the pool
//     keeps a worker goroutine alive until StopAndWait. If a future
//     test forgets to drain its pool the goleak check will flag it,
//     which is what we want; we therefore do NOT add a blanket
//     ignore for pond. Instead, tests must call userPool.pool.Stop()
//     or rely on the LoadOrStore-loser branch of getOrCreateUserPool
//     that already calls StopAndWait.
//   - klog has its own logging goroutine in some configurations; if
//     CI flakes on it we can add it to the ignore list, but the
//     default klog.SetOutput is synchronous so it should not appear.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
