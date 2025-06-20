package drive

import (
	"files/pkg/drivers/fs/base"
	"files/pkg/models"
)

type DriveStorage struct {
	Base *base.FSStorage
}

func (s *DriveStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}

func (s *DriveStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}

func (s *DriveStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Base.Rename(fileParam)
}
