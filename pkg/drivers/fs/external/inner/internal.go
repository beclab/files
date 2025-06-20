package inner

import (
	"files/pkg/drivers/fs/base"
	"files/pkg/models"
)

type InternalStorage struct {
	Base *base.FSStorage
}

func (s *InternalStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}

func (s *InternalStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Base.Rename(fileParam)
}
