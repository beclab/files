package posix

import (
	"files/pkg/models"
	"io"
)

type SmbStorage struct {
	Posix *PosixStorage
}

func (s *SmbStorage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Posix.List(fileParam)
}

func (s *SmbStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	return s.Posix.Preview(fileParam, queryParam)
}

func (s *SmbStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	return s.Posix.Raw(fileParam, queryParam)
}
