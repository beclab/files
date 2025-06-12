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
