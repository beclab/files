package drive

import (
	"files/pkg/drivers/fs/base"
)

type DriveStorage struct {
	Base *base.FSStorage
}

func (s *DriveStorage) List() (int, error) {
	return s.Base.List()
}
