package posix

import (
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"os"
	"testing"
	"time"
)

func TestGetExternalMountName(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
		ok       bool
	}{
		{name: "root slash", path: "/", expected: "", ok: false},
		{name: "empty", path: "", expected: "", ok: false},
		{name: "mount root", path: "/smb-1/", expected: "smb-1", ok: true},
		{name: "mount child", path: "/smb-1/docs/a.txt", expected: "smb-1", ok: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := getExternalMountName(tc.path)
			if ok != tc.ok {
				t.Fatalf("ok mismatch: got=%v expected=%v", ok, tc.ok)
			}
			if got != tc.expected {
				t.Fatalf("name mismatch: got=%q expected=%q", got, tc.expected)
			}
		})
	}
}

func TestShouldUseFastExternalRootList(t *testing.T) {
	s := &PosixStorage{}

	rootExternal := &models.FileParam{
		FileType: common.External,
		Path:     "/",
	}
	if !s.shouldUseFastExternalRootList(rootExternal, "") {
		t.Fatalf("expected fast list for external root")
	}

	mountPath := &models.FileParam{
		FileType: common.External,
		Path:     "/smb-1/",
	}
	if s.shouldUseFastExternalRootList(mountPath, "") {
		t.Fatalf("did not expect fast list inside mount path")
	}

	if !s.shouldUseFastExternalRootList(rootExternal, "share-id") {
		t.Fatalf("expected fast list for share listing")
	}
}

func TestHydrateExternalRootItemMetadataNonMounted(t *testing.T) {
	dir := t.TempDir()
	fullPath := dir + "/local.txt"
	if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	modTime := time.Date(2026, 6, 1, 7, 0, 0, 0, time.UTC)
	if err := os.Chtimes(fullPath, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	item := &files.FileInfo{
		Name:  "local.txt",
		Path:  "/local.txt",
		IsDir: false,
	}

	hydrateExternalRootItemMetadata(item, fullPath, "/", nil, false)

	if !item.ModTime.Equal(modTime) {
		t.Fatalf("mod time mismatch: got=%s expected=%s", item.ModTime, modTime)
	}
	if item.Size != int64(len("content")) {
		t.Fatalf("size mismatch: got=%d", item.Size)
	}
	if item.Path != "/local.txt" {
		t.Fatalf("path mismatch: got=%q", item.Path)
	}
}

func TestHydrateExternalRootItemMetadataMountedValid(t *testing.T) {
	dir := t.TempDir()
	fullPath := dir + "/mounted"
	if err := os.Mkdir(fullPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	modTime := time.Date(2026, 6, 1, 7, 1, 0, 0, time.UTC)
	if err := os.Chtimes(fullPath, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	item := &files.FileInfo{
		Name:  "mounted",
		Path:  "/mounted/",
		IsDir: true,
	}
	mounted := &files.DiskInfo{
		Path:    "mounted-valid-test",
		Type:    "usb",
		Invalid: false,
	}

	// Skip the asynchronous mount probe by injecting a healthy state directly.
	externalMountGuard.mu.Lock()
	externalMountGuard.states[mounted.Path] = &mountGuardState{
		mounted:       true,
		probeState:    mountProbeHealthy,
		lastHealthyAt: time.Now(),
		probePath:     fullPath,
	}
	externalMountGuard.mu.Unlock()
	defer func() {
		externalMountGuard.mu.Lock()
		delete(externalMountGuard.states, mounted.Path)
		externalMountGuard.mu.Unlock()
	}()

	hydrateExternalRootItemMetadata(item, fullPath, "/", mounted, true)

	if !item.ModTime.Equal(modTime) {
		t.Fatalf("mod time mismatch: got=%s expected=%s", item.ModTime, modTime)
	}
	if !item.IsDir {
		t.Fatalf("expected item to remain a directory")
	}
	if item.Path != "/mounted/" {
		t.Fatalf("path mismatch: got=%q", item.Path)
	}
}

func TestHydrateExternalRootItemMetadataMountedInvalidSkipsStat(t *testing.T) {
	dir := t.TempDir()
	fullPath := dir + "/invalid"
	if err := os.Mkdir(fullPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	item := &files.FileInfo{
		Name:  "invalid",
		Path:  "/invalid/",
		IsDir: true,
	}
	mounted := &files.DiskInfo{
		Path:    "mounted-invalid-test",
		Type:    "smb",
		Invalid: true,
	}

	hydrateExternalRootItemMetadata(item, fullPath, "/", mounted, true)

	if !item.ModTime.IsZero() {
		t.Fatalf("invalid mounted item should not be statted, got mod time %s", item.ModTime)
	}
}
