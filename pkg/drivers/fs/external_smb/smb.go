package externalsmb

import "files/pkg/drivers/fs/base"

type SMBStorage struct {
	Base *base.FSStorage
}

func (s *SMBStorage) List() (int, error) {
	return s.Base.List()
}
