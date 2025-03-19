package http

import (
	"files/pkg/drives"
	"files/pkg/rpc"
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
		checked, err := CheckPathOwner(r, prefix)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
		}
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

func CheckPathOwner(r *http.Request, prefix string) (bool, error) {
	if prefix != "/api/resources" && prefix != "/api/raw" && prefix != "/api/preview" && prefix != "/api/paste" {
		return true, nil
	}

	var err error = nil
	method := r.Method
	src := r.URL.Path

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

	bflRequest := r.Header.Get("X-Bfl-User")
	bflParsed := ""
	if drives.IsBaseDrives(srcType) {
		bflParsed, err = rpc.PVCs.GetBfl(rpc.ExtractPvcFromURL(src))
		if err != nil {
			return false, err
		}
		if bflParsed != bflRequest {
			return false, nil
		}
	}

	if prefix == "/api/paste" || (prefix == "/api/resources" && r.Method == http.MethodPatch) {
		if drives.IsBaseDrives(dstType) {
			bflParsed, err = rpc.PVCs.GetBfl(rpc.ExtractPvcFromURL(dst))
			if err != nil {
				return false, err
			}
			if bflParsed != bflRequest {
				return false, nil
			}
		}
	}
	return true, nil
}
