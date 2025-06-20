package google

import (
	"files/pkg/drivers/base"
	basecloud "files/pkg/drivers/clouds/base"
)

type GoogleStorage struct {
	Base    *basecloud.CloudStorage
	Service base.CloudServiceInterface
}

var _ base.Execute = &GoogleStorage{}
