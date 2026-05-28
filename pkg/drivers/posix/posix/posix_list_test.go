package posix

import (
	"files/pkg/common"
	"files/pkg/models"
	"testing"
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

	if s.shouldUseFastExternalRootList(rootExternal, "share-id") {
		t.Fatalf("did not expect fast list for share listing")
	}
}
