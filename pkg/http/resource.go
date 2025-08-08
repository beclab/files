package http

import (
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/files"
	"net/http"
)

func resourceDeleteHandler(fileCache files.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		//srcType := r.URL.Query().Get("src")
		srcType, err := drives.ParsePathType(r.URL.Path, r, false, true)
		if err != nil {
			return http.StatusBadRequest, err
		}

		handler, err := drives.GetResourceService(srcType)
		if err != nil {
			return http.StatusBadRequest, err
		}

		return handler.DeleteHandler(fileCache)(w, r, d)
	}
}

func resourcePutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	fileParam, handler, err := UrlPrep(r, "")
	if err != nil {
		return http.StatusBadRequest, err
	}

	return handler.PutHandler(fileParam)(w, r, d)
}

func resourcePatchHandler(fileCache files.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		fileParam, handler, err := UrlPrep(r, "")
		if err != nil {
			return http.StatusBadRequest, err
		}

		return handler.PatchHandler(fileCache, fileParam)(w, r, d)
	}
}
