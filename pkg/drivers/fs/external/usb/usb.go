package usb

import (
	"files/pkg/drivers/fs/base"
	"files/pkg/models"
)

type UsbStorage struct {
	Base *base.FSStorage
}

func (s *UsbStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}

func (s *UsbStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}

func (s *UsbStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Base.Rename(fileParam)
}
