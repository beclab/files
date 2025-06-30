package clouds

import "files/pkg/models"

type S3Storage struct {
	Cloud *CloudStorage
}

func (s *S3Storage) List(fileParam *models.FileParam) ([]byte, error) {
	return s.Cloud.List(fileParam)
}
