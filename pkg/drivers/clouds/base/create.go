package base

import (
	"errors"
	"files/pkg/common"
	"files/pkg/models"
	"files/pkg/utils"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

func (s *CloudStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	klog.Infof("CLOUD BASE create, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	var path = strings.Trim(fileParam.Path, "/")
	if path == "" {
		return 400, errors.New("path invalid")
	}

	var paths = strings.Split(path, "/")
	if len(paths) < 2 {
		return 400, errors.New("folder name invalid")
	}
	var parentPath = filepath.Join(paths[0 : len(paths)-1]...)
	var folderName = paths[len(paths)-1]

	param := &models.PostParam{
		Drive:      fileParam.FileType,
		Name:       fileParam.Extend,
		ParentPath: parentPath,
		FolderName: folderName,
	}
	klog.Infof("CLOUD BASE create, owner: %s, post: %s", s.Handler.Owner, utils.ToJson(param))

	res, err := s.Service.CreateFolder(param)
	if err != nil {
		klog.Errorln("Error calling drive/create_folder:", err)
		return common.ErrToStatus(err), err
	}

	result := res.(*models.CloudResponse)
	klog.Infof("CLOUD BASE create, owner: %s, result: %+v", s.Handler.Owner, result)
	return 0, nil

}
