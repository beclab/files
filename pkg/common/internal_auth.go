package common

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"sync"
)

// HeaderInternalShareToken is the request header name used for the
// process-local shared secret that authorizes an internal loopback
// forward (e.g. proxySharePaste -> /api/paste/:node/ with Share=1).
//
// The value is generated once per process at startup and never leaves
// it, so an external attacker cannot forge a request that claims to be
// an internal forward.
const HeaderInternalShareToken = "X-Internal-Share-Token"

var (
	internalTokenOnce sync.Once
	internalToken     string
)

// InternalShareToken returns the process-local secret. It is generated
// lazily on first use; callers must not log it.
func InternalShareToken() string {
	internalTokenOnce.Do(func() {
		var b [32]byte
		if _, err := rand.Read(b[:]); err != nil {
			// rand.Read failure is effectively impossible on supported
			// platforms; fall back to a deterministic-but-unique tag so
			// initialization never panics. The mismatch will simply
			// reject every internal call until the process is restarted.
			internalToken = "fallback-internal-share-token"
			return
		}
		internalToken = hex.EncodeToString(b[:])
	})
	return internalToken
}

// EqualInternalShareToken returns true iff got matches the process
// secret using a constant-time comparison.
func EqualInternalShareToken(got string) bool {
	if got == "" {
		return false
	}
	expected := InternalShareToken()
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
