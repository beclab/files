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
	GetFsPrefix(param *models.FileParam) (string, error)

	GetConfig() config.Interface
	GetOperation() operations.Interface
	GetServe() serve.Interface
	GetJob() job.Interface

	GetStat(param *models.FileParam) (*operations.OperationsStat, error)
	GetFilesSize(fileParam *models.FileParam) (int64, error)
	GetFilesList(param *models.FileParam, getPrefix bool) (*operations.OperationsList, error)
	CreateEmptyDirectory(param *models.FileParam) error
	CreateEmptyDirectories(src, target *models.FileParam) error

	Copy(src, dst *models.FileParam) (*operations.OperationsAsyncJobResp, error)
	Delete(param *models.FileParam, dirents []string) ([]string, error)

	Clear(param *models.FileParam) error
	ClearTaskCaches(param *models.FileParam, taskId string) error

	CreatePlaceHolder(dst *models.FileParam) error

	StopJobs() error

	GetMatchedItems(fs string, opt *operations.OperationsOpt, filter *operations.OperationsFilter) (*operations.OperationsList, error)

	FormatFilter(s string, fuzzy bool) []string
}

const (
	ListAll   = 0
	FilesOnly = 1
	DirsOnly  = 2
)
