package rclone

import (
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/operations"
)

type Interface interface {
	InitServes()
	StartHttp(configs []*config.Config) error
	GetConfig() config.Interface
	GetOperation() operations.Interface
}
