package commands

import (
	"context"
	"files/pkg/models"
)

type CommandInterface interface {
	Rsync() error // include rsync and mv

	UploadToSync() error
	UploadToCloud() error

	DownloadFromFiles() error
	DownloadFromSync() error
	DownloadFromCloud() error

	SyncCopy() error
	CloudCopy() error
}

type Command struct {
	ctx             context.Context
	owner           string
	action          string
	src             *models.FileParam
	dst             *models.FileParam
	Exec            func() error
	UpdateTotalSize func(totalSize int64)
	UpdateProgress  func(progress int, transfer int64)
}

func NewCommand(ctx context.Context, param *models.PasteParam) *Command {
	return &Command{
		ctx:    ctx,
		owner:  param.Owner,
		action: param.Action,
		src:    param.Src,
		dst:    param.Dst,
	}
}
