package clouds

import (
	"errors"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"files/pkg/workers"

	"k8s.io/klog/v2"
)

func (s *CloudStorage) CopyToDrive(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCloudToDrive"
	_ = task

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copyCloudToDrive: not master node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromCloud); err != nil {
		klog.Errorf("task copyCloudToDrive error: %v", err)
		return err
	}

	return nil
}

func (s *CloudStorage) CopyToExternal(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCloudToExternal"
	_ = task

	var dstNode = dstParam.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copyCloudToExternal: not dst node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromCloud); err != nil {
		return err
	}
	return nil
}

func (s *CloudStorage) CopyToCache(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCloudToCache"
	_ = task

	var dstNode = dstParam.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copyCloudToCache: not dst node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromCloud); err != nil {
		return err
	}
	return nil
}

func (s *CloudStorage) CopyToSync(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCloudToSync"
	_ = task

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copyCloudToSync: not master node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromCloud); err != nil {
		return err
	}
	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToSync); err != nil {
		return err
	}

	return nil
}

func (s *CloudStorage) CopyToCloud(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyCloudToCloud"
	_ = task
	var command = commands.NewCommand()

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copyCloudToCloud: not master node")
	}

	var srcFileType = srcParam.FileType
	var srcCloudAccount = srcParam.Extend

	var dstFileType = dstParam.FileType
	var dstCloudAccount = dstParam.Extend

	if srcFileType == dstFileType && srcCloudAccount == dstCloudAccount {
		// same cloud
		if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecCloudCopy); err != nil {
			return err
		}
	} else {
		// different cloud
		if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecDownloadFromFiles); err != nil {
			return err
		}
		if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToCloud); err != nil {
			return err
		}
	}

	return nil
}
