package base

import (
	"files/pkg/drivers/base"
)

type CloudStorage struct {
	Base    *base.BaseStorage
	Service base.CloudServiceInterface
}
