package posix

import (
	"files/pkg/constant"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"files/pkg/workers"
	"fmt"
)

// action: copy / move
func (s *PosixStorage) Paste(pasteParam *models.PasteParam) error {
	var user = pasteParam.Owner
	srcFileParam, err := models.CreateFileParam(user, pasteParam.Source)
	if err != nil {
		return err
	}

	dstFileParam, err := models.CreateFileParam(user, pasteParam.Destination)
	if err != nil {
		return err
	}

	var srcFileType = srcFileParam.FileType

	if srcFileType == constant.Drive {
		s.CopyToDrive(srcFileParam, dstFileParam)
	} else if srcFileType == constant.External {
		s.CopyToExternal(srcFileParam, dstFileParam)
	} else if srcFileType == constant.Cache {
		s.CopyToCache(srcFileParam, dstFileParam)
	} else {
		return fmt.Errorf("error file type: %s", srcFileType)
	}

	return nil
}

func (s *PosixStorage) CopyToDrive(srcParam, dstParam *models.FileParam) {
	var ctx = s.handler.Ctx
	var task = "task-1"
	_ = task
	var command = commands.NewCommand()

	var src = srcParam.FileType

	if src == constant.Drive || src == constant.Cache || src == constant.External {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync)

	} else if src == constant.Sync {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromSync)

	} else if src == constant.Cloud {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromCloud)

	}

}

func (s *PosixStorage) CopyToExternal(srcParam, dstParam *models.FileParam) {
	var ctx = s.handler.Ctx
	var task = "task-1"
	_ = task
	var command = commands.NewCommand()

	var src = srcParam.FileType
	var currentNode = global.CurrentNodeName

	if src == constant.Drive {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync)

	} else if src == constant.Cache || src == constant.External {
		var dstNode = dstParam.Extend
		if dstNode == currentNode {
			workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync)
		} else {
			workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromFiles)
		}

	} else if src == constant.Sync {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromSync)

	} else if src == constant.Cloud {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromCloud)

	}
}

func (s *PosixStorage) CopyToCache(srcParam, dstParam *models.FileParam) {
	var ctx = s.handler.Ctx
	var task = "task-1"
	_ = task
	var command = commands.NewCommand()

	var src = srcParam.FileType
	var currentNode = global.CurrentNodeName

	if src == constant.Drive {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync)

	} else if src == constant.External || src == constant.Cache {
		var dstNode = dstParam.Extend
		if dstNode == currentNode {
			workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync)
		} else {
			workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromFiles)
		}

	} else if src == constant.Sync {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromSync)

	} else if src == constant.Cloud {
		workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromCloud)

	}
}
