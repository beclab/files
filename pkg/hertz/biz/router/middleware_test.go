package router

import (
	"testing"
)

// TestCheckNonSharedPath_KnownPaths pins the well-formed routing
// decisions that the share middleware relies on. It deliberately
// avoids loose-matching edge cases (e.g. paths that just *contain*
// "/api/share" as a substring); those are covered separately so that
// future tightening (path-prefix matching) can update only that test.
func TestCheckNonSharedPath_KnownPaths(t *testing.T) {
	mustSkipShare := []string{
		"/api/nodes",
		"/api/nodes/master",
		"/api/task",
		"/api/task/list",
		"/api/accounts",
		"/api/accounts/me",
		"/api/users",
		"/api/users/alice",
		"/api/share",
		"/api/share/path",
		"/api/mounted",
		"/api/mount",
		"/api/mount/abc",
		"/api/unmount",
		"/api/unmount/abc",
		"/api/smb_history",
		"/api/search",
		"/videos/",
		"/videos/preview/abc.mp4",
	}
	for _, p := range mustSkipShare {
		t.Run("skip="+p, func(t *testing.T) {
			if got := checkNonSharedPath(p); got {
				t.Fatalf("checkNonSharedPath(%q) = true, want false (path should bypass share middleware)", p)
			}
		})
	}

	mustGoThroughShare := []string{
		"/api/resources/master/path/file.txt",
		"/api/preview/master/path/file.png",
		"/api/raw/master/path/file.bin",
		"/api/paste/master",
		"/upload/upload-link/master",
		"/upload/file-uploaded-bytes/master",
	}
	for _, p := range mustGoThroughShare {
		t.Run("share="+p, func(t *testing.T) {
			if got := checkNonSharedPath(p); !got {
				t.Fatalf("checkNonSharedPath(%q) = false, want true (path should hit share middleware)", p)
			}
		})
	}
}

// TestCheckNonSharedPath_RejectsPrefixCollisions is a regression test
// for the previous strings.Contains-based gate. Paths that merely
// share a textual prefix with a non-share route - e.g. /api/sharefoo
// vs /api/share - must NOT bypass the share middleware. Loosening
// this check effectively grants unauthenticated access to whatever
// route an attacker can craft on top of the prefix.
func TestCheckNonSharedPath_RejectsPrefixCollisions(t *testing.T) {
	mustGoThroughShare := []string{
		"/api/sharefoo",
		"/api/sharepoint/file",
		"/api/share-thing",
		"/api/users-data",
		"/api/usersbackup/dump",
		"/api/nodesfoo",
		"/api/taskmanager/list",
		"/api/searchengine",
		"/videos",
		"/videosproxy/x",
	}
	for _, p := range mustGoThroughShare {
		t.Run(p, func(t *testing.T) {
			if got := checkNonSharedPath(p); !got {
				t.Fatalf("checkNonSharedPath(%q) = false, want true (prefix collision must not bypass share middleware)", p)
			}
		})
	}
}
