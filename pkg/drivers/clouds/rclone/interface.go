package rclone

import (
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/drivers/clouds/rclone/serve"
	"files/pkg/models"
)

type Interface interface {
	InitServes()
	StartHttp(configs []*config.Config) error
	FormatFs(param *models.FileParam) (string, error)
	FormatRemote(param *models.FileParam) (string, error)
	GenerateS3EmptyDirectories(srcConfigName, dstConfigName string, srcPath, dstPath, srcName, dstName string) error

	GetConfig() config.Interface
	GetOperation() operations.Interface
	GetServe() serve.Interface
	GetJob() job.Interface
}
