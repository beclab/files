package sync

import (
	"errors"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"files/pkg/workers"

	"k8s.io/klog/v2"
)

func (s *SyncStorage) CopyToDrive(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copySyncToDrive"
	_ = task

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copySyncToDrive: not master node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromSync); err != nil {
		return err
	}
	return nil
}

func (s *SyncStorage) CopyToExternal(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copySyncToExternal"
	_ = task

	var dstNode = dstParam.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copySyncToExternal: not dst node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromSync); err != nil {
		return err
	}
	return nil
}

func (s *SyncStorage) CopyToCache(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copySyncToCache"
	_ = task

	var dstNode = dstParam.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copySyncToCache: not dst node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromSync); err != nil {
		return err
	}
	return nil
}

func (s *SyncStorage) CopyToSync(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copySyncToSync"
	_ = task

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copySyncToSync: not master node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecSyncCopy); err != nil {
		return err
	}
	return nil
}

func (s *SyncStorage) CopyToCloud(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copySyncToCloud"
	_ = task
	var command = commands.NewCommand()

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copySyncToCloud: not master node")
	}

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromSync); err != nil {
		return err
	}
	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToCloud); err != nil {
		return err
	}

	return nil
}
