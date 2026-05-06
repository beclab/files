package share

import (
	"time"

	"files/pkg/common"

	share "files/pkg/hertz/biz/model/api/share"
)

// ShareTokenValidationError categorises the reason a share token
// failed validation. The handler maps each reason to a specific
// HTTP error envelope (token-expired vs link-expired) and to a
// timestamp the client can use to display a "your link expired at
// X" message.
type ShareTokenValidationError int

const (
	// ShareTokenValidationOK means the token is valid for the
	// requested path and not yet expired.
	ShareTokenValidationOK ShareTokenValidationError = iota
	// ShareTokenValidationNilToken means the token was nil (DB
	// returned no row).
	ShareTokenValidationNilToken
	// ShareTokenValidationPathMismatch means the token is valid
	// but was issued for a different path_id than the one the
	// caller is asking about. This is the IDOR case PR #226 closed:
	// without this rejection any valid token could be paired with
	// an arbitrary path_id to read that share's metadata.
	ShareTokenValidationPathMismatch
	// ShareTokenValidationBadExpire means the token's expire_at
	// string is malformed and cannot be parsed.
	ShareTokenValidationBadExpire
	// ShareTokenValidationExpired means the token expired before
	// the validation moment.
	ShareTokenValidationExpired
)

// ValidateShareTokenForPath enforces the contract documented above:
// the token must be non-nil, must be issued for requestedPathID,
// must have a parseable expire_at, and must not have expired.
//
// On any failure it returns:
//   - the validation reason (use to pick the HTTP error envelope),
//   - a unix timestamp the handler should report to the client. For
//     "expired" cases this is the actual expiry time so the UI can
//     say when the link died; for the other cases it is "now" (the
//     handler must not leak a year-1 zero-Unix value back to the
//     client when the timestamp is unparseable).
//
// On success it returns (ShareTokenValidationOK, 0).
//
// This is split out from the handler so the IDOR + expiry logic can
// be unit-tested without spinning up Postgres or stubbing the DAL.
// The handler refactor that replaces the inline checks with a call
// to this function is intentionally a follow-up PR.
func ValidateShareTokenForPath(token *share.ShareToken, requestedPathID string, now time.Time) (ShareTokenValidationError, int64) {
	if token == nil {
		return ShareTokenValidationNilToken, now.Unix()
	}
	if token.PathID != requestedPathID {
		return ShareTokenValidationPathMismatch, now.Unix()
	}
	expired, ok := common.ParseRFC3339Nano(token.ExpireAt)
	if !ok {
		return ShareTokenValidationBadExpire, now.Unix()
	}
	if now.After(expired) {
		return ShareTokenValidationExpired, expired.Unix()
	}
	return ShareTokenValidationOK, 0
}
