package clouds

import (
	"files/pkg/models"
)

type DropboxStorage struct {
	Cloud *CloudStorage
}

func (s *DropboxStorage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Cloud.List(fileParam)
}
