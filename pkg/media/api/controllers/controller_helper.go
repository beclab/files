package controllers

import (
	"net/http"
	"net/url"

	"github.com/cloudwego/hertz/pkg/app"
)

type hertzResponseWriter struct {
	ctx     *app.RequestContext
	wroteHS bool
	status  int
	header  http.Header
}

func buildHTTPRequest(ctx *app.RequestContext) *http.Request {
	u := &url.URL{
		Scheme: func() string {
			return "http"
		}(),
		Host:     string(ctx.Host()),
		Path:     string(ctx.Path()),
		RawQuery: string(ctx.URI().QueryString()),
	}

	req := &http.Request{
		Method: string(ctx.Method()),
		URL:    u,
		Header: http.Header{},
		Host:   string(ctx.Host()),
	}

	ctx.Request.Header.VisitAll(func(k, v []byte) {
		req.Header.Add(string(k), string(v))
	})

	return req
}

func newHertzResponseWriter(ctx *app.RequestContext) *hertzResponseWriter {
	return &hertzResponseWriter{
		ctx:    ctx,
		header: make(http.Header),
	}
}

func (w *hertzResponseWriter) Header() http.Header {
	return w.header
}

func (w *hertzResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHS {
		return
	}
	w.wroteHS = true
	w.status = statusCode

	for k, vals := range w.header {
		for _, v := range vals {
			w.ctx.Response.Header.Add(k, v)
		}
	}
	w.ctx.SetStatusCode(statusCode)
}

func (w *hertzResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHS {
		w.WriteHeader(http.StatusOK)
	}
	if len(b) == 0 {
		return 0, nil
	}
	if _, err := w.ctx.Write(b); err != nil {
		return 0, err
	}
	return len(b), nil
}
