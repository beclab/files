package sync

import (
	"errors"
	"files/pkg/constant"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"
	"files/pkg/utils"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *SyncStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {
	s.paste = pasteParam

	var dstType = s.paste.Dst.FileType

	klog.Infof("Sync - Paste, dst: %s, param: %s", dstType, utils.ToJson(pasteParam))

	if dstType == constant.Drive {
		return s.copyToDrive()

	} else if dstType == constant.External {
		return s.copyToExternal()

	} else if dstType == constant.Cache {
		return s.copyToCache()

	} else if dstType == constant.Sync {
		return s.copyToSync()

	} else if dstType == constant.Cloud {
		return s.copyToCloud()
	}

	return nil, fmt.Errorf("invalid paste dst fileType: %s", dstType)
}

func (s *SyncStorage) copyToDrive() (task *tasks.Task, err error) {

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copySyncToDrive: not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromSync, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}

func (s *SyncStorage) copyToExternal() (task *tasks.Task, err error) {

	var dstNode = s.paste.Dst.Extend

	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copySyncToExternal: not dst node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromSync, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}

func (s *SyncStorage) copyToCache() (task *tasks.Task, err error) {

	var dstNode = s.paste.Dst.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copySyncToCache: not dst node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromSync, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}

func (s *SyncStorage) copyToSync() (task *tasks.Task, err error) {

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copySyncToSync: not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.SyncCopy, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}

func (s *SyncStorage) copyToCloud() (task *tasks.Task, err error) {

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copySyncToCloud: not master node")
		return
	}

	// DownloadFromSync + UploadToCloud
	task = tasks.TaskManager.CreateTask(tasks.DownloadFromSync, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return

}
