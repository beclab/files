package dropbox

import (
	"files/pkg/drivers/clouds/base"
	"files/pkg/models"
)

type DropBoxStorage struct {
	Base *base.CloudStorage
}

func (s *DropBoxStorage) List(fileParam *models.FileParam) (int, error) {
	return s.Base.List(fileParam)
}
