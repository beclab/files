package sync

import (
	"files/pkg/constant"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"files/pkg/workers"
)

func (s *SyncStorage) Paste(pasteParam *models.PasteParam) error {
	var user = pasteParam.Owner
	srcFileParam, err := models.CreateFileParam(user, pasteParam.Source)
	if err != nil {
		return err
	}

	dstFileParam, err := models.CreateFileParam(user, pasteParam.Destination)
	if err != nil {
		return err
	}

	s.CopyToSync(srcFileParam, dstFileParam)

	return nil

}

func (s *SyncStorage) CopyToSync(srcParam, dstParam *models.FileParam) {
	var ctx = s.handler.Ctx
	var task = "task-1"
	_ = task
	var command = commands.NewCommand()

	var src = srcParam.FileType

	if src == constant.Drive || src == constant.Cache || src == constant.External {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToSync)

	} else if src == constant.Sync {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecSyncCopy)

	} else if src == constant.Cloud {
		// todo add two task in a group
		workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromCloud)
		workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToSync)

	}
}
