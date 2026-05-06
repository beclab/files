package common

import (
	"time"

	"k8s.io/klog/v2"
)

// ParseRFC3339Nano parses an RFC 3339 nano timestamp string.
//
// On success it returns (parsed, true).
//
// On failure it returns (time.Time{}, false) **and** logs the offending
// string via klog.Errorf so the parse failure is visible in operations.
// The previous pattern across share / driver code was:
//
//	expired, _ := time.Parse(time.RFC3339Nano, s)
//
// which silently coerces a malformed expire/last-modify string to the
// zero year. For expiry checks that happens to "fail closed"
// (`time.Now().After(time.Time{})` is true → treated as expired), which
// is safe in spirit but invisible to operators and produces nonsensical
// negative Unix timestamps in API responses (Unix epoch of year-1 is
// -62135596800). Callers should switch to:
//
//	t, ok := common.ParseRFC3339Nano(s)
//	if !ok { ...handle (typically: treat as expired, return time.Now()) }
//
// so the failure is both logged and explicitly handled.
func ParseRFC3339Nano(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		klog.Errorf("ParseRFC3339Nano: invalid timestamp %q: %v", s, err)
		return time.Time{}, false
	}
	return t, true
}
