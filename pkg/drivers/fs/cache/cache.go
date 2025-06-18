package cache

import (
	"files/pkg/drivers/fs/base"
	"files/pkg/models"
)

type CacheStorage struct {
	Base *base.FSStorage
}

func (s *CacheStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}

func (s *CacheStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}
