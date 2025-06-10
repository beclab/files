package posix

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type DataStorage struct {
	Posix *PosixStorage
}

func (s *DataStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Posix.List(fileParam)
}

func (s *DataStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Posix.CreateFolder(fileParam)
}

func (s *DataStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Posix.Rename(fileParam)
}

func (s *DataStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Posix.Preview(fileParam, imgSvc, fileCache)
}
