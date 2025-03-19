package http

import (
	"files/pkg/common"
	"files/pkg/drives"
	"net/http"
)

func rawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	srcType := r.URL.Query().Get("src")
	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return http.StatusBadRequest, err
	}

	return handler.RawHandler(w, r, d)
}
