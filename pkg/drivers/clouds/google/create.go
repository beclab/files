package google

import (
	"errors"
	"files/pkg/common"
	"files/pkg/drives/model"
	"files/pkg/models"
	"files/pkg/utils"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

func (s *GoogleStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	klog.Infof("CLOUD GOOGLE create, owner: %s, param: %s", s.Base.Handler.Owner, fileParam.Json())
	var w = s.Base.Handler.ResponseWriter
	var r = s.Base.Handler.Request
	var err error

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

	klog.Infof("GOOGLE BASE create, owner: %s, post: %s", s.Base.Handler.Owner, utils.ToJson(param))

	res, err := s.Service.CreateFolder(param)
	if err != nil {
		klog.Errorf("CLOUD GOOGLE create folder error: %v, owner: %s", err, s.Base.Handler.Owner)
		return common.ErrToStatus(err), err
	}

	result := res.(*model.GoogleDriveResponse)
	klog.Infof("CLOUD GOOGLE create, owner: %s, result: %+v", s.Base.Handler.Owner, result)
	return common.RenderSuccess(w, r)
}
