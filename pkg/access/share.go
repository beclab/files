package access

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"files/pkg/common"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/hertz/biz/model/api/share"

	"k8s.io/klog/v2"
)

// ShareAccess describes the operation a share request performs. It is
// the canonical form the share permission matrix is evaluated against.
type ShareAccess struct {
	Method    string
	Resource  bool
	Preview   bool
	Raw       bool
	Download  bool
	Paste     bool
	Upload    bool
	FromShare bool
}

// SharePermitted reports whether a share grant of the given permission
// level allows the requested operation. The matrix mirrors the share
// permission semantics:
//
//	0 - no permit
//	1 - view, download
//	2 - upload only (external only)
//	3 - upload, download
//	4 - admin
func SharePermitted(currentUser, shareBy, shareType string, permission int32, a *ShareAccess) bool {
	if shareType == common.ShareTypeInternal && currentUser == shareBy {
		return true
	}

	switch permission {
	case 1:
		return a.Method == http.MethodGet && !a.Upload
	case 2:
		// Upload-only, external links only. The original matrix wrote
		// this as `a.Upload || (GET && a.Upload)`, which collapses to a.Upload.
		return shareType == common.ShareTypeExternal && a.Upload
	case 3:
		return true
	case 4:
		return true
	default:
		return false
	}
}

// Sentinel errors returned by ShareCheckPaste so callers can map them to
// source/destination-specific, user-facing messages.
var (
	ErrShareNotFound = errors.New("share not found")
	ErrShareExpired  = errors.New("share expired")
	ErrShareDenied   = errors.New("share permission denied")
)

// shareExpired reports whether the stored RFC3339Nano expireTime is in the
// past or unparseable. expiresUnix is the unix expiry to surface (now() when
// unparseable, so callers can still report a token/link-expired error).
func shareExpired(expireTime string) (expiresUnix int64, expired bool) {
	t, ok := common.ParseRFC3339Nano(expireTime)
	if !ok {
		return time.Now().Unix(), true
	}
	if time.Now().After(t) {
		return t.Unix(), true
	}
	return 0, false
}

// ShareCheckPaste resolves shareID for a paste operation performed by
// owner, enforcing expiry and the paste-specific member threshold: a
// paste source needs view access (permission >= 1) and a destination
// needs write/upload access (permission >= 2). These thresholds
// intentionally differ from SharePermitted's HTTP-method matrix - a
// view-only member may still be a paste source. The share owner bypasses
// the member check. Expiry is enforced for everyone (matching the prior
// inline behavior). Callers map the sentinel errors to the right message.
func ShareCheckPaste(owner, shareID string, write bool) (*share.SharePath, error) {
	shared, err := database.GetSharePath(shareID)
	if err != nil {
		return nil, fmt.Errorf("query share: %w", err)
	}
	if shared == nil {
		return nil, ErrShareNotFound
	}

	if _, expired := shareExpired(shared.ExpireTime); expired {
		return nil, ErrShareExpired
	}

	if owner == shared.Owner {
		return shared, nil
	}

	member, err := database.GetShareMember(shared.ID, owner)
	if err != nil {
		return nil, fmt.Errorf("query share member: %w", err)
	}
	if member == nil || member.ShareMember == "" {
		return nil, ErrShareNotFound
	}

	minPerm := int32(1)
	if write {
		minPerm = 2
	}
	if member.Permission < minPerm {
		return nil, ErrShareDenied
	}
	return shared, nil
}

// ShareResolvePath loads the share record by id and enforces expiry.
// The returned int64 is a unix timestamp used to surface link-expired
// errors to the caller.
func ShareResolvePath(currentUser, shareId string, fromShare bool) (*share.SharePath, int64, error) {
	sharePath, err := database.GetSharePath(shareId)
	if err != nil {
		klog.Errorf("GetSharePath error: %v", err)
		return nil, 0, errors.New(common.ErrorMessageWrongShare)
	}
	if sharePath == nil {
		klog.Errorf("sharePath not found, shareId: %s", shareId)
		return nil, 0, errors.New(common.ErrorMessageWrongShare)
	}

	if !fromShare && currentUser == sharePath.Owner {
		return sharePath, 0, nil
	}

	if exp, expired := shareExpired(sharePath.ExpireTime); expired {
		klog.Errorf("sharePath expired, expireTime: %s", sharePath.ExpireTime)
		return nil, exp, errors.New(common.ErrorMessageLinkExpired)
	}
	return sharePath, 0, nil
}

// ShareCheckInternal validates an internal share member and returns it
// when the requested operation is permitted.
func ShareCheckInternal(currentOwner string, sharePaths *share.SharePath, a *ShareAccess) (*share.ShareMember, error) {
	shareMember, err := database.GetShareMember(sharePaths.ID, currentOwner)
	if err != nil {
		return nil, fmt.Errorf("GetShareMember error: %v", err)
	}
	if shareMember == nil || shareMember.ShareMember == "" {
		return nil, errors.New("shareMember not found")
	}
	if !SharePermitted(currentOwner, sharePaths.Owner, sharePaths.ShareType, shareMember.Permission, a) {
		return nil, errors.New("authorization check failed")
	}
	return shareMember, nil
}

// ShareCheckExternal validates an external share token and reports
// whether the requested operation is permitted. The int64 is a unix
// timestamp used to surface token-expired errors.
func ShareCheckExternal(currentUser, token string, sharePaths *share.SharePath, a *ShareAccess) (int64, bool, error) {
	if !a.FromShare && currentUser == sharePaths.Owner {
		return 0, true, nil
	}
	var defaultExpired = time.Now().Unix()
	token = strings.TrimSpace(token)
	if token == "" {
		return defaultExpired, false, errors.New("token is nil")
	}

	shareToken, err := database.QueryShareExternalById(sharePaths.ID, token)
	if err != nil {
		return defaultExpired, false, fmt.Errorf("QueryShareExternalById error: %v", err)
	}
	if shareToken == nil {
		return defaultExpired, false, errors.New("shareToken not found")
	}

	expired, ok := common.ParseRFC3339Nano(shareToken.ExpireAt)
	if !ok {
		return time.Now().Unix(), false, fmt.Errorf("shareToken expireAt unparseable: %q", shareToken.ExpireAt)
	}
	if time.Now().After(expired) {
		klog.Errorf("[share] shareToken expired, expireAt: %s", shareToken.ExpireAt)
		return expired.Unix(), false, fmt.Errorf("shareToken expired, expireAt: %s", shareToken.ExpireAt)
	}

	return 0, SharePermitted(currentUser, sharePaths.Owner, sharePaths.ShareType, sharePaths.Permission, a), nil
}

// ShareAuthorize runs the internal/external dispatch for an already-resolved
// share. It returns the matched member (internal members only; nil for the
// owner short-circuit and for external shares). On failure err is non-nil;
// expiredUnix > 0 marks an external token expiry the caller may surface as
// CodeTokenExpired (plain denials return expiredUnix == 0). An unknown share
// type fails closed (nil, 0, ErrShareDenied).
func ShareAuthorize(owner, token string, shared *share.SharePath, a *ShareAccess) (*share.ShareMember, int64, error) {
	switch strings.ToLower(shared.ShareType) {
	case common.ShareTypeInternal:
		if owner == shared.Owner {
			return nil, 0, nil
		}
		member, err := ShareCheckInternal(owner, shared, a)
		if err != nil {
			return nil, 0, err
		}
		return member, 0, nil
	case common.ShareTypeExternal:
		expires, permit, err := ShareCheckExternal(owner, token, shared, a)
		if err != nil {
			return nil, expires, err
		}
		if !permit {
			return nil, 0, ErrShareDenied
		}
		return nil, 0, nil
	default:
		// Fail closed: the only share types these call sites authorize are
		// internal/external. An unrecognized type (corrupt row, or an smb
		// share that should never reach this HTTP proxy) is denied rather
		// than silently passed through.
		return nil, 0, ErrShareDenied
	}
}
