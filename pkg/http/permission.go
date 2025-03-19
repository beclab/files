package http

import (
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"fmt"
	"github.com/spf13/afero"
	"net/http"
	"strconv"
)

func permissionGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	responseData := make(map[string]interface{})
	responseData["uid"], err = fileutils.GetUID(files.DefaultFs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return common.RenderJSON(w, r, responseData)
}

func permissionPutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	uidStr := r.URL.Query().Get("uid")
	if uidStr == "" {
		return http.StatusBadRequest, fmt.Errorf("uid param is required")
	}
	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return http.StatusBadRequest, err
	}
	gid := uid

	recursiveStr := r.URL.Query().Get("recursive")
	recursive := 0
	if recursiveStr != "" {
		recursive, err = strconv.Atoi(recursiveStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	if recursive == 0 {
		err = fileutils.Chown(files.DefaultFs, r.URL.Path, uid, gid)
	} else {
		err = fileutils.ChownRecursive("/data"+r.URL.Path, uid, gid)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return common.ErrToStatus(err), err
}
