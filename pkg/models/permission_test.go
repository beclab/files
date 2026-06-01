package models

import "testing"

func TestLevelAllow(t *testing.T) {
	cases := []struct {
		level  Level
		action Action
		want   bool
	}{
		{LevelNone, ActionList, false},
		{LevelNone, ActionRead, false},
		{LevelRead, ActionList, true},
		{LevelRead, ActionRead, true},
		{LevelRead, ActionPreview, true},
		{LevelRead, ActionDownload, true},
		{LevelRead, ActionWrite, false},
		{LevelRead, ActionUpload, false},
		{LevelRead, ActionDelete, false},
		{LevelRead, ActionShareManage, false},
		{LevelWrite, ActionRead, true},
		{LevelWrite, ActionWrite, true},
		{LevelWrite, ActionUpload, true},
		{LevelWrite, ActionDelete, true},
		{LevelWrite, ActionShareManage, false},
		{LevelAdmin, ActionWrite, true},
		{LevelAdmin, ActionShareManage, true},
	}
	for _, c := range cases {
		if got := c.level.Allow(c.action); got != c.want {
			t.Errorf("Level(%v).Allow(%v) = %v, want %v", c.level, c.action, got, c.want)
		}
	}
}

func TestLevelFromSyncPermission(t *testing.T) {
	cases := map[string]Level{
		"":           LevelNone,
		"unknown":    LevelNone,
		"r":          LevelRead,
		"preview":    LevelRead,
		"cloud-edit": LevelRead,
		"rw":         LevelWrite,
		"admin":      LevelAdmin,
		"custom-xyz": LevelNone,
		// LevelFromSyncPermission does not trim; callers (ResolveSyncLevel)
		// are responsible for trimming, so padded input is unrecognized.
		"  rw  ": LevelNone,
		" r":     LevelNone,
	}
	for perm, want := range cases {
		if got := LevelFromSyncPermission(perm); got != want {
			t.Errorf("LevelFromSyncPermission(%q) = %v, want %v", perm, got, want)
		}
	}
}
