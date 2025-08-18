package clouds

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *CloudStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {

	s.paste = pasteParam

	var dstType = s.paste.Dst.FileType

	klog.Infof("Cloud - Paste, dst: %s, param: %s", dstType, common.ToJson(pasteParam))

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
 * ~ copyToDrive
 */
func (s *CloudStorage) copyToDrive() (task *tasks.Task, err error) {
	klog.Info("Cloud copyToDrive")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Cloud copyToDrive, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromCloud); err != nil {
		klog.Errorf("Cloud copyToDrive error: %v", err)
	}

	return

}

/**
 * ~ copyToExternal
 */
func (s *CloudStorage) copyToExternal() (task *tasks.Task, err error) {
	klog.Info("Cloud copyToExternal")

	var dstNode = s.paste.Dst.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Cloud copyToExternal, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromCloud); err != nil {
		klog.Errorf("Cloud copyToExternal error: %v", err)
	}

	return

}

/**
 * ~ copyToCache
 */
func (s *CloudStorage) copyToCache() (task *tasks.Task, err error) {
	klog.Info("Cloud copyToCache")

	var dstNode = s.paste.Dst.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Cloud copyToCache, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromCloud); err != nil {
		klog.Errorf("Cloud copyToCache error: %v", err)
	}

	return
}

/**
 * ~ copyToSync
 */
func (s *CloudStorage) copyToSync() (task *tasks.Task, err error) {
	klog.Info("Cloud copyToSync")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Cloud copyToSync, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromCloud, task.UploadToSync); err != nil { // ! todo
		klog.Errorf("Cloud copyToSync error: %v", err)
	}

	return
}

/**
 * ~ copyToCloud
 */
func (s *CloudStorage) copyToCloud() (task *tasks.Task, err error) {
	klog.Info("Cloud copyToCloud")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Cloud copyToCloud, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.CopyToCloud); err != nil {
		klog.Errorf("Cloud copyToCloud error: %v", err)
	}

	return
}
