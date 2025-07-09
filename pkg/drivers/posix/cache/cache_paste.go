package cache

import (
	"errors"
	"files/pkg/drivers/base"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"files/pkg/workers"

	"k8s.io/klog/v2"
)

type CacheStorage struct {
	Handler *base.HandlerParam
}

func NewCacheStorage(handler *base.HandlerParam) *CacheStorage {
	return &CacheStorage{
		Handler: handler,
	}
}

func (s *CacheStorage) CopyToDrive(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCacheToDrive"
	_ = task

	var srcNode = srcParam.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		return errors.New("copyCacheToDrive: not src node")
	}

	var command = commands.NewCommand()
	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync); err != nil {
		return err
	}
	return nil
}

func (s *CacheStorage) CopyToExternal(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCacheToExternal"
	_ = task

	var srcNode = srcParam.Extend
	var dstNode = dstParam.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copyCacheToExternal: not dst node")
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

func (s *CacheStorage) CopyToCache(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCacheToCache"
	_ = task

	var srcNode = srcParam.Extend
	var dstNode = dstParam.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copyCacheToCache: not dst node")
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

func (s *CacheStorage) CopyToSync(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCacheToCache"
	_ = task

	var srcNode = srcParam.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		return errors.New("copyCacheToCache: not src node")
	}

	var command = commands.NewCommand()
	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToSync); err != nil {
		return err
	}

	return nil
}

func (s *CacheStorage) CopyToCloud(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCacheToCloud"
	_ = task

	var srcNode = srcParam.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		return errors.New("copyCacheToCloud: not src node")
	}

	var command = commands.NewCommand()
	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToCloud); err != nil {
		return err
	}

	return nil
}
