package models

import (
	"context"
	"errors"
	"files/pkg/common"
	"github.com/cloudwego/hertz/pkg/app"
	"strings"
)

type HttpContextArgs struct {
	FileParam  *FileParam  `json:"fileParam"`
	QueryParam *QueryParam `json:"queryParam"`
}

func NewHttpContextArgs(ctx context.Context, c *app.RequestContext, prefix string, enableThumbnails bool, resizePreview bool) (*HttpContextArgs, error) {
	var p = string(c.Path())
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, errors.New("path invalid")
	}

	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var fileParam, err = CreateFileParam(owner, path)
	if err != nil {
		return nil, err
	}

	var queryParam = CreateQueryParam(owner, ctx, c, enableThumbnails, resizePreview)

	return &HttpContextArgs{
		FileParam:  fileParam,
		QueryParam: queryParam,
	}, nil
}
