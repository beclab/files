package common

import (
	"testing"
	"time"
)

// TestParseRFC3339Nano_Valid covers the happy path: a well-formed
// RFC 3339 nano timestamp must round-trip through ParseRFC3339Nano
// without losing precision.
func TestParseRFC3339Nano_Valid(t *testing.T) {
	want := time.Date(2026, 5, 6, 16, 30, 45, 123456789, time.UTC)
	in := want.Format(time.RFC3339Nano)

	got, ok := ParseRFC3339Nano(in)
	if !ok {
		t.Fatalf("ParseRFC3339Nano(%q) ok = false, want true", in)
	}
	if !got.Equal(want) {
		t.Fatalf("ParseRFC3339Nano(%q) = %v, want %v", in, got, want)
	}
}

// TestParseRFC3339Nano_Invalid covers the failure path that the
// helper exists to make visible. Each input is a kind of malformed
// timestamp that the original `time.Parse(time.RFC3339Nano, _)`
// pattern would have silently coerced to time.Time{}.
//
// We require: ok == false and the returned time is the zero value.
// (The function also klog.Errorf's, but we don't assert on that
// because klog routes to its own sinks and the project's redact
// package owns that contract.)
func TestParseRFC3339Nano_Invalid(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"garbage", "not-a-time"},
		{"missing zone", "2026-05-06T16:30:45.123456789"},
		{"date only", "2026-05-06"},
		{"wrong separator", "2026/05/06 16:30:45Z"},
		{"unix epoch number", "1762432245"},
		{"nano without separator", "2026-05-06T16:30:45.123456789Z!"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseRFC3339Nano(tc.in)
			if ok {
				t.Errorf("ParseRFC3339Nano(%q) ok = true, want false (got %v)", tc.in, got)
			}
			if !got.IsZero() {
				t.Errorf("ParseRFC3339Nano(%q) returned %v, want zero time", tc.in, got)
			}
		})
	}
}

// TestParseRFC3339Nano_FailClosedExpiry encodes the documented
// "fail closed" property: callers compare the returned time against
// time.Now() to decide whether something is expired. A zero time is
// always before now, so After() reports "expired" - that is the
// safe direction for share/token validation paths and we want a
// regression test that pins this behavior.
func TestParseRFC3339Nano_FailClosedExpiry(t *testing.T) {
	zero, ok := ParseRFC3339Nano("garbage")
	if ok {
		t.Fatalf("ParseRFC3339Nano(garbage) ok = true, want false")
	}
	if !time.Now().After(zero) {
		t.Fatalf("time.Now().After(zeroTime) = false, expected true (fail-closed expiry semantics broken)")
	}
}
