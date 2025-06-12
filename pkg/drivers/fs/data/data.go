package data

import (
	"files/pkg/drivers/fs/base"
	"files/pkg/models"
)

type DataStorage struct {
	Base *base.FSStorage
}

func (s *DataStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}
