package google

import (
	"errors"
	"files/pkg/common"
	"files/pkg/models"
	"files/pkg/utils"
	"strings"

	"k8s.io/klog/v2"
)

// List implements base.DriverInterface.
func (g *GoogleStorage) List(fileParam *models.FileParam) (int, error) {
	klog.Infof("GOOGLE list, owner: %s, param: %s", g.Base.Handler.Owner, fileParam.Json())
	var w = g.Base.Handler.ResponseWriter
	var r = g.Base.Handler.Request
	var err error

	var path = fileParam.Path
	if path == "/root/" {
		path = "/"
	} else {
		path = strings.Trim(path, "/")
	}

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  path,
	}

	klog.Infof("GOOGLE list, owner: %s, get: %s", g.Base.Handler.Owner, utils.ToJson(data))

	res, err := g.Service.List(data)
	if err != nil {
		klog.Errorf("List error: %v", err)
		return common.ErrToStatus(err), err
	}

	listFiles := res.(*models.GoogleDriveListResponse)
	if !listFiles.IsSuccess() {
		err = errors.New(listFiles.FailMessage())
		return common.ErrToStatus(err), err
	}

	klog.Infof("CLOUD GOOGLE list result, count: %d", len(listFiles.Data))

	return common.RenderJSON(w, r, listFiles)

}
