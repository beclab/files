package cache

import "files/pkg/models"

func (s *CacheStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Base.Rename(fileParam)
}
