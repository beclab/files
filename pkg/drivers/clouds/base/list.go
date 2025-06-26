package base

import (
	"files/pkg/common"
	"files/pkg/models"
	"files/pkg/utils"

	"k8s.io/klog/v2"
)

func (s *CloudStorage) List(fileParam *models.FileParam) (int, error) {
	klog.Infof("CLOUD list, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	var w = s.Handler.ResponseWriter
	var r = s.Handler.Request
	var err error

	var path = fileParam.Path
	if fileParam.FileType == "dropbox" {
		path = "/" + path
	}

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  path,
	}

	klog.Infof("CLOUD list, owner: %s, get: %s", s.Handler.Owner, utils.ToJson(data))

	res, err := s.Service.List(data)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return common.ErrToStatus(err), err
	}
	listRes := res.(*models.CloudListResponse)
	return common.RenderJSON(w, r, listRes)
}
