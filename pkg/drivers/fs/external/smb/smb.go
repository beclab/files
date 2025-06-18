package smb

import (
	"files/pkg/drivers/fs/base"
	"files/pkg/models"
)

type SmbStorage struct {
	Base *base.FSStorage
}

func (s *SmbStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}

func (s *SmbStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}
