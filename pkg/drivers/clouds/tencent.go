package clouds

import (
	"files/pkg/models"
)

type TencentStorage struct {
	Cloud *CloudStorage
}

func (s *TencentStorage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Cloud.List(fileParam)
}
