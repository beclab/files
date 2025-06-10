package dropbox

import (
	"files/pkg/drivers/clouds/base"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type DropBoxStorage struct {
	Base *base.CloudStorage
}

func (s *DropBoxStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}

func (s *DropBoxStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}

func (s *DropBoxStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Base.Rename(fileParam)
}

func (s *DropBoxStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Base.Preview(fileParam, imgSvc, fileCache)
}
