// Package redact provides minimal helpers for masking credential-like
// values in *startup-diagnostic* log lines that must dump a whole table
// (env vars, viper keys). For one-off log call sites that just contain a
// secret, the right answer is "do not log it" rather than calling these
// helpers.
package redact

import "regexp"

// sensitiveKey matches field/env names that look credential-bearing. The
// pattern is case-insensitive and matches anywhere in the name so e.g.
// `MY_API_KEY`, `accessToken`, `dbPassword` all hit.
var sensitiveKey = regexp.MustCompile(`(?i)(password|passwd|secret|token|apikey|api_key|key|cookie|cred)`)

// Key reports whether name looks like a credential identifier.
func Key(name string) bool {
	return sensitiveKey.MatchString(name)
}

// Value returns "***" when key is sensitive and value is non-empty;
// otherwise it returns value unchanged. Empty values pass through so the
// caller can still see "key was unset".
func Value(key, value string) string {
	if value == "" {
		return value
	}
	if Key(key) {
		return "***"
	}
	return value
}
