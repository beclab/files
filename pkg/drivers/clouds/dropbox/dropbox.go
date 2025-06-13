package dropbox

import (
	"files/pkg/drivers/clouds/base"
)

type DropBoxStorage struct {
	Base *base.CloudStorage
}

func (s *DropBoxStorage) List() (int, error) {
	return s.Base.List()
}
