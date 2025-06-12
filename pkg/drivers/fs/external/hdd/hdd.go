package hdd

import (
	"files/pkg/drivers/fs/base"
	"files/pkg/models"
)

type HddStorage struct {
	Base *base.FSStorage
}

func (s *HddStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}
