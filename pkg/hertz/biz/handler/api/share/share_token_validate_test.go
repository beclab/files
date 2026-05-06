package share

import (
	"testing"
	"time"

	share "files/pkg/hertz/biz/model/api/share"
)

// fixedNow is a stable reference time the table tests below use.
// All fixture expiry strings are computed relative to it.
var fixedNow = time.Date(2026, 5, 6, 16, 30, 0, 0, time.UTC)

func mkToken(pathID, expireAt string) *share.ShareToken {
	return &share.ShareToken{
		PathID:   pathID,
		ExpireAt: expireAt,
	}
}

// TestValidateShareTokenForPath covers the four failure modes that
// PR #226 (the share token / path_id IDOR fix) introduced or pinned
// down. Each case must produce the documented (reason, ts) pair.
//
// The IDOR case (PathMismatch) is the load-bearing one: removing
// the PathID equality check would re-open the bug PR #226 fixed.
func TestValidateShareTokenForPath(t *testing.T) {
	const wantPath = "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		name       string
		token      *share.ShareToken
		path       string
		wantReason ShareTokenValidationError
		wantTSNow  bool // true if returned ts must be == fixedNow.Unix()
		wantTSAt   int64
	}{
		{
			name:       "ok: matching path_id and future expiry",
			token:      mkToken(wantPath, fixedNow.Add(time.Hour).Format(time.RFC3339Nano)),
			path:       wantPath,
			wantReason: ShareTokenValidationOK,
			wantTSAt:   0, // OK -> ts unused; helper returns 0
		},
		{
			name:       "nil token",
			token:      nil,
			path:       wantPath,
			wantReason: ShareTokenValidationNilToken,
			wantTSNow:  true,
		},
		{
			// IDOR: token issued for some OTHER path. PR #226's
			// regression test case. If the equality check is
			// dropped from ValidateShareTokenForPath this case
			// will start returning OK and this test fails.
			name:       "path mismatch (IDOR)",
			token:      mkToken("99999999-9999-9999-9999-999999999999", fixedNow.Add(time.Hour).Format(time.RFC3339Nano)),
			path:       wantPath,
			wantReason: ShareTokenValidationPathMismatch,
			wantTSNow:  true,
		},
		{
			name:       "malformed expire_at",
			token:      mkToken(wantPath, "not-a-timestamp"),
			path:       wantPath,
			wantReason: ShareTokenValidationBadExpire,
			wantTSNow:  true,
		},
		{
			name:       "expired (1 hour ago)",
			token:      mkToken(wantPath, fixedNow.Add(-time.Hour).Format(time.RFC3339Nano)),
			path:       wantPath,
			wantReason: ShareTokenValidationExpired,
			wantTSAt:   fixedNow.Add(-time.Hour).Unix(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotReason, gotTS := ValidateShareTokenForPath(tc.token, tc.path, fixedNow)
			if gotReason != tc.wantReason {
				t.Errorf("reason = %v, want %v", gotReason, tc.wantReason)
			}
			if tc.wantTSNow {
				if gotTS != fixedNow.Unix() {
					t.Errorf("ts = %d, want fixedNow.Unix() = %d", gotTS, fixedNow.Unix())
				}
			} else if gotTS != tc.wantTSAt {
				t.Errorf("ts = %d, want %d", gotTS, tc.wantTSAt)
			}
		})
	}
}

// TestValidateShareTokenForPath_NoYearOneLeak pins the explicit
// guard against returning the year-1 Unix epoch (-62135596800) when
// the expire string is malformed. The PR #226 fix and the
// ParseRFC3339Nano helper both work toward "do not leak a year-1
// timestamp into API responses"; this is a regression watch.
func TestValidateShareTokenForPath_NoYearOneLeak(t *testing.T) {
	tok := mkToken("path-x", "garbage")
	_, ts := ValidateShareTokenForPath(tok, "path-x", fixedNow)
	if ts < 0 {
		t.Fatalf("returned ts = %d (negative); year-1 zero-time leak detected", ts)
	}
	if ts != fixedNow.Unix() {
		t.Fatalf("returned ts = %d, want fixedNow.Unix() = %d (must coerce to a sane value)", ts, fixedNow.Unix())
	}
}
