package posix

import (
	"errors"
	"files/pkg/common"
	"testing"
	"time"
)

func TestMountGuardBeginReturnsUserFacingMessageForInvalid(t *testing.T) {
	g := newMountGuard(time.Second, time.Second, time.Second)

	_, _, err := g.begin("smb", true, "get_files")
	if err == nil {
		t.Fatalf("expected invalid mount to be rejected")
	}
	if err.Error() != common.ErrorMessageExternalMountUnavailable {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestMountGuardBeginReturnsUserFacingMessageWhileProbeInProgress(t *testing.T) {
	g := newMountGuard(time.Second, time.Second, time.Second)

	startProbe, generation, err := g.begin("smb", false, "get_files")
	if err != nil || !startProbe {
		t.Fatalf("expected first probe begin to succeed, start=%v err=%v", startProbe, err)
	}
	if generation == 0 {
		t.Fatalf("expected non-zero generation")
	}

	_, _, err = g.begin("smb", false, "get_files")
	if err == nil {
		t.Fatalf("expected second begin while probing to fail")
	}
	if err.Error() != common.ErrorMessageExternalMountUnavailable {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestMountGuardBeginReturnsUserFacingMessageDuringCooldown(t *testing.T) {
	g := newMountGuard(time.Second, time.Minute, time.Second)

	startProbe, generation, err := g.begin("smb", false, "get_files")
	if err != nil || !startProbe {
		t.Fatalf("expected first probe begin to succeed, start=%v err=%v", startProbe, err)
	}
	g.finishProbe("smb", generation, errors.New("probe failed"))

	_, _, err = g.begin("smb", false, "get_files")
	if err == nil {
		t.Fatalf("expected cooldown begin to fail")
	}
	if err.Error() != common.ErrorMessageExternalMountUnavailable {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestMountGuardRunReturnsUserFacingMessageOnProbeTimeout(t *testing.T) {
	g := newMountGuard(5*time.Millisecond, time.Second, time.Second)

	err := g.run("smb", false, "get_files", func() error {
		time.Sleep(30 * time.Millisecond)
		return nil
	})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if err.Error() != common.ErrorMessageExternalMountUnavailable {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}
