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

func (s *HddStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}

func (s *HddStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Base.Rename(fileParam)
}
