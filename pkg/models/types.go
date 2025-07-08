package models

import (
	"errors"
	"files/pkg/constant"
	"net/http"
	"strings"
)

type HttpContextArgs struct {
	FileParam  *FileParam  `json:"fileParam"`
	QueryParam *QueryParam `json:"queryParam"`
}

func NewHttpContextArgs(r *http.Request, prefix string, enableThumbnails bool, resizePreview bool) (*HttpContextArgs, error) {
	var p = r.URL.Path
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, errors.New("path invalid")
	}

	var owner = r.Header.Get(constant.REQUEST_HEADER_OWNER)
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var fileParam, err = CreateFileParam(owner, path)
	if err != nil {
		return nil, err
	}

	var queryParam = CreateQueryParam(owner, r, enableThumbnails, resizePreview)

	return &HttpContextArgs{
		FileParam:  fileParam,
		QueryParam: queryParam,
	}, nil
}
