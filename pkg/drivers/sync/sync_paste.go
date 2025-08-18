package sync

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *SyncStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {
	s.paste = pasteParam

	var dstType = s.paste.Dst.FileType

	klog.Infof("Sync - Paste, dst: %s, param: %s", dstType, common.ToJson(pasteParam))

	if dstType == common.Drive {
		return s.copyToDrive()

	} else if dstType == common.External {
		return s.copyToExternal()

	} else if dstType == common.Cache {
		return s.copyToCache()

	} else if dstType == common.Sync {
		return s.copyToSync()

	} else if dstType == common.Cloud {
		return s.copyToCloud()
	}

	return nil, fmt.Errorf("invalid paste dst fileType: %s", dstType)
}

/**
 * ~ copyToDrive
 */
func (s *SyncStorage) copyToDrive() (task *tasks.Task, err error) {
	klog.Info("Sync copyToDrive")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Sync copyToDrive, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromSync); err != nil {
		klog.Errorf("Sync copyToDrive error: %v", err)
	}

	return
}

/**
 * ~ copyToExternal
 */
func (s *SyncStorage) copyToExternal() (task *tasks.Task, err error) {
	klog.Info("Sync copyToExternal")

	var dstNode = s.paste.Dst.Extend

	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Sync copyToExternal, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromSync); err != nil {
		klog.Errorf("Sync copyToExternal error: %v", err)
	}

	return
}

/**
 * ~ copyToCache
 */
func (s *SyncStorage) copyToCache() (task *tasks.Task, err error) {
	klog.Info("Sync copyToCache")

	var dstNode = s.paste.Dst.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Sync copyToCache, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromSync); err != nil {
		klog.Errorf("Sync copyToCache error: %v", err)
	}

	return
}

/**
 * ~ copyToSync
 */
func (s *SyncStorage) copyToSync() (task *tasks.Task, err error) {
	klog.Info("Sync copyToSync")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Sync copyToSync, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.SyncCopy); err != nil {
		klog.Errorf("Sync copyToSync error: %v", err)
	}

	return
}

/**
 * ~ copyToCloud
 */
func (s *SyncStorage) copyToCloud() (task *tasks.Task, err error) {
	klog.Info("Sync copyToCloud")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Sync copyToCloud, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromSync, task.UploadToCloud); err != nil { // ! todo
		klog.Errorf("Sync copyToCloud error: %v", err)
	}

	return

}
