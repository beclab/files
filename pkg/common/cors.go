package common

import (
	"net/url"
	"os"
	"strings"
)

// CorsAllowedOriginsEnv is the optional comma-separated list of extra
// CORS-allowed origins, in addition to the request's own host. Each
// entry may be a bare hostname ("foo.example.com"), host:port, or a
// full origin URL ("https://foo.example.com"); the host portion is
// compared case-insensitively.
const CorsAllowedOriginsEnv = "CORS_ALLOWED_ORIGINS"

// AllowedOrigin reports the value to echo back in
// Access-Control-Allow-Origin for a request. An empty result means
// the header should be omitted; the browser then blocks the response
// from JavaScript.
//
// An origin is allowed when:
//
//  1. It is non-empty and parses to a host.
//  2. Its host equals the request's effective host - the
//     X-Forwarded-Host header set by the gateway, falling back to
//     the Host header for direct connections.
//  3. Or its host matches one of the values configured via
//     $CORS_ALLOWED_ORIGINS.
//
// Previously the file service reflected the inbound Origin header
// verbatim while also setting Access-Control-Allow-Credentials: true,
// which let any cross-site page issue credentialed XHR against the
// service. This helper restores the same-origin invariant by default
// and lets operators opt extra origins back in via env.
func AllowedOrigin(originHeader, forwardedHost, host string) string {
	if originHeader == "" {
		return ""
	}
	u, err := url.Parse(originHeader)
	if err != nil || u.Host == "" {
		return ""
	}
	target := forwardedHost
	if target == "" {
		target = host
	}
	if target != "" && strings.EqualFold(u.Host, target) {
		return originHeader
	}
	for _, allowed := range corsExtraAllowedHosts() {
		if strings.EqualFold(u.Host, allowed) {
			return originHeader
		}
	}
	return ""
}

func corsExtraAllowedHosts() []string {
	return parseAllowedOrigins(os.Getenv(CorsAllowedOriginsEnv))
}

// parseAllowedOrigins extracts the comparable host portion from each
// comma-separated entry. Entries are accepted in three forms: bare
// hostname, host:port, or full URL (whose host is taken).
func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if u, err := url.Parse(p); err == nil && u.Host != "" {
			out = append(out, u.Host)
		} else {
			out = append(out, p)
		}
	}
	return out
}
