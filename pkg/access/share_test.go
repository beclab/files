package access

import (
	"files/pkg/common"
	"net/http"
	"testing"
)

// TestSharePermitted_Matrix pins the share permission matrix that every
// share-API request runs through. The matrix encodes the trust boundary
// between owners, internal members, and external link recipients;
// future refactors must not silently shift any of these decisions.
//
// Permission values:
//
//	0 - no permit
//	1 - view, download
//	2 - upload only (external links only)
//	3 - upload + download
//	4 - admin
func TestSharePermitted_Matrix(t *testing.T) {
	const (
		owner = "alice"
		other = "bob"
	)

	mk := func(method string, mutate func(a *ShareAccess)) *ShareAccess {
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
			access: mk(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   true,
		},
		{
			name:        "internal share owner allowed even for upload",
			currentUser: owner, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 0,
			access: mk(http.MethodPost, func(a *ShareAccess) { a.Upload = true }),
			want:   true,
		},
		{
			name:        "external owner short-circuit does NOT apply (handled by caller)",
			currentUser: owner, shareBy: owner,
			shareType: common.ShareTypeExternal, permission: 0,
			access: mk(http.MethodGet, nil),
			want:   false,
		},
		{
			name:        "internal member with permission 0 denied",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 0,
			access: mk(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   false,
		},
		{
			name:        "internal member with permission 1 GET resource allowed",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 1,
			access: mk(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   true,
		},
		{
			name:        "internal member with permission 1 GET download allowed",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 1,
			access: mk(http.MethodGet, func(a *ShareAccess) { a.Download = true }),
			want:   true,
		},
		{
			name:        "permission 1 POST denied",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 1,
			access: mk(http.MethodPost, func(a *ShareAccess) { a.Resource = true }),
			want:   false,
		},
		{
			name:        "permission 1 with Upload flag denied even on GET",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 1,
			access: mk(http.MethodGet, func(a *ShareAccess) { a.Upload = true }),
			want:   false,
		},
		{
			name:        "internal share with permission 2 (upload-only) is denied",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 2,
			access: mk(http.MethodPost, func(a *ShareAccess) { a.Upload = true }),
			want:   false,
		},
		{
			name:        "external share with permission 2 (upload-only) allows upload",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeExternal, permission: 2,
			access: mk(http.MethodPost, func(a *ShareAccess) { a.Upload = true }),
			want:   true,
		},
		{
			name:        "external share with permission 2 denies plain GET",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeExternal, permission: 2,
			access: mk(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   false,
		},
		{
			name:        "permission 3 allows everything",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 3,
			access: mk(http.MethodPost, func(a *ShareAccess) { a.Upload = true }),
			want:   true,
		},
		{
			name:        "permission 3 allows GET resource",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeExternal, permission: 3,
			access: mk(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   true,
		},
		{
			name:        "permission 4 admin allows everything",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 4,
			access: mk(http.MethodPatch, func(a *ShareAccess) { a.Paste = true }),
			want:   true,
		},
		{
			name:        "unknown permission denied",
			currentUser: other, shareBy: owner,
			shareType: common.ShareTypeInternal, permission: 99,
			access: mk(http.MethodGet, func(a *ShareAccess) { a.Resource = true }),
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SharePermitted(tc.currentUser, tc.shareBy, tc.shareType, tc.permission, tc.access)
			if got != tc.want {
				t.Fatalf("SharePermitted(currentUser=%q, shareBy=%q, type=%q, perm=%d, access=%+v) = %v, want %v",
					tc.currentUser, tc.shareBy, tc.shareType, tc.permission, tc.access, got, tc.want)
			}
		})
	}
}
