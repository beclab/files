package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"k8s.io/klog/v2"

	"github.com/tomasen/realip"

	"files/pkg/common"
)

type handleFunc func(w http.ResponseWriter, r *http.Request) (int, error)

func MonkeyHandle(fn handleFunc, prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		status, err := fn(w, r)

		if status >= 400 || err != nil {
			clientIP := realip.FromRequest(r)
			klog.Errorf("%s: %v %s %v", r.URL.Path, status, clientIP, err)
		}

		if status != 0 {
			if status >= http.StatusBadRequest {
				// if status == http.StatusInternalServerError {
				txt := http.StatusText(status)
				if err != nil {
					txt = err.Error()
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code":    1,
					"message": txt,
				})
			} else {
				txt := http.StatusText(status)
				http.Error(w, strconv.Itoa(status)+" "+txt, status)
			}
			return
		}
	})

	return stripPrefix(prefix, handler)
}

func timingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path, err := url.QueryUnescape(r.URL.Path)
		if err != nil {
			klog.Errorf("url decode error: %v", err)
		}
		klog.Infof("%s %s starts at %v\n", r.Method, path, start)
		defer func() {
			elapsed := time.Since(start)
			klog.Infof("%s %s execution time: %v\n", r.Method, path, elapsed)
		}()

		next.ServeHTTP(w, r)
	})
}

func cookieMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bflName := r.Header.Get("X-Bfl-User")
		oldCookie := common.BflCookieCache[bflName]
		newCookie := r.Header.Get("Cookie")
		if newCookie != oldCookie {
			common.BflCookieCache[bflName] = newCookie
		}
		klog.Infof("BflCookieCache= %v", common.BflCookieCache)
		next.ServeHTTP(w, r)
	})
}
