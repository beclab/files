package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

// DefaultOlaresdTimeout is the default timeout for HTTP requests to olaresd
// (mount / umount / mounted list). Keeping it bounded prevents a stuck olaresd
// from piling up goroutines inside the Files server.
const DefaultOlaresdTimeout = 30 * time.Second

// ErrOlaresdNoResponse is returned when every URL in a fallback chain failed
// without producing a parseable JSON body.
var ErrOlaresdNoResponse = errors.New("olaresd upstream unreachable")

// CallOlaresdFallback POSTs body to each URL in order until one succeeds.
//
// A URL is considered successful if the HTTP call returns a 2xx/3xx status
// and a JSON body that can be decoded into map[string]interface{}. On any other
// outcome (network error, non-JSON body, 4xx/5xx) we log the failure and move
// on to the next URL.
//
// The returned map is the most recent parsed JSON body (may be nil when every
// attempt failed before producing one). The error is nil when at least one URL
// succeeded; otherwise it is the last underlying error (or ErrOlaresdNoResponse
// when no attempt even produced a usable error).
func CallOlaresdFallback(urls []string, body []byte, header http.Header, timeout time.Duration) (map[string]interface{}, error) {
	if len(urls) == 0 {
		return nil, errors.New("no olaresd url provided")
	}
	if timeout <= 0 {
		timeout = DefaultOlaresdTimeout
	}

	client := &http.Client{Timeout: timeout}

	var (
		lastBody map[string]interface{}
		lastErr  error
	)

	for _, url := range urls {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			klog.Warningf("olaresd: build request failed, url=%s, err=%v", url, err)
			lastErr = err
			continue
		}

		if header != nil {
			req.Header = header.Clone()
		}
		req.Header.Set("Content-Type", "application/json")
		if req.Header.Get("X-Signature") == "" {
			req.Header.Set("X-Signature", "temp_signature")
		}

		resp, err := client.Do(req)
		if err != nil {
			klog.Warningf("olaresd: request failed, url=%s, err=%v", url, err)
			lastErr = err
			continue
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			klog.Warningf("olaresd: read body failed, url=%s, status=%s, err=%v", url, resp.Status, err)
			lastErr = err
			continue
		}

		parsed := map[string]interface{}{}
		if jerr := json.Unmarshal(respBody, &parsed); jerr != nil {
			klog.Warningf("olaresd: body not json, url=%s, status=%s, body=%s", url, resp.Status, truncate(respBody, 512))
			lastErr = fmt.Errorf("non-json response from %s (status %s)", url, resp.Status)
			continue
		}

		lastBody = parsed

		if resp.StatusCode >= 400 {
			klog.Warningf("olaresd: http %s from %s, body=%v", resp.Status, url, parsed)
			lastErr = fmt.Errorf("http %s from %s", resp.Status, url)
			continue
		}

		return parsed, nil
	}

	if lastErr == nil {
		lastErr = ErrOlaresdNoResponse
	}
	return lastBody, lastErr
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}
