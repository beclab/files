//go:build linux

package files

import (
	"path/filepath"
	"testing"
)

// Smoke test that SupportsOwnership doesn't panic on real or missing
// paths. Result is environment-dependent so we don't assert it.
func TestSupportsOwnership(t *testing.T) {
	_ = SupportsOwnership("/")
	_ = SupportsOwnership(filepath.Join(t.TempDir(), "nonexistent"))
}
