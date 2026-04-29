package tasks

import (
	"net/http"
	"time"
)

// streamHTTPClient is shared by task paths that download large bodies (file
// downloads, SSE-style file lists). Client.Timeout is intentionally NOT set
// because it covers the entire response body read; for multi-GB downloads
// that would clip legitimate transfers. ResponseHeaderTimeout instead
// guards against an origin that never starts sending headers, while the
// per-request context.Context (passed via http.NewRequestWithContext)
// remains the primary deadline a caller can use to cancel.
var streamHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	},
}
