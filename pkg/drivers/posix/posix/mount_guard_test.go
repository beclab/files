package posix

import (
	"files/pkg/common"
	"files/pkg/files"
	"testing"
	"time"
)

func waitForProbeState(t *testing.T, g *mountGuard, mountName string, expected mountProbeState) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		state := g.states[mountName]
		var got mountProbeState
		if state != nil {
			got = state.probeState
		}
		g.mu.Unlock()

		if got == expected {
			return
		}
		time.Sleep(time.Millisecond)
	}

	g.mu.Lock()
	state := g.states[mountName]
	g.mu.Unlock()
	t.Fatalf("timed out waiting for probe state %s, got=%+v", expected, state)
}

func TestMountGuardEnsureAvailableReturnsUserFacingMessageForInvalid(t *testing.T) {
	g := newMountGuard(time.Second, time.Second, time.Second)
	g.probe = func(string) error { return nil }
	defer g.reconcile(nil)

	err := g.ensureAvailable("smb", &files.DiskInfo{Path: "smb", Invalid: true}, "get_files")
	if err == nil {
		t.Fatalf("expected invalid mount to be rejected")
	}
	if err.Error() != common.ErrorMessageExternalMountUnavailable {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestMountGuardEnsureAvailableAllowsHealthyMount(t *testing.T) {
	g := newMountGuard(time.Second, time.Second, time.Second)
	g.probeInterval = time.Hour
	g.probe = func(string) error { return nil }
	defer g.reconcile(nil)

	g.reconcile([]files.DiskInfo{{Path: "smb", Invalid: false}})
	waitForProbeState(t, g, "smb", mountProbeHealthy)

	if err := g.ensureAvailable("smb", &files.DiskInfo{Path: "smb", Invalid: false}, "get_files"); err != nil {
		t.Fatalf("expected healthy mount to be available: %v", err)
	}
}

func TestMountGuardEnsureAvailableAllowsRecentHealthyWhileProbeInProgress(t *testing.T) {
	g := newMountGuard(time.Second, time.Second, time.Second)
	g.states["smb"] = &mountGuardState{
		mounted:       true,
		probeState:    mountProbeProbing,
		lastHealthyAt: time.Now(),
		stopCh:        make(chan struct{}),
	}
	defer g.reconcile(nil)

	if err := g.ensureAvailable("smb", &files.DiskInfo{Path: "smb", Invalid: false}, "upload_chunks"); err != nil {
		t.Fatalf("expected recent healthy mount to remain available while probing: %v", err)
	}
}

func TestMountGuardEnsureAvailableReturnsUserFacingMessageDuringCooldown(t *testing.T) {
	g := newMountGuard(time.Second, time.Minute, time.Second)
	g.states["smb"] = &mountGuardState{
		mounted:       true,
		probeState:    mountProbeUnhealthy,
		cooldownUntil: time.Now().Add(time.Minute),
		lastProbeErr:  "probe failed",
		stopCh:        make(chan struct{}),
	}
	defer g.reconcile(nil)

	err := g.ensureAvailable("smb", &files.DiskInfo{Path: "smb", Invalid: false}, "get_files")
	if err == nil {
		t.Fatalf("expected cooldown to be rejected")
	}
	if err.Error() != common.ErrorMessageExternalMountUnavailable {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestMountGuardProbeTimeoutMarksMountUnavailable(t *testing.T) {
	g := newMountGuard(5*time.Millisecond, time.Second, time.Second)
	g.probeInterval = time.Hour
	block := make(chan struct{})
	g.probe = func(string) error {
		<-block
		return nil
	}
	defer func() {
		close(block)
		g.reconcile(nil)
	}()

	g.reconcile([]files.DiskInfo{{Path: "smb", Invalid: false}})
	waitForProbeState(t, g, "smb", mountProbeUnhealthy)

	err := g.ensureAvailable("smb", &files.DiskInfo{Path: "smb", Invalid: false}, "get_files")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if err.Error() != common.ErrorMessageExternalMountUnavailable {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}
