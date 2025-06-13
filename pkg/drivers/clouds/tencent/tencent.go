package tencent

import (
	"files/pkg/drivers/clouds/base"
)

type TencentStorage struct {
	Base *base.CloudStorage
}

func (s *TencentStorage) List() (int, error) {
	return s.Base.List()
}
