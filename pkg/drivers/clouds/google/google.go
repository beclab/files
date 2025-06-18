package google

import (
	"files/pkg/drivers/base"
	basecloud "files/pkg/drivers/clouds/base"
	"files/pkg/models"
)

type GoogleStorage struct {
	Base    *basecloud.CloudStorage
	Service base.CloudServiceInterface
}

var _ base.Execute = &GoogleStorage{}

func (s *GoogleStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return 0, nil
}
