package common

import "testing"

func TestEqualInternalShareTokenCore(t *testing.T) {
	cases := []struct {
		name          string
		got, expected string
		want          bool
	}{
		{"both empty rejected", "", "", false},
		{"empty expected rejected", "anything", "", false},
		{"empty got rejected", "", "secret", false},
		{"mismatch rejected", "wrong", "secret", false},
		{"different length rejected", "secret-extra", "secret", false},
		{"exact match accepted", "secret", "secret", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := equalInternalShareToken(c.got, c.expected); got != c.want {
				t.Fatalf("equalInternalShareToken(%q,%q) = %v, want %v", c.got, c.expected, got, c.want)
			}
		})
	}
}

func TestInternalShareTokenIsHexAndStable(t *testing.T) {
	first := InternalShareToken()
	if first == "" {
		t.Fatal("InternalShareToken returned empty value (rand init failed?)")
	}
	if first == "fallback-internal-share-token" {
		t.Fatal("InternalShareToken returned the previously-known constant fallback")
	}
	const wantLen = 32 * 2
	if len(first) != wantLen {
		t.Fatalf("InternalShareToken length = %d, want %d", len(first), wantLen)
	}
	for i, r := range first {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("InternalShareToken[%d] = %q, want lowercase hex", i, r)
		}
	}
	if second := InternalShareToken(); second != first {
		t.Fatalf("InternalShareToken not stable: first=%q second=%q", first, second)
	}
}

func TestEqualInternalShareTokenAgainstProcessSecret(t *testing.T) {
	if !EqualInternalShareToken(InternalShareToken()) {
		t.Fatal("EqualInternalShareToken rejected its own token")
	}
	if EqualInternalShareToken("") {
		t.Fatal("EqualInternalShareToken accepted empty header")
	}
	if EqualInternalShareToken("not-the-token") {
		t.Fatal("EqualInternalShareToken accepted an unrelated value")
	}
	if EqualInternalShareToken("fallback-internal-share-token") {
		t.Fatal("EqualInternalShareToken accepted the previously-known constant fallback")
	}
}
