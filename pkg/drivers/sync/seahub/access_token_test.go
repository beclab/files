package seahub

import (
	"strconv"
	"sync"
	"testing"
)

// TestAccessTokenAccessors covers the basic single-goroutine semantics:
// missing key, set+get, overwrite, delete, clear.
func TestAccessTokenAccessors(t *testing.T) {
	t.Cleanup(ClearAccessTokens)

	if v, ok := GetAccessToken("missing"); ok || v != "" {
		t.Fatalf("expected miss, got (%q, %v)", v, ok)
	}

	SetAccessToken("u1", "t1")
	if v, ok := GetAccessToken("u1"); !ok || v != "t1" {
		t.Fatalf("after Set: got (%q, %v), want (\"t1\", true)", v, ok)
	}

	SetAccessToken("u1", "t1b")
	if v, _ := GetAccessToken("u1"); v != "t1b" {
		t.Fatalf("overwrite: got %q, want \"t1b\"", v)
	}

	DeleteAccessToken("u1")
	if _, ok := GetAccessToken("u1"); ok {
		t.Fatalf("expected key gone after Delete")
	}

	SetAccessToken("u2", "t2")
	SetAccessToken("u3", "t3")
	ClearAccessTokens()
	if _, ok := GetAccessToken("u2"); ok {
		t.Fatalf("u2 should be cleared")
	}
	if _, ok := GetAccessToken("u3"); ok {
		t.Fatalf("u3 should be cleared")
	}
}

// TestAccessTokenConcurrent is the race detector's job. We hammer
// Set/Get/Delete/Clear from many goroutines and rely on `go test -race`
// to flag any synchronization regression.
func TestAccessTokenConcurrent(t *testing.T) {
	t.Cleanup(ClearAccessTokens)

	const goroutines = 16
	const iterations = 1000

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := "uid-" + strconv.Itoa(g%4)
				SetAccessToken(key, strconv.Itoa(i))
				_, _ = GetAccessToken(key)
				if i%50 == 0 {
					DeleteAccessToken(key)
				}
				if i%200 == 0 {
					ClearAccessTokens()
				}
			}
		}(g)
	}
	wg.Wait()
}
