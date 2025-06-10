package posix

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type HddStorage struct {
	Posix *PosixStorage
}

func (s *HddStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Posix.List(fileParam)
}

func (s *HddStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Posix.CreateFolder(fileParam)
}

func (s *HddStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Posix.Rename(fileParam)
}

func (s *HddStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Posix.Preview(fileParam, imgSvc, fileCache)
}
