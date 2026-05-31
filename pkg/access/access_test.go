package access

import (
	"context"
	"testing"

	"files/pkg/drivers"
	"files/pkg/global"
	"files/pkg/integration"
	"files/pkg/models"
)

// withGlobals swaps the package-level globals CheckAccess depends on and
// returns a restore func. Tests run serially (no t.Parallel), so this is
// safe for hermetic dispatch checks that never touch k8s or disk.
func withGlobals(t *testing.T, data *global.Data, withAdaptor bool) {
	t.Helper()
	savedData := global.GlobalData
	savedAdaptor := drivers.Adaptor
	savedIntegration := integration.IntegrationService

	global.GlobalData = data
	integration.IntegrationService = nil
	if withAdaptor {
		drivers.NewDriverHandler()
	} else {
		drivers.Adaptor = nil
	}

	t.Cleanup(func() {
		global.GlobalData = savedData
		drivers.Adaptor = savedAdaptor
		integration.IntegrationService = savedIntegration
	})
}

func userData(owner string) *global.Data {
	return &global.Data{UserPvcMap: map[string]string{owner: "pvc-" + owner}}
}

func TestCheckAccess_InputValidation(t *testing.T) {
	ctx := context.Background()

	// Empty owner is rejected before anything else; no globals needed.
	if lvl, err := CheckAccess(ctx, "", "/drive/Home/a.txt"); err == nil || lvl != models.LevelNone {
		t.Fatalf("empty owner: got (%v, %v), want (LevelNone, error)", lvl, err)
	}

	// Unresolvable URL fails before owner validation / dispatch.
	if lvl, err := CheckAccess(ctx, "alice", "/foobar/x/y"); err == nil || lvl != models.LevelNone {
		t.Fatalf("bad url: got (%v, %v), want (LevelNone, error)", lvl, err)
	}
}

func TestCheckAccess_UnknownOwner(t *testing.T) {
	// No PVC entry and no integration service -> user not found.
	withGlobals(t, &global.Data{UserPvcMap: map[string]string{}}, true)

	if lvl, err := CheckAccess(context.Background(), "ghost", "/drive/Home/a.txt"); err == nil || lvl != models.LevelNone {
		t.Fatalf("unknown owner: got (%v, %v), want (LevelNone, error)", lvl, err)
	}
}

func TestCheckAccess_DriveHomeAdmin(t *testing.T) {
	withGlobals(t, userData("alice"), true)

	lvl, err := CheckAccess(context.Background(), "alice", "/drive/Home/a.txt")
	if err != nil {
		t.Fatalf("drive/Home: unexpected error: %v", err)
	}
	if lvl != models.LevelAdmin {
		t.Fatalf("drive/Home: level = %v, want LevelAdmin", lvl)
	}
	if !lvl.Allow(models.ActionWrite) {
		t.Fatalf("drive/Home: Allow(Write) = false, want true")
	}
}

func TestCheckAccess_DriveCommonReadOnlyForNonAdmin(t *testing.T) {
	// IntegrationService is nil, so posix CheckPermission falls back to
	// LevelRead for the shared drive/Common volume.
	withGlobals(t, userData("alice"), true)

	lvl, err := CheckAccess(context.Background(), "alice", "/drive/Common/docs/x.md")
	if err != nil {
		t.Fatalf("drive/Common: unexpected error: %v", err)
	}
	if lvl != models.LevelRead {
		t.Fatalf("drive/Common: level = %v, want LevelRead", lvl)
	}
	if lvl.Allow(models.ActionWrite) {
		t.Fatalf("drive/Common: Allow(Write) = true, want false")
	}
	if !lvl.Allow(models.ActionRead) {
		t.Fatalf("drive/Common: Allow(Read) = false, want true")
	}
}

func TestCheckAccessParam(t *testing.T) {
	withGlobals(t, userData("alice"), true)
	ctx := context.Background()

	if lvl, err := CheckAccessParam(ctx, "alice", nil); err == nil || lvl != models.LevelNone {
		t.Fatalf("nil fp: got (%v, %v), want (LevelNone, error)", lvl, err)
	}

	fp := &models.FileParam{Owner: "alice", FileType: "drive", Extend: "Home", Path: "/a.txt"}
	lvl, err := CheckAccessParam(ctx, "alice", fp)
	if err != nil {
		t.Fatalf("CheckAccessParam: unexpected error: %v", err)
	}
	if lvl != models.LevelAdmin {
		t.Fatalf("CheckAccessParam: level = %v, want LevelAdmin", lvl)
	}
}

func TestCheckAccess_NilAdaptor(t *testing.T) {
	// Owner is valid (PVC), but the driver adaptor is not initialized.
	withGlobals(t, userData("alice"), false)

	fp := &models.FileParam{Owner: "alice", FileType: "drive", Extend: "Home", Path: "/a.txt"}
	if lvl, err := CheckAccessParam(context.Background(), "alice", fp); err == nil || lvl != models.LevelNone {
		t.Fatalf("nil adaptor: got (%v, %v), want (LevelNone, error)", lvl, err)
	}
}

func TestCheckAccess_ParamParity(t *testing.T) {
	// CheckAccess (URL) and CheckAccessParam (parsed fp) must resolve to
	// the same Level for the same resource.
	withGlobals(t, userData("alice"), true)
	ctx := context.Background()

	lvlURL, err := CheckAccess(ctx, "alice", "/drive/Home/a.txt")
	if err != nil {
		t.Fatalf("CheckAccess: unexpected error: %v", err)
	}
	fp := &models.FileParam{Owner: "alice", FileType: "drive", Extend: "Home", Path: "/a.txt"}
	lvlParam, err := CheckAccessParam(ctx, "alice", fp)
	if err != nil {
		t.Fatalf("CheckAccessParam: unexpected error: %v", err)
	}
	if lvlURL != lvlParam {
		t.Fatalf("parity mismatch: CheckAccess=%v CheckAccessParam=%v", lvlURL, lvlParam)
	}
}
