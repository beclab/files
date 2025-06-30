package posix

import (
	"files/pkg/models"
	"io"
)

type InternalStorage struct {
	Posix *PosixStorage
}

func (s *InternalStorage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Posix.List(fileParam)
}

func (s *InternalStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	return s.Posix.Preview(fileParam, queryParam)
}

func (s *InternalStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	return s.Posix.Raw(fileParam, queryParam)
}
