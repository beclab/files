package handler

import (
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

var permissionPrefix = "/api/permission"

func PermissionGetHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	var p = r.URL.Path
	var path = strings.TrimPrefix(p, permissionPrefix)
	if path == "" {
		return http.StatusBadRequest, errors.New("path invalid")
	}

	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return http.StatusBadRequest, errors.New("user not found")
	}
	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return http.StatusBadRequest, err
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	urlPath := uri + fileParam.Path
	dealUrlPath := strings.TrimPrefix(urlPath, "/data")

	exists, err := afero.Exists(files.DefaultFs, dealUrlPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	responseData := make(map[string]interface{})
	responseData["uid"], err = files.GetUID(files.DefaultFs, dealUrlPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return common.RenderJSON(w, r, responseData)
}

func PermissionPutHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	var p = r.URL.Path
	var path = strings.TrimPrefix(p, permissionPrefix)
	if path == "" {
		return http.StatusBadRequest, errors.New("path invalid")
	}

	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return http.StatusBadRequest, errors.New("user not found")
	}
	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return http.StatusBadRequest, err
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	urlPath := uri + fileParam.Path
	dealUrlPath := strings.TrimPrefix(urlPath, "/data")

	exists, err := afero.Exists(files.DefaultFs, dealUrlPath)
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
		err = files.Chown(files.DefaultFs, dealUrlPath, uid, gid)
	} else {
		err = files.ChownRecursive(urlPath, uid, gid)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return common.ErrToStatus(err), err
}
