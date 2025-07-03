package http

import (
	"files/pkg/common"
	"net/http"
)

func rawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	fileParam, handler, err := UrlPrep(r, "")
	if err != nil {
		return http.StatusBadRequest, err
	}

	return handler.RawHandler(fileParam)(w, r, d)
}
