package http

import (
	"files/pkg/drivers/base"
	"files/pkg/models"
)

func listHandler(handler base.Execute, fileParam *models.FileParam) (int, error) {
	return handler.List(fileParam)
}

// func listHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
// srcType, err := drives.ParsePathType(r.URL.Path, r, false, true)
// if err != nil {
// 	return http.StatusBadRequest, err
// }

// klog.Infof("srcType: %s, path: %s", srcType, r.URL.Path)

// handler := drivers.NewDriver(srcType, w, r, d)
// return handler.List()

// return 0, nil
// }
