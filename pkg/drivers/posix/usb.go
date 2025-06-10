package posix

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type UsbStorage struct {
	Posix *PosixStorage
}

func (s *UsbStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Posix.List(fileParam)
}

func (s *UsbStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Posix.CreateFolder(fileParam)
}

func (s *UsbStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Posix.Rename(fileParam)
}

func (s *UsbStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Posix.Preview(fileParam, imgSvc, fileCache)
}
