package lifecycle

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain wraps every test in this package with goleak so that any
// hook / channel / wait that fails to clean up after itself is
// flagged as a leaked goroutine. This is the same kind of guard the
// recent ticker / SimulateProgress / UploadDirToSync fixes (PRs
// #250, #251, #252, #253, #254, #265, #267) added at the source
// level - here we make sure the lifecycle package's own contracts
// (Coordinator runs hooks, hooks unblock waiters) keep that
// property across future edits.
//
// If a test legitimately needs a long-lived background goroutine
// it should pass an explicit ignore list to goleak.VerifyNone in
// that test; the default here is "no leaked goroutines at all".
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
