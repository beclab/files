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

/**
 * copyToDrive
 */
func (s *PosixStorage) copyToDrive() (task *tasks.Task, err error) {
	klog.Info("Posix copyToDrive")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Posix copyToDrive, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("Posix copyToDrive error: %v", err)
	}

	return
}

/**
 * copyToExternal
 */
func (s *PosixStorage) copyToExternal() (task *tasks.Task, err error) {
	klog.Info("Posix copyToExternal")

	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Posix copyToExternal, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("Posix copyToExternal error: %v", err)
	}

	return
}

/**
 * copyToCache
 */
func (s *PosixStorage) copyToCache() (task *tasks.Task, err error) {
	klog.Info("Posix copyToCache")

	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Posix copyToCache, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("Posix copyToCache error: %v", err)
	}

	return
}

/**
 * copyToSync
 */
func (s *PosixStorage) copyToSync() (task *tasks.Task, err error) {
	klog.Info("Posix copyToSync")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Posix copyToSync, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.UploadToSync); err != nil {
		klog.Errorf("Posix copyToSync error: %v", err)
	}

	return

}

/**
 * copyToCloud
 */
func (s *PosixStorage) copyToCloud() (task *tasks.Task, err error) {
	klog.Info("Posix copyToCloud")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Posix copyToCloud, not master node")
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.UploadToCloud); err != nil {
		klog.Errorf("Posix copyToCloud error: %v", err)
	}

	return
}
