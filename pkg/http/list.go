package http

import (
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drives"
	"net/http"

	"k8s.io/klog/v2"
)

func listHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {

	srcType, err := drives.ParsePathType(r.URL.Path, r, false, true)
	if err != nil {
		return http.StatusBadRequest, err
	}

	klog.Infof("srcType: %s, path: %s", srcType, r.URL.Path)

	handler := drivers.NewDriver(srcType, w, r, d)
	return handler.List()
}
