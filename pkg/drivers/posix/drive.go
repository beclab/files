package posix

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type DriveStorage struct {
	Posix *PosixStorage
}

func (s *DriveStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Posix.List(fileParam)
}

func (s *DriveStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Posix.CreateFolder(fileParam)
}

func (s *DriveStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Posix.Rename(fileParam)
}

func (s *DriveStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Posix.Preview(fileParam, imgSvc, fileCache)
}
