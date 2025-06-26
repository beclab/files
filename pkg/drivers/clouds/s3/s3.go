package s3

import (
	"files/pkg/drivers/clouds/base"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type S3Storage struct {
	Base *base.CloudStorage
}

func (s *S3Storage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}

func (s *S3Storage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}

func (s *S3Storage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Base.Rename(fileParam)
}

func (s *S3Storage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Base.Preview(fileParam, imgSvc, fileCache)
}
