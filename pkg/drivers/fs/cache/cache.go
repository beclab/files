package cache

import (
	"files/pkg/drivers/fs/base"
)

type CacheStorage struct {
	Base *base.FSStorage
}

func (s *CacheStorage) List() (int, error) {
	return s.Base.List()
}
