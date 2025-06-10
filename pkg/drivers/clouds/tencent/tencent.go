package tencent

import (
	"files/pkg/drivers/clouds/base"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type TencentStorage struct {
	Base *base.CloudStorage
}

func (s *TencentStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}

func (s *TencentStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}

func (s *TencentStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Base.Rename(fileParam)
}

func (s *TencentStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Base.Preview(fileParam, imgSvc, fileCache)
}
