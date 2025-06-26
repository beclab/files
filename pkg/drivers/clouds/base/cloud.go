package base

import (
	"files/pkg/drivers/base"
)

type CloudStorage struct {
	Handler *base.HandlerParam
	Service base.CloudServiceInterface
}
