package http

import (
	"files/pkg/common"
	"files/pkg/drives"
	"net/http"

	"k8s.io/klog/v2"
)

func rawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	srcType := r.URL.Query().Get("src")
	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return http.StatusBadRequest, err
	}
	klog.Info("rawHandler", "srcType", srcType, "handler", handler)

	klog.Info(w, r, d)
	return handler.RawHandler(w, r, d)
}
