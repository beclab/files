package common

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"sync"

	"k8s.io/klog/v2"
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
//
// If the OS RNG ever fails (effectively impossible on supported
// platforms), the token stays empty. EqualInternalShareToken treats an
// empty expected value as a hard mismatch, so every internal forward
// fails closed until the process is restarted. The previous behavior
// fell back to a constant string, which made the trust boundary
// trivially bypassable any time RNG initialization failed.
func InternalShareToken() string {
	internalTokenOnce.Do(func() {
		var b [32]byte
		if _, err := rand.Read(b[:]); err != nil {
			klog.Errorf("internal share token init failed; internal forwards disabled until restart: %v", err)
			return
		}
		internalToken = hex.EncodeToString(b[:])
	})
	return internalToken
}

// EqualInternalShareToken returns true iff got matches the process
// secret using a constant-time comparison.
//
// An empty got or empty expected (i.e. uninitialized token) always
// returns false; callers cannot accidentally authorize requests by
// sending an empty header against an empty secret.
func EqualInternalShareToken(got string) bool {
	return equalInternalShareToken(got, InternalShareToken())
}

// equalInternalShareToken is the testable core of EqualInternalShareToken.
// It exists so tests can cover the empty-expected and mismatch cases
// without having to fake the package-level token.
func equalInternalShareToken(got, expected string) bool {
	if got == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
