package rclone

import (
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/drivers/clouds/rclone/serve"
)

type Interface interface {
	InitServes()
	StartHttp(configs []*config.Config) error
	GetConfig() config.Interface
	GetOperation() operations.Interface
	GetServe() serve.Interface
}
