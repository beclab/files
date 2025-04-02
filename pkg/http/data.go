package http

import (
	"encoding/json"
	"files/pkg/drives"
	"files/pkg/rpc"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tomasen/realip"

	"files/pkg/common"
	"files/pkg/settings"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)

func handle(fn handleFunc, prefix string, server *settings.Server) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checked := CheckPathOwner(r, prefix)
		if !checked {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
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
			if status == http.StatusInternalServerError {
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

func NeedCheckPrefix(prefix string) bool {
	switch prefix {
	case "/api/resources", "/api/raw", "/api/preview", "/api/paste", "/api/permission", "/api/share":
		return true
	default:
		return false
	}
}

func CheckPathOwner(r *http.Request, prefix string) bool {
	if !NeedCheckPrefix(prefix) {
		return true
	}

	method := r.Method
	src := ""
	if prefix == "/api/preview" {
		vars := mux.Vars(r)
		src = "/" + vars["path"]
	} else {
		src = r.URL.Path
	}

	srcType, err := drives.ParsePathType(src, r, false, true)
	if err != nil {
		//klog.Errorf("ParsePathType error: %v", err)
		//return false
		srcType = drives.SrcTypeDrive
	}
	//srcType := r.URL.Query().Get("src_type")
	//if srcType == "" {
	//	srcType = r.URL.Query().Get("src")
	//	if srcType == "" {
	//		srcType = drives.SrcTypeDrive
	//	}
	//}

	dst := r.URL.Query().Get("destination")
	dstType, err := drives.ParsePathType(dst, r, true, true)
	if err != nil {
		if prefix == "/api/resources" && r.Method == http.MethodPatch {
			dstType = srcType
		} else {
			//klog.Errorf("ParsePathType error: %v", err)
			//return false
			dstType = drives.SrcTypeDrive
		}
	}
	//dstType := ""
	//if dst != "" {
	//	dstType = r.URL.Query().Get("dst_type")
	//	if dstType == "" {
	//		if prefix == "/api/resources" && r.Method == http.MethodPatch {
	//			dstType = srcType
	//		} else {
	//			dstType = drives.SrcTypeDrive
	//		}
	//	}
	//}

	klog.Infof("Checking owner for method: %s, prefix: %s, srcType: %s, src: %s, dstType: %s, dst: %s", method, prefix, srcType, src, dstType, dst)

	bfl := r.Header.Get("X-Bfl-User")
	pvc := ""
	if drives.IsBaseDrives(srcType) {
		pvc = rpc.ExtractPvcFromURL(src)
		klog.Infof("pvc: %s", pvc)
		if pvc != "External" && !strings.HasPrefix(pvc, "pvc-userspace-"+bfl+"-") && !strings.HasPrefix(pvc, "pvc-appcache-"+bfl+"-") {
			return false
		}
	}

	if prefix == "/api/paste" || (prefix == "/api/resources" && r.Method == http.MethodPatch) {
		if drives.IsBaseDrives(dstType) {
			pvc = rpc.ExtractPvcFromURL(dst)
			klog.Infof("pvc: %s", pvc)
			if pvc != "External" && !strings.HasPrefix(pvc, "pvc-userspace-"+bfl+"-") && !strings.HasPrefix(pvc, "pvc-appcache-"+bfl+"-") {
				return false
			}
		}
	}
	return true
}
