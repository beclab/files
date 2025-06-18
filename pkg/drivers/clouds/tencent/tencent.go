package tencent

import (
	"files/pkg/drivers/clouds/base"
	"files/pkg/models"
)

type TencentStorage struct {
	Base *base.CloudStorage
}

func (s *TencentStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}

func (s *TencentStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Base.CreateFolder(fileParam)
}
