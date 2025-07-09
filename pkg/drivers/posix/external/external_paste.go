package external

import (
	"errors"
	"files/pkg/drivers/base"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"files/pkg/workers"

	"k8s.io/klog/v2"
)

type ExternalStorage struct {
	Handler *base.HandlerParam
}

func NewExternalStorage(handler *base.HandlerParam) *ExternalStorage {
	return &ExternalStorage{
		Handler: handler,
	}
}

func (s *ExternalStorage) CopyToDrive(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyExternalToDrive"
	_ = task

	var srcNode = srcParam.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Error("not src node")
		return errors.New("copyExternalToDrive: not src node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync); err != nil {
		return err
	}
	return nil
}

func (s *ExternalStorage) CopyToExternal(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyExternalToExternal"
	_ = task

	var srcNode = srcParam.Extend
	var dstNode = dstParam.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copyExternalToExternal: not dst node")
	}

	var command = commands.NewCommand()

	if srcNode == dstNode {
		if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync); err != nil {
			return err
		}
	} else {
		if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromFiles); err != nil {
			return err
		}
	}

	return nil
}

func (s *ExternalStorage) CopyToCache(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyExternalToCache"
	_ = task

	var srcNode = srcParam.Extend
	var dstNode = dstParam.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copyExternalToCache: not dst node")
	}

	var command = commands.NewCommand()

	if srcNode == dstNode {
		if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync); err != nil {
			return err
		}
	} else {
		if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromFiles); err != nil {
			return err
		}
	}

	return nil
}

func (s *ExternalStorage) CopyToSync(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyExternalToSync"
	_ = task

	var srcNode = srcParam.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		return errors.New("copyExternalToSync: not src node")
	}

	var command = commands.NewCommand()
	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToSync); err != nil {
		return err
	}
	return nil
}

func (s *ExternalStorage) CopyToCloud(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyExternalToCloud"
	_ = task

	var srcNode = srcParam.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		return errors.New("copyExternalToCloud: not src node")
	}

	var command = commands.NewCommand()
	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToCloud); err != nil {
		return err
	}
	return nil
}
