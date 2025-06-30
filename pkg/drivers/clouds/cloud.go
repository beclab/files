package clouds

import (
	"files/pkg/drivers/base"
	"files/pkg/models"
	"fmt"
	"io"
	"strings"

	"k8s.io/klog/v2"
)

type CloudStorage struct {
	Handler *base.HandlerParam
	Service base.CloudServiceInterface
}

func (s *CloudStorage) List(fileParam *models.FileParam) ([]byte, error) {
	klog.Infof("CLOUD list, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  fileParam.Path,
	}

	res, err := s.Service.List(data)
	if err != nil {
		return nil, fmt.Errorf("service list error: %v", err)
	}

	return res, nil
}

func (s *CloudStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	var owner = s.Handler.Owner

	klog.Infof("CLOUD preview, owner: %s, param: %s", owner, fileParam.Json())

	var path = fileParam.Path
	if strings.HasSuffix(path, "/") {
		return nil, fmt.Errorf("can't preview folder")
	}

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  path,
	}

	res, err := s.Service.GetFileMetaData(data)
	if err != nil {
		return nil, fmt.Errorf("service get file meta error: %v", err)
	}

	// todo task manager refactor

	return res, nil
}

func (s *CloudStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	return nil, nil
}
