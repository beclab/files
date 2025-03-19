package http

import (
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/fileutils"
	"net/http"
)

func resourceGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	srcType := r.URL.Query().Get("src")

	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return http.StatusBadRequest, err
	}

	return handler.GetHandler(w, r, d)
}

func resourceDeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		srcType := r.URL.Query().Get("src")

		handler, err := drives.GetResourceService(srcType)
		if err != nil {
			return http.StatusBadRequest, err
		}

		return handler.DeleteHandler(fileCache)(w, r, d)
	}
}

func resourcePostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	srcType := r.URL.Query().Get("src")

	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return http.StatusBadRequest, err
	}

	return handler.PostHandler(w, r, d)
}

func resourcePutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	srcType := r.URL.Query().Get("src")

	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return http.StatusBadRequest, err
	}

	return handler.PutHandler(w, r, d)
}

func resourcePatchHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		srcType := r.URL.Query().Get("src")

		handler, err := drives.GetResourceService(srcType)
		if err != nil {
			return http.StatusBadRequest, err
		}

		return handler.PatchHandler(fileCache)(w, r, d)
	}
}
