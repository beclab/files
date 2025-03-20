package http

import (
	"files/pkg/drives"
	"files/pkg/rpc"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
	"net/http"
	"strconv"
	"strings"

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
			txt := http.StatusText(status)
			http.Error(w, strconv.Itoa(status)+" "+txt, status)
			return
		}
	})

	return stripPrefix(prefix, handler)
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
	klog.Infof("~~~~Temp log: URL = %s, prefix = %s", r.URL, prefix)
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

	srcType := r.URL.Query().Get("src_type")
	if srcType == "" {
		srcType = r.URL.Query().Get("src")
		if srcType == "" {
			srcType = drives.SrcTypeDrive
		}
	}

	dst := r.URL.Query().Get("destination")
	dstType := ""
	if dst != "" {
		dstType = r.URL.Query().Get("dst_type")
		if dstType == "" {
			dstType = drives.SrcTypeDrive
		}
	}

	klog.Infof("Checking owner for method: %s, prefix: %s, srcType: %s, src: %s, dstType: %s, dst: %s", method, prefix, srcType, src, dstType, dst)

	bfl := r.Header.Get("X-Bfl-User")
	pvc := ""
	if drives.IsBaseDrives(srcType) {
		pvc = rpc.ExtractPvcFromURL(src)
		klog.Infof("pvc: %s", pvc)
		if !strings.HasPrefix(pvc, "pvc-userspace-"+bfl+"-") && !strings.HasPrefix(pvc, "pvc-appcache-"+bfl+"-") {
			return false
		}
	}

	if prefix == "/api/paste" || (prefix == "/api/resources" && r.Method == http.MethodPatch) {
		if drives.IsBaseDrives(dstType) {
			pvc = rpc.ExtractPvcFromURL(src)
			klog.Infof("pvc: %s", pvc)
			if !strings.HasPrefix(pvc, "pvc-userspace-"+bfl+"-") && !strings.HasPrefix(pvc, "pvc-appcache-"+bfl+"-") {
				return false
			}
		}
	}
	return true
}
