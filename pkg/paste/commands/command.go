package commands

import "files/pkg/models"

type CommandInterface interface {
	Rsync() error
	Move() error

	UploadToSync() error
	UploadToCloud() error

	DownloadFromFiles() error
	DownloadFromSync() error
	DownloadFromCloud() error

	SyncCopy() error
	CloudCopy() error
}

type command struct {
	owner  string
	action string
	src    *models.FileParam
	dst    *models.FileParam
}

func NewCommand(param *models.PasteParam) CommandInterface {
	return &command{
		owner:  param.Owner,
		action: param.Action,
		src:    param.Src,
		dst:    param.Dst,
	}
}
