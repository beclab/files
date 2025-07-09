package posix

import (
	"errors"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/paste/commands"
	"files/pkg/workers"

	"k8s.io/klog/v2"
)

func (s *PosixStorage) CopyToDrive(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyDriveToDrive"
	_ = task

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copyDriveToDrive: not master node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync); err != nil {
		return err
	}

	return nil

}

func (s *PosixStorage) CopyToExternal(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyDriveToExternal"
	_ = task
	var command = commands.NewCommand()

	var dstNode = dstParam.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copyDriveToExternal: not dst node")
	}

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync); err != nil {
		return err
	}
	return nil
}

func (s *PosixStorage) CopyToCache(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyDriveToCache"
	_ = task
	var command = commands.NewCommand()

	var dstNode = dstParam.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		return errors.New("copyDriveToCache: not dst node")
	}

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecRsync); err != nil {
		return err
	}
	return nil
}

func (s *PosixStorage) CopyToSync(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyDriveToSync"
	_ = task

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copyDriveToSync: not master node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToSync); err != nil {
		return err
	}
	return nil
}

func (s *PosixStorage) CopyToCloud(srcParam, dstParam *models.FileParam) error {
	var ctx = s.Handler.Ctx
	var task = "copyPosixToCloud"
	_ = task

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		return errors.New("copyPosixToCloud: not master node")
	}

	var command = commands.NewCommand()

	if _, err := workers.NewTask(ctx, srcParam, dstParam, command.ExecUploadToCloud); err != nil {
		return err
	}
	return nil
}
