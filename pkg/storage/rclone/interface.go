package rclone

import (
	"files/pkg/storage/rclone/config"
	"files/pkg/storage/rclone/operations"
)

type Interface interface {
	InitServes()
	StartHttp(configs []*config.Config) error
	GetConfig() config.Interface
	GetOperation() operations.Interface
}
