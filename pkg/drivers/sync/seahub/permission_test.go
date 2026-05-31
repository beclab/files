package seahub

import (
	"errors"
	"testing"

	"files/pkg/models"
)

// stubRPC overrides the GetRepoStatus / CheckPermissionByPath seams and
// restores them on cleanup. status/perm are the values to return; statusErr
// and permErr inject RPC failures.
func stubRPC(t *testing.T, status int, statusErr error, perm string, permErr error) {
	t.Helper()
	savedStatus := getRepoStatus
	savedPerm := checkPermissionByPath
	getRepoStatus = func(string) (int, error) { return status, statusErr }
	checkPermissionByPath = func(string, string, string) (string, error) { return perm, permErr }
	t.Cleanup(func() {
		getRepoStatus = savedStatus
		checkPermissionByPath = savedPerm
	})
}

func TestSyncParentDir(t *testing.T) {
	cases := map[string]string{
		"":           "/",
		"/":          "/",
		"a":          "/",
		"/a":         "/",
		"/a/":        "/",
		"/a/b.txt":   "/a/",
		"/a/b/":      "/a/",
		"/a/b/c.txt": "/a/b/",
		"a/b.txt":    "a/",
	}
	for in, want := range cases {
		if got := SyncParentDir(in); got != want {
			t.Errorf("SyncParentDir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckFolderPermission_ReadOnlyRepoShortCircuits(t *testing.T) {
	// repoStatus == 1 -> read-only repo; must short-circuit to PERMISSION_READ
	// without consulting CheckPermissionByPath (which would return "rw").
	stubRPC(t, 1, nil, "rw", nil)

	perm, err := CheckFolderPermission("alice@auth.local", "repo1", "/docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perm != PERMISSION_READ {
		t.Fatalf("perm = %q, want %q", perm, PERMISSION_READ)
	}
}

func TestCheckFolderPermission_RPCErrorsPropagate(t *testing.T) {
	t.Run("status error", func(t *testing.T) {
		stubRPC(t, 0, errors.New("status rpc down"), "rw", nil)
		if _, err := CheckFolderPermission("alice@auth.local", "repo1", "/x"); err == nil {
			t.Fatalf("want error from GetRepoStatus failure")
		}
	})
	t.Run("perm error", func(t *testing.T) {
		stubRPC(t, 0, nil, "", errors.New("perm rpc down"))
		if _, err := CheckFolderPermission("alice@auth.local", "repo1", "/x"); err == nil {
			t.Fatalf("want error from CheckPermissionByPath failure")
		}
	})
}

func TestResolveSyncLevel_Mapping(t *testing.T) {
	tests := []struct {
		name string
		perm string
		want models.Level
	}{
		{"read", "r", models.LevelRead},
		{"preview", "preview", models.LevelRead},
		{"cloud-edit", "cloud-edit", models.LevelRead},
		{"read-write", "rw", models.LevelWrite},
		{"admin", "admin", models.LevelAdmin},
		{"empty", "", models.LevelNone},
		{"unknown", "custom-xyz", models.LevelNone},
		{"whitespace padded", "  rw  ", models.LevelWrite},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubRPC(t, 0, nil, tc.perm, nil)
			lvl, err := ResolveSyncLevel("alice@auth.local", "repo1", "/docs")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lvl != tc.want {
				t.Fatalf("level = %v, want %v", lvl, tc.want)
			}
		})
	}
}

func TestEnsureSyncPermission(t *testing.T) {
	t.Run("allow", func(t *testing.T) {
		stubRPC(t, 0, nil, "rw", nil)
		if err := EnsureSyncPermission("alice@auth.local", "repo1", "/docs", models.ActionWrite); err != nil {
			t.Fatalf("allow: unexpected error: %v", err)
		}
	})

	t.Run("deny", func(t *testing.T) {
		stubRPC(t, 0, nil, "r", nil)
		err := EnsureSyncPermission("alice@auth.local", "repo1", "/docs", models.ActionWrite)
		if !errors.Is(err, ErrSyncPermissionDenied) {
			t.Fatalf("deny: err = %v, want ErrSyncPermissionDenied", err)
		}
	})

	t.Run("rpc error not masked as denial", func(t *testing.T) {
		stubRPC(t, 0, errors.New("rpc down"), "", nil)
		err := EnsureSyncPermission("alice@auth.local", "repo1", "/docs", models.ActionRead)
		if err == nil || errors.Is(err, ErrSyncPermissionDenied) {
			t.Fatalf("rpc error: err = %v, want underlying error (not ErrSyncPermissionDenied)", err)
		}
	})
}
