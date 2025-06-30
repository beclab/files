package posix

import (
	"files/pkg/models"
	"io"
)

type ExternalStorage struct {
	Posix *PosixStorage
}

func (s *ExternalStorage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Posix.List(fileParam)
}

func (s *ExternalStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	return s.Posix.Preview(fileParam, queryParam)
}

func (s *ExternalStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	return s.Posix.Raw(fileParam, queryParam)
}
