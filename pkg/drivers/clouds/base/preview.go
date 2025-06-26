package base

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

func (s *CloudStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	var owner = s.Handler.Owner

	klog.Infof("CLOUD preview, owner: %s, param: %s", owner, fileParam.Json())

	var path = fileParam.Path
	if strings.HasSuffix(path, "/") {
		return 503, fmt.Errorf("can't preview folder")
	}

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  path,
	}

	res, err := s.Service.GetFileMetaData(data)
	if err != nil {
		return 503, err
	}

	listRes := res.(*models.CloudListResponse)
	fmt.Println("---1---", listRes)

	return 0, nil
}
