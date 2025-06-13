package s3

import (
	"files/pkg/drivers/clouds/base"
)

type S3Storage struct {
	Base *base.CloudStorage
}

func (s *S3Storage) List() (int, error) {
	return s.Base.List()
}
