package common

import "testing"

func TestParseAllowedOrigins(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"only whitespace", "  , , ,", nil},
		{"single bare host", "foo.example.com", []string{"foo.example.com"}},
		{"host with port", "foo.example.com:8443", []string{"foo.example.com:8443"}},
		{"full origin URL", "https://foo.example.com", []string{"foo.example.com"}},
		{"mix", "https://a.example.com, b.example.com:9000 ,c.example.com",
			[]string{"a.example.com", "b.example.com:9000", "c.example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAllowedOrigins(tc.in)
			if !equalStringSlice(got, tc.want) {
				t.Fatalf("parseAllowedOrigins(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestAllowedOrigin_SameHost(t *testing.T) {
	got := AllowedOrigin("https://files.alice.example.com",
		"files.alice.example.com", "")
	if got != "https://files.alice.example.com" {
		t.Fatalf("AllowedOrigin same forwarded host: got %q", got)
	}
}

func TestAllowedOrigin_FallsBackToHost(t *testing.T) {
	got := AllowedOrigin("http://files.local:8080", "", "files.local:8080")
	if got != "http://files.local:8080" {
		t.Fatalf("AllowedOrigin same Host fallback: got %q", got)
	}
}

func TestAllowedOrigin_RejectsCrossOrigin(t *testing.T) {
	got := AllowedOrigin("https://evil.example.com",
		"files.alice.example.com", "")
	if got != "" {
		t.Fatalf("AllowedOrigin cross-origin: got %q, want empty", got)
	}
}

func TestAllowedOrigin_RejectsEmpty(t *testing.T) {
	if got := AllowedOrigin("", "files.alice.example.com", ""); got != "" {
		t.Fatalf("AllowedOrigin empty origin: got %q, want empty", got)
	}
}

func TestAllowedOrigin_RejectsMalformed(t *testing.T) {
	if got := AllowedOrigin("not a url", "files.alice.example.com", ""); got != "" {
		t.Fatalf("AllowedOrigin malformed origin: got %q, want empty", got)
	}
}

func TestAllowedOrigin_RejectsNoTarget(t *testing.T) {
	if got := AllowedOrigin("https://anything.example.com", "", ""); got != "" {
		t.Fatalf("AllowedOrigin missing target host: got %q, want empty", got)
	}
}

func TestAllowedOrigin_AllowsExtraOriginsFromEnv(t *testing.T) {
	t.Setenv(CorsAllowedOriginsEnv,
		"https://dashboard.olares.example.com, foo.example.com:8443")

	cases := []struct {
		name   string
		origin string
		want   string
	}{
		{"extra origin URL form", "https://dashboard.olares.example.com",
			"https://dashboard.olares.example.com"},
		{"extra host:port", "https://foo.example.com:8443",
			"https://foo.example.com:8443"},
		{"extra mismatch port still rejected", "https://foo.example.com:9000", ""},
		{"unknown still rejected", "https://other.example.com", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AllowedOrigin(tc.origin, "files.alice.example.com", "")
			if got != tc.want {
				t.Fatalf("AllowedOrigin(%q) = %q, want %q", tc.origin, got, tc.want)
			}
		})
	}
}

func TestAllowedOrigin_CaseInsensitiveHost(t *testing.T) {
	got := AllowedOrigin("https://FILES.alice.example.com",
		"files.alice.example.com", "")
	if got != "https://FILES.alice.example.com" {
		t.Fatalf("AllowedOrigin case-insensitive same host: got %q", got)
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
