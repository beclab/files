package precheck

import (
	"files/pkg/models"
	"strings"
	"testing"
)

// TestSourceExists_StructuralValidation pins the cheap, side-effect-free
// rejections so a future refactor that swaps the switch for a registry
// doesn't accidentally drop the nil-guards or the unsupported-type
// branch. These cases never reach any backend RPC, so they're safe to
// run without seafile / rclone / k8s wired up.
func TestSourceExists_StructuralValidation(t *testing.T) {
	cases := []struct {
		name             string
		param            *models.PasteParam
		wantErrSubstring string
	}{
		{
			name:             "nil param",
			param:            nil,
			wantErrSubstring: "paste param is nil",
		},
		{
			name:             "nil src",
			param:            &models.PasteParam{},
			wantErrSubstring: "paste source param is nil",
		},
		{
			name: "unsupported file type",
			param: &models.PasteParam{
				Src: &models.FileParam{FileType: "ftp", Extend: "host", Path: "/x"},
			},
			wantErrSubstring: "unsupported source file type for precheck: ftp",
		},
		{
			name: "empty sync repo id",
			param: &models.PasteParam{
				Src: &models.FileParam{FileType: "sync", Extend: "", Path: "/x"},
			},
			wantErrSubstring: "sync source not found: repo id is empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := SourceExists(tc.param)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrSubstring) {
				t.Fatalf("error mismatch:\n got: %q\nwant substring: %q", err.Error(), tc.wantErrSubstring)
			}
		})
	}
}

// TestSourceExists_ShareOwnerOverride documents that share=1 swaps the
// effective owner used for downstream RPC calls. We can't actually
// dial seafile here, so we only assert that the type-routing reaches
// the right branch (sync -> checkSync -> empty-repo) without panicking
// on the share-resolution code path.
func TestSourceExists_ShareOwnerOverride(t *testing.T) {
	p := &models.PasteParam{
		Share:    1,
		SrcOwner: "grantor",
		Src: &models.FileParam{
			FileType: "sync",
			Owner:    "recipient",
			Extend:   "", // forces fast-path "repo id is empty" from checkSync
			Path:     "/x",
		},
	}

	err := SourceExists(p)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "sync source not found") {
		t.Fatalf("expected sync-not-found error, got: %v", err)
	}
}
