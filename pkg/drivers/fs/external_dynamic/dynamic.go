package externaldynamic

import "files/pkg/drivers/fs/base"

type DynamicStorage struct {
	Base *base.FSStorage
}

func (s *DynamicStorage) List() (int, error) {
	return s.Base.List()
}
