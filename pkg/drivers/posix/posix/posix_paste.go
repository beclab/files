package posix

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *PosixStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {
	s.paste = pasteParam

	var dstType = pasteParam.Dst.FileType

	klog.Infof("Posix - Paste, dst: %s, param: %s", dstType, common.ToJson(pasteParam))

	if dstType == common.Drive || dstType == common.External || dstType == common.Cache {
		// todo check disk space
	}

	if dstType == common.Drive {
		return s.copyToDrive()

	} else if dstType == common.External {
		return s.copyToExternal()

	} else if dstType == common.Cache {
		return s.copyToCache()

	} else if dstType == common.Sync {
		return s.copyToSync()

	} else if dstType == common.AwsS3 || dstType == common.TencentCos || dstType == common.GoogleDrive || dstType == common.DropBox {
		return s.copyToCloud()
	}

	return nil, fmt.Errorf("invalid paste dst fileType: %s", dstType)
}

func (s *PosixStorage) copyToDrive() (task *tasks.Task, err error) {
	klog.Info("Paste - Paste, copytodrive")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyDriveToDrive: not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.Rsync, s.paste)
	if err = task.Run(); err != nil {
		klog.Errorf("Posix - Paste, copytodrive run task error: %v", err)
		return
	}

	return
}

func (s *PosixStorage) copyToExternal() (task *tasks.Task, err error) {

	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyDriveToExternal: not dst node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.Rsync, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}

func (s *PosixStorage) copyToCache() (task *tasks.Task, err error) {

	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyDriveToCache: not dst node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.Rsync, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}

func (s *PosixStorage) copyToSync() (task *tasks.Task, err error) {

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyDriveToSync: not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.UploadToSync, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return

}

func (s *PosixStorage) copyToCloud() (task *tasks.Task, err error) {
	klog.Info("Posix - Paste, copytocloud")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyPosixToCloud: not master node")
	}

	task = tasks.TaskManager.CreateTask(tasks.UploadToCloud, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}
