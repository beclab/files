package router

import (
	"files/pkg/common"
	"net/http"
	"testing"
	"time"
)

// TestCheckPermission_Matrix pins the share permission matrix that
// every share-API request runs through. The matrix encodes the trust
// boundary between owners, internal members, and external link
// recipients; future refactors (CORS hardening, path-matching cleanup,
// streaming rewrite) must not silently shift any of these decisions.
//
// Permission values, as documented in checkPermission:
//
//	0 - no permit
//	1 - view, download
//	2 - upload only (external links only)
//	3 - upload + download
//	4 - admin
func TestCheckPermission_Matrix(t *testing.T) {
	const (
		owner = "alice"
		other = "bob"
	)

	access := func(method string, mutate func(a *ShareAccess)) *ShareAccess {
		a := &ShareAccess{Method: method}
		if mutate != nil {
			mutate(a)
		}
		return a
	}

	cases := []struct {
		name        string
		currentUser string
		shareBy     string
		shareType   string
		permission  int32
		access      *ShareAccess
		want        bool
	}{
		{
			name:        "internal share owner always allowed",
			currentUser: owner, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 0,
			access: access(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   true,
		},
		{
			name:        "internal share owner allowed even for upload",
			currentUser: owner, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 0,
			access: access(http.MethodPost, func(a *ShareAccess) { a.Upload = true }),
			want:   true,
		},
		{
			name:        "external owner short-circuit does NOT apply (handled by checkExternal caller)",
			currentUser: owner, shareBy: owner,
			shareType: common.ShareTypeExternal, permission: 0,
			access: access(http.MethodGet, nil),
			want:   false,
		},
		{
			name:        "internal member with permission 0 denied",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 0,
			access: access(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   false,
		},
		{
			name:        "internal member with permission 1 GET resource allowed",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 1,
			access: access(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   true,
		},
		{
			name:        "internal member with permission 1 GET download allowed",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 1,
			access: access(http.MethodGet, func(a *ShareAccess) { a.Download = true }),
			want:   true,
		},
		{
			name:        "permission 1 POST denied",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 1,
			access: access(http.MethodPost, func(a *ShareAccess) { a.Resource = true }),
			want:   false,
		},
		{
			name:        "permission 1 with Upload flag denied even on GET",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 1,
			access: access(http.MethodGet, func(a *ShareAccess) { a.Upload = true }),
			want:   false,
		},
		{
			name:        "internal share with permission 2 (upload-only) is denied",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 2,
			access: access(http.MethodPost, func(a *ShareAccess) { a.Upload = true }),
			want:   false,
		},
		{
			name:        "external share with permission 2 (upload-only) allows upload",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeExternal, permission: 2,
			access: access(http.MethodPost, func(a *ShareAccess) { a.Upload = true }),
			want:   true,
		},
		{
			name:        "external share with permission 2 denies plain GET",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeExternal, permission: 2,
			access: access(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   false,
		},
		{
			name:        "permission 3 allows everything",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 3,
			access: access(http.MethodPost, func(a *ShareAccess) { a.Upload = true }),
			want:   true,
		},
		{
			name:        "permission 3 allows GET resource",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeExternal, permission: 3,
			access: access(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   true,
		},
		{
			name:        "permission 4 admin allows everything",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 4,
			access: access(http.MethodPatch, func(a *ShareAccess) { a.Paste = true }),
			want:   true,
		},
		{
			name:        "unknown permission denied",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 99,
			access: access(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkPermission(tc.currentUser, tc.shareBy, tc.shareType, tc.permission, tc.access)
			if got != tc.want {
				t.Fatalf("checkPermission(currentUser=%q, shareBy=%q, type=%q, perm=%d, access=%+v) = %v, want %v",
					tc.currentUser, tc.shareBy, tc.shareType, tc.permission, tc.access, got, tc.want)
			}
		})
	}
}

// TestCheckNonSharedPath_KnownPaths pins the well-formed routing
// decisions that the share middleware relies on. It deliberately
// avoids loose-matching edge cases (e.g. paths that just *contain*
// "/api/share" as a substring); those are covered separately so that
// future tightening (path-prefix matching) can update only that test.
func TestCheckNonSharedPath_KnownPaths(t *testing.T) {
	mustSkipShare := []string{
		"/api/nodes",
		"/api/task/list",
		"/api/accounts/me",
		"/api/users/alice",
		"/api/share/path",
		"/api/mounted",
		"/api/mount/abc",
		"/api/unmount/abc",
		"/api/smb_history",
		"/api/search?q=foo",
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

// TestCheckNonSharedPath_LooseMatchingDocumented documents the
// current strings.Contains-based behavior of checkNonSharedPath so
// the next PR (which is expected to switch to prefix-based matching)
// has a concrete test to update. Until then, paths that simply
// contain a non-share substring are incorrectly skipped.
func TestCheckNonSharedPath_LooseMatchingDocumented(t *testing.T) {
	loose := []struct {
		path           string
		currentlyShare bool // current behavior; true=goes through share middleware
	}{
		{"/api/sharepoint/file", false},     // contains "/api/share"
		{"/api/users-data", false},          // contains "/api/users"
		{"/api/resources/api/share", false}, // share substring later in path
	}
	for _, c := range loose {
		t.Run(c.path, func(t *testing.T) {
			if got := checkNonSharedPath(c.path); got != c.currentlyShare {
				t.Fatalf("checkNonSharedPath(%q) = %v, want %v (current loose-match behavior)",
					c.path, got, c.currentlyShare)
			}
		})
	}
}

// TestCheckExpired pins the fail-closed contract that every share
// expiry decision rides on: any expiry the parser cannot understand
// must be treated as expired.
func TestCheckExpired(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty string treated as expired", "", true},
		{"garbage treated as expired", "not-a-timestamp", true},
		{"past timestamp expired", now.Add(-time.Hour).UTC().Format(time.RFC3339Nano), true},
		{"future timestamp not expired", now.Add(time.Hour).UTC().Format(time.RFC3339Nano), false},
		{"far-future timestamp not expired", now.Add(24 * time.Hour).UTC().Format(time.RFC3339Nano), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := checkExpired(c.in); got != c.want {
				t.Fatalf("checkExpired(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
