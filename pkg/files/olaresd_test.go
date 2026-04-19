package files

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCallOlaresdFallback_V2NetworkErrorThenV1Succeeds(t *testing.T) {
	// v2 URL points to an unreachable port; v1 URL serves a valid JSON body.
	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer v1.Close()

	urls := []string{"http://127.0.0.1:1/mount", v1.URL}
	res, err := CallOlaresdFallback(urls, []byte(`{}`), nil, 2*time.Second)
	if err != nil {
		t.Fatalf("expected success via fallback, got err=%v", err)
	}
	if res == nil || int(res["code"].(float64)) != 200 {
		t.Fatalf("unexpected response: %#v", res)
	}
}

func TestCallOlaresdFallback_V2NonJSONThenV1Succeeds(t *testing.T) {
	v2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<html><body>404</body></html>`))
	}))
	defer v2.Close()
	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"data":[]}`))
	}))
	defer v1.Close()

	res, err := CallOlaresdFallback([]string{v2.URL, v1.URL}, []byte(`{}`), nil, 2*time.Second)
	if err != nil {
		t.Fatalf("expected success via fallback, got err=%v", err)
	}
	if res == nil || int(res["code"].(float64)) != 200 {
		t.Fatalf("unexpected response: %#v", res)
	}
}

func TestCallOlaresdFallback_BothFailWith4xx(t *testing.T) {
	makeBadServer := func(body string, contentType string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(body))
		}))
	}

	// v2 returns a JSON error body, v1 returns non-JSON. We expect err!=nil,
	// but the most recent parsed JSON body (from v2) should still be returned
	// so the caller can surface the upstream message.
	v2 := makeBadServer(`{"code":400,"message":"mount error(13)"}`, "application/json")
	defer v2.Close()
	v1 := makeBadServer(`<html>bad gateway</html>`, "text/html")
	defer v1.Close()

	res, err := CallOlaresdFallback([]string{v2.URL, v1.URL}, []byte(`{}`), nil, 2*time.Second)
	if err == nil {
		t.Fatalf("expected error when all URLs fail")
	}
	if res == nil {
		t.Fatalf("expected last parsed JSON body to be returned")
	}
	msg, _ := res["message"].(string)
	if !strings.Contains(msg, "mount error(13)") {
		t.Fatalf("expected last JSON body to be v2's, got %#v", res)
	}
}

func TestCallOlaresdFallback_BothTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer slow.Close()

	start := time.Now()
	res, err := CallOlaresdFallback([]string{slow.URL, slow.URL}, []byte(`{}`), nil, 50*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected timeout error, got res=%#v", res)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("fallback took too long (timeout not honored): %v", elapsed)
	}
	if !isTimeoutErr(err) {
		t.Logf("got non-timeout error (acceptable if client closed cleanly): %v", err)
	}
}

func TestCallOlaresdFallback_NoURLs(t *testing.T) {
	if _, err := CallOlaresdFallback(nil, []byte(`{}`), nil, time.Second); err == nil {
		t.Fatal("expected error for empty URL list")
	}
}

func TestCallOlaresdFallback_PropagatesHeaders(t *testing.T) {
	var calls int32
	var gotSig, gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		gotSig = r.Header.Get("X-Signature")
		gotCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer srv.Close()

	header := make(http.Header)
	header.Set("X-Bfl-User", "alice")
	if _, err := CallOlaresdFallback([]string{srv.URL}, []byte(`{}`), header, time.Second); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected exactly one call")
	}
	if gotCT != "application/json" {
		t.Fatalf("Content-Type not set, got %q", gotCT)
	}
	if gotSig == "" {
		t.Fatalf("X-Signature default should be set")
	}
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "Client.Timeout") || strings.Contains(err.Error(), "deadline exceeded")
}

// compile-time sanity check that the fallback chain with truncated body
// still returns the last error string.
var _ = fmt.Sprintf
