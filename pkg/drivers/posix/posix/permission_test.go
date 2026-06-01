package posix

import (
	"testing"

	"files/pkg/integration"
	"files/pkg/models"
)

// TestCheckPermissionNonCommon covers the branches of
// PosixStorage.CheckPermission that do not require a populated
// integration service: owner-scoped resources are always admin, and
// drive/Common with no integration service falls back to read-only.
func TestCheckPermission(t *testing.T) {
	s := &PosixStorage{}

	ownerScoped := []*models.FileParam{
		{FileType: "drive", Extend: "Home", Path: "/a.txt"},
		{FileType: "drive", Extend: "Data", Path: "/a.txt"},
		{FileType: "cache", Extend: "node1", Path: "/a.txt"},
		{FileType: "external", Extend: "node1", Path: "/a.txt"},
	}
	for _, fp := range ownerScoped {
		lvl, err := s.CheckPermission(fp, "alice")
		if err != nil {
			t.Fatalf("CheckPermission(%s/%s) error: %v", fp.FileType, fp.Extend, err)
		}
		if lvl != models.LevelAdmin {
			t.Errorf("CheckPermission(%s/%s) = %v, want LevelAdmin", fp.FileType, fp.Extend, lvl)
		}
	}

	// drive/Common with no integration service available: a non-admin
	// (here, unknown) owner gets read-only.
	orig := integration.IntegrationService
	integration.IntegrationService = nil
	defer func() { integration.IntegrationService = orig }()

	common := &models.FileParam{FileType: "drive", Extend: "Common", Path: "/"}
	lvl, err := s.CheckPermission(common, "alice")
	if err != nil {
		t.Fatalf("CheckPermission(drive/Common) error: %v", err)
	}
	if lvl != models.LevelRead {
		t.Errorf("CheckPermission(drive/Common, no integration) = %v, want LevelRead", lvl)
	}
}

// TestCheckPermissionDriveCommonRole exercises the platform-role branch of
// drive/Common via the isPlatformAdmin seam: platform admins get LevelAdmin,
// everyone else LevelRead.
func TestCheckPermissionDriveCommonRole(t *testing.T) {
	s := &PosixStorage{}
	common := &models.FileParam{FileType: "drive", Extend: "Common", Path: "/docs"}

	saved := isPlatformAdmin
	t.Cleanup(func() { isPlatformAdmin = saved })

	isPlatformAdmin = func(owner string) bool { return owner == "boss" }

	cases := map[string]models.Level{
		"boss":  models.LevelAdmin,
		"alice": models.LevelRead,
	}
	for owner, want := range cases {
		lvl, err := s.CheckPermission(common, owner)
		if err != nil {
			t.Fatalf("CheckPermission(drive/Common, %s) error: %v", owner, err)
		}
		if lvl != want {
			t.Errorf("CheckPermission(drive/Common, %s) = %v, want %v", owner, lvl, want)
		}
	}
}
