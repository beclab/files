package http

import (
	"k8s.io/klog/v2"
	"net/http"
	"strconv"

	"github.com/tomasen/realip"

	"files/pkg/common"
	"files/pkg/settings"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)

func handle(fn handleFunc, prefix string, server *settings.Server) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if prefix == "/api/paste" || (prefix == "/api/resources" && r.Method == http.MethodPatch) {
			klog.Warningf("Is src and dst yours? We'll check it for %s %s", r.Method, r.URL.Path)
		} else if prefix == "/api/resources" || prefix == "/api/raw" || prefix == "/api/preview" {
			klog.Warningf("Is src yours? We'll check it for %s %s", r.Method, r.URL.Path)
		}
		
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		status, err := fn(w, r, &common.Data{
			Server: server,
		})

		if status >= 400 || err != nil {
			clientIP := realip.FromRequest(r)
			klog.Errorf("%s: %v %s %v", r.URL.Path, status, clientIP, err)
		}

		if status != 0 {
			txt := http.StatusText(status)
			http.Error(w, strconv.Itoa(status)+" "+txt, status)
			return
		}
	})

	return stripPrefix(prefix, handler)
}
