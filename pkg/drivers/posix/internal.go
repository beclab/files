package posix

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type InternalStorage struct {
	Posix *PosixStorage
}

func (s *InternalStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Posix.List(fileParam)
}

func (s *InternalStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Posix.CreateFolder(fileParam)
}

func (s *InternalStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Posix.Rename(fileParam)
}

func (s *InternalStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Posix.Preview(fileParam, imgSvc, fileCache)
}
