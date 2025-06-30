package clouds

import "files/pkg/models"

type GoogleDriveStorage struct {
	Cloud *CloudStorage
}

func (s *GoogleDriveStorage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Cloud.List(fileParam)
}
