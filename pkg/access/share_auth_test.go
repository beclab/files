package access

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"files/pkg/common"
	"files/pkg/hertz/biz/model/api/share"
)

func TestShareResolvePath(t *testing.T) {
	newShareTestDB(t)

	seedSharePath(t, "valid", "alice", common.ShareTypeInternal, futureRFC3339(time.Hour), 1)
	seedSharePath(t, "expired", "alice", common.ShareTypeInternal, futureRFC3339(-time.Hour), 1)
	seedSharePath(t, "bad-time", "alice", common.ShareTypeInternal, "not-a-timestamp", 1)

	tests := []struct {
		name        string
		currentUser string
		shareId     string
		fromShare   bool
		wantErr     string
		wantExpires bool // expires > 0
		wantPath    bool
	}{
		{name: "missing", currentUser: "alice", shareId: "nope", wantErr: common.ErrorMessageWrongShare},
		{name: "owner not from share", currentUser: "alice", shareId: "expired", fromShare: false, wantPath: true},
		{name: "owner from share still expires", currentUser: "alice", shareId: "expired", fromShare: true, wantErr: common.ErrorMessageLinkExpired, wantExpires: true},
		{name: "expired", currentUser: "bob", shareId: "expired", wantErr: common.ErrorMessageLinkExpired, wantExpires: true},
		{name: "member from share expired (upload path)", currentUser: "bob", shareId: "expired", fromShare: true, wantErr: common.ErrorMessageLinkExpired, wantExpires: true},
		{name: "unparseable", currentUser: "bob", shareId: "bad-time", wantErr: common.ErrorMessageLinkExpired, wantExpires: true},
		{name: "valid", currentUser: "bob", shareId: "valid", wantPath: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, expires, err := ShareResolvePath(tc.currentUser, tc.shareId, tc.fromShare)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.wantExpires && expires <= 0 {
				t.Fatalf("expires = %d, want > 0", expires)
			}
			if !tc.wantExpires && expires != 0 {
				t.Fatalf("expires = %d, want 0", expires)
			}
			if tc.wantPath && path == nil {
				t.Fatalf("path = nil, want non-nil")
			}
			if !tc.wantPath && path != nil {
				t.Fatalf("path = %+v, want nil", path)
			}
		})
	}
}

func TestShareCheckPaste(t *testing.T) {
	newShareTestDB(t)

	seedSharePath(t, "valid", "alice", common.ShareTypeInternal, futureRFC3339(time.Hour), 1)
	seedSharePath(t, "expired", "alice", common.ShareTypeInternal, futureRFC3339(-time.Hour), 1)
	seedShareMember(t, "valid", "viewer", 1)
	seedShareMember(t, "valid", "writer", 2)

	tests := []struct {
		name    string
		owner   string
		shareID string
		write   bool
		wantErr error
		wantOK  bool
	}{
		{name: "missing", owner: "x", shareID: "nope", wantErr: ErrShareNotFound},
		{name: "expired", owner: "x", shareID: "expired", wantErr: ErrShareExpired},
		{name: "owner bypass", owner: "alice", shareID: "valid", wantOK: true},
		{name: "member absent", owner: "ghost", shareID: "valid", wantErr: ErrShareNotFound},
		{name: "read viewer ok", owner: "viewer", shareID: "valid", write: false, wantOK: true},
		{name: "write viewer denied", owner: "viewer", shareID: "valid", write: true, wantErr: ErrShareDenied},
		{name: "write writer ok", owner: "writer", shareID: "valid", write: true, wantOK: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shared, err := ShareCheckPaste(tc.owner, tc.shareID, tc.write)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.wantOK && shared == nil {
				t.Fatalf("shared = nil, want non-nil")
			}
		})
	}
}

func TestShareCheckInternal(t *testing.T) {
	newShareTestDB(t)
	seedShareMember(t, "p1", "viewer", 1)

	shared := &share.SharePath{ID: "p1", Owner: "alice", ShareType: common.ShareTypeInternal}

	t.Run("member absent", func(t *testing.T) {
		_, err := ShareCheckInternal("ghost", shared, &ShareAccess{Method: http.MethodGet, Resource: true})
		if err == nil {
			t.Fatalf("want error for absent member")
		}
	})

	t.Run("perm1 GET ok", func(t *testing.T) {
		member, err := ShareCheckInternal("viewer", shared, &ShareAccess{Method: http.MethodGet, Resource: true})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if member == nil {
			t.Fatalf("member = nil, want non-nil")
		}
	})

	t.Run("perm1 POST denied", func(t *testing.T) {
		_, err := ShareCheckInternal("viewer", shared, &ShareAccess{Method: http.MethodPost, Upload: true})
		if err == nil {
			t.Fatalf("want authorization error for POST on view-only member")
		}
	})
}

func TestShareCheckExternal(t *testing.T) {
	newShareTestDB(t)
	seedShareToken(t, "p1", "good", futureRFC3339(time.Hour))
	seedShareToken(t, "p1", "stale", futureRFC3339(-time.Hour))

	read := &ShareAccess{Method: http.MethodGet, Resource: true, FromShare: true}

	t.Run("owner short-circuit", func(t *testing.T) {
		shared := &share.SharePath{ID: "p1", Owner: "alice", ShareType: common.ShareTypeExternal, Permission: 1}
		expires, permit, err := ShareCheckExternal("alice", "", shared, &ShareAccess{Method: http.MethodGet, FromShare: false})
		if err != nil || !permit || expires != 0 {
			t.Fatalf("owner short-circuit: expires=%d permit=%v err=%v", expires, permit, err)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		shared := &share.SharePath{ID: "p1", Owner: "alice", ShareType: common.ShareTypeExternal, Permission: 1}
		expires, permit, err := ShareCheckExternal("bob", "  ", shared, read)
		if err == nil || permit || expires <= 0 {
			t.Fatalf("empty token: expires=%d permit=%v err=%v", expires, permit, err)
		}
	})

	t.Run("not found token", func(t *testing.T) {
		shared := &share.SharePath{ID: "p1", Owner: "alice", ShareType: common.ShareTypeExternal, Permission: 1}
		expires, permit, err := ShareCheckExternal("bob", "missing", shared, read)
		if err == nil || permit || expires <= 0 {
			t.Fatalf("not found: expires=%d permit=%v err=%v", expires, permit, err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		shared := &share.SharePath{ID: "p1", Owner: "alice", ShareType: common.ShareTypeExternal, Permission: 1}
		expires, permit, err := ShareCheckExternal("bob", "stale", shared, read)
		if err == nil || permit || expires <= 0 {
			t.Fatalf("expired token: expires=%d permit=%v err=%v", expires, permit, err)
		}
	})

	t.Run("valid permitted", func(t *testing.T) {
		shared := &share.SharePath{ID: "p1", Owner: "alice", ShareType: common.ShareTypeExternal, Permission: 1}
		expires, permit, err := ShareCheckExternal("bob", "good", shared, read)
		if err != nil || !permit || expires != 0 {
			t.Fatalf("valid permitted: expires=%d permit=%v err=%v", expires, permit, err)
		}
	})

	t.Run("valid denied", func(t *testing.T) {
		shared := &share.SharePath{ID: "p1", Owner: "alice", ShareType: common.ShareTypeExternal, Permission: 1}
		// permission 1 does not allow upload, so an upload op is denied (err nil).
		expires, permit, err := ShareCheckExternal("bob", "good", shared, &ShareAccess{Method: http.MethodPost, Upload: true, FromShare: true})
		if err != nil || permit || expires != 0 {
			t.Fatalf("valid denied: expires=%d permit=%v err=%v", expires, permit, err)
		}
	})
}

func TestShareAuthorize(t *testing.T) {
	newShareTestDB(t)
	seedShareMember(t, "internal", "viewer", 1)
	seedShareToken(t, "external", "good", futureRFC3339(time.Hour))
	seedShareToken(t, "external", "stale", futureRFC3339(-time.Hour))

	internal := &share.SharePath{ID: "internal", Owner: "alice", ShareType: common.ShareTypeInternal, Permission: 1}
	external := &share.SharePath{ID: "external", Owner: "alice", ShareType: common.ShareTypeExternal, Permission: 1}
	read := &ShareAccess{Method: http.MethodGet, Resource: true, FromShare: true}
	upload := &ShareAccess{Method: http.MethodPost, Upload: true, FromShare: true}

	t.Run("internal owner bypass", func(t *testing.T) {
		member, expires, err := ShareAuthorize("alice", "", internal, read)
		if err != nil || expires != 0 || member != nil {
			t.Fatalf("owner bypass: member=%v expires=%d err=%v", member, expires, err)
		}
	})

	t.Run("internal member allow", func(t *testing.T) {
		member, expires, err := ShareAuthorize("viewer", "", internal, read)
		if err != nil || expires != 0 || member == nil {
			t.Fatalf("member allow: member=%v expires=%d err=%v", member, expires, err)
		}
	})

	t.Run("internal denied", func(t *testing.T) {
		member, expires, err := ShareAuthorize("viewer", "", internal, upload)
		if err == nil || expires != 0 || member != nil {
			t.Fatalf("internal denied: member=%v expires=%d err=%v", member, expires, err)
		}
	})

	t.Run("internal member absent", func(t *testing.T) {
		_, _, err := ShareAuthorize("ghost", "", internal, read)
		if err == nil {
			t.Fatalf("want error for absent member")
		}
	})

	t.Run("external allow", func(t *testing.T) {
		member, expires, err := ShareAuthorize("bob", "good", external, read)
		if err != nil || expires != 0 || member != nil {
			t.Fatalf("external allow: member=%v expires=%d err=%v", member, expires, err)
		}
	})

	t.Run("external denied", func(t *testing.T) {
		_, expires, err := ShareAuthorize("bob", "good", external, upload)
		if !errors.Is(err, ErrShareDenied) || expires != 0 {
			t.Fatalf("external denied: expires=%d err=%v", expires, err)
		}
	})

	t.Run("external expired", func(t *testing.T) {
		_, expires, err := ShareAuthorize("bob", "stale", external, read)
		if err == nil || expires <= 0 {
			t.Fatalf("external expired: expires=%d err=%v", expires, err)
		}
	})

	t.Run("unknown share type fails closed", func(t *testing.T) {
		other := &share.SharePath{ID: "x", Owner: "alice", ShareType: "weird"}
		member, expires, err := ShareAuthorize("bob", "", other, read)
		if !errors.Is(err, ErrShareDenied) || expires != 0 || member != nil {
			t.Fatalf("unknown type: member=%v expires=%d err=%v", member, expires, err)
		}
	})
}
