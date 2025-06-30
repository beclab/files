package posix

import (
	"files/pkg/models"
	"io"
)

type CacheStorage struct {
	Posix *PosixStorage
}

func (s *CacheStorage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Posix.List(fileParam)
}

func (s *CacheStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	return s.Posix.Preview(fileParam, queryParam)
}

func (s *CacheStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	return s.Posix.Raw(fileParam, queryParam)
}
