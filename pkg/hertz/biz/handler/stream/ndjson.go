// Package stream provides a small NDJSON chunked-streaming writer shared
// by the archive entries and dir-usage endpoints. It owns the response
// framing (content-type, per-line flush, terminal _done / _error lines)
// so handlers only decide what to emit.
package stream

import (
	"encoding/json"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

// Writer streams newline-delimited JSON over a Hertz response. The wire
// protocol matches the archive entries stream: zero or more progress
// objects, then exactly one terminal line that is either
// {"_done":true,...} or {"_error":"...","code":"..."}.
type Writer struct {
	c   *app.RequestContext
	enc *json.Encoder
}

// NewWriter sets the NDJSON response headers and 200 status, then binds
// a Writer to the response body.
func NewWriter(c *app.RequestContext) *Writer {
	c.SetContentType("application/x-ndjson; charset=utf-8")
	c.Response.Header.Set("Cache-Control", "no-store")
	c.Response.Header.Set("X-Content-Type-Options", "nosniff")
	c.SetStatusCode(http.StatusOK)
	return &Writer{c: c, enc: json.NewEncoder(c.Response.BodyWriter())}
}

// Emit writes one progress object and flushes. The encode error (e.g.
// the client went away) is returned so the producer can stop early.
func (w *Writer) Emit(v any) error {
	if err := w.enc.Encode(v); err != nil {
		return err
	}
	w.c.Flush()
	return nil
}

// Done writes the terminal {"_done":true, ...extra} line.
func (w *Writer) Done(extra map[string]any) {
	line := map[string]any{"_done": true}
	for k, v := range extra {
		line[k] = v
	}
	_ = w.enc.Encode(line)
	w.c.Flush()
}

// Fail writes the terminal {"_error":msg,"code":code} line.
func (w *Writer) Fail(err error, code string) {
	_ = w.enc.Encode(map[string]any{"_error": err.Error(), "code": code})
	w.c.Flush()
}
