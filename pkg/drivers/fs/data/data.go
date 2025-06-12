package data

import (
	"files/pkg/drivers/fs/base"
)

type DataStorage struct {
	Base *base.FSStorage
}

func (s *DataStorage) List() (int, error) {
	return s.Base.List()
}
