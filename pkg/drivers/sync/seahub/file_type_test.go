package seahub

import (
	"os"
	"testing"
)

func TestSyncPermToMode(t *testing.T) {
	cases := []struct {
		perm string
		want os.FileMode
	}{
		{"", 0},
		{"r", 0555},
		{"preview", 0555},
		{"cloud-edit", 0555},
		{"rw", 0755},
		{"admin", 0755},
		{"custom-xyz", 0},
		{"  rw  ", 0755},
		{" r", 0555},
	}
	for _, tc := range cases {
		if got := SyncPermToMode(tc.perm); got != tc.want {
			t.Errorf("SyncPermToMode(%q) = %#o, want %#o", tc.perm, got, tc.want)
		}
	}
}
