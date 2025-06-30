package posix

import (
	"files/pkg/models"
	"io"
)

type UsbStorage struct {
	Posix *PosixStorage
}

func (s *UsbStorage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Posix.List(fileParam)
}

func (s *UsbStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	return s.Posix.Preview(fileParam, queryParam)
}

func (s *UsbStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	return s.Posix.Raw(fileParam, queryParam)
}
