package handle_func

import (
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/hertz/biz/model/api/permission"
	"files/pkg/models"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/spf13/afero"
	"strings"
)

func PermissionGetHandler(c *app.RequestContext, _ interface{}, prefix string) ([]byte, int, error) {
	var p = string(c.Path())
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, consts.StatusBadRequest, errors.New("path invalid")
	}

	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, consts.StatusBadRequest, errors.New("user not found")
	}
	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return nil, consts.StatusBadRequest, err
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, consts.StatusBadRequest, err
	}
	urlPath := uri + fileParam.Path
	dealUrlPath := strings.TrimPrefix(urlPath, "/data")

	exists, err := afero.Exists(files.DefaultFs, dealUrlPath)
	if err != nil {
		return nil, consts.StatusInternalServerError, err
	}
	if !exists {
		return nil, consts.StatusNotFound, nil
	}

	responseData := make(map[string]interface{})
	responseData["uid"], err = files.GetUID(files.DefaultFs, dealUrlPath)
	if err != nil {
		return nil, consts.StatusInternalServerError, err
	}
	return common.ToBytes(responseData), consts.StatusOK, nil
}

func PermissionPutHandler(c *app.RequestContext, req interface{}, prefix string) ([]byte, int, error) {
	var p = string(c.Path())
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, consts.StatusBadRequest, errors.New("path invalid")
	}

	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, consts.StatusBadRequest, errors.New("user not found")
	}
	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return nil, consts.StatusBadRequest, err
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, consts.StatusBadRequest, err
	}
	urlPath := uri + fileParam.Path
	dealUrlPath := strings.TrimPrefix(urlPath, "/data")

	exists, err := afero.Exists(files.DefaultFs, dealUrlPath)
	if err != nil {
		return nil, consts.StatusInternalServerError, err
	}
	if !exists {
		return nil, consts.StatusNotFound, nil
	}

	uid := int(req.(permission.PutPermissionReq).Uid)
	gid := uid

	recursive := req.(permission.PutPermissionReq).Recursive

	if recursive == nil || *recursive == 0 {
		err = files.Chown(files.DefaultFs, dealUrlPath, uid, gid)
	} else {
		err = files.ChownRecursive(urlPath, uid, gid)
	}
	if err != nil {
		return nil, consts.StatusInternalServerError, err
	}
	return nil, common.ErrToStatus(err), err
}
