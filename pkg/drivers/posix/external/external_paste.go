package external

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *ExternalStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {
	s.paste = pasteParam

	var dstType = s.paste.Dst.FileType

	klog.Infof("External - Paste, dst: %s, param: %s", dstType, common.ToJson(pasteParam))

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
func (s *ExternalStorage) copyToDrive() (task *tasks.Task, err error) {
	klog.Infof("External copyToDrive, currentnode: %s", global.CurrentNodeName)

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Error("not src node")
		err = errors.New("External copyToDrive, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("External copyToDrive error: %v", err)
	}

	return
}

/**
 * ~ copyToExternal
 */
func (s *ExternalStorage) copyToExternal() (task *tasks.Task, err error) {
	klog.Infof("External copyToExternal, currentnode: %s", global.CurrentNodeName)

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("External copyToExternal, not master node")
		return
	}

	if srcNode == dstNode {
		task = tasks.TaskManager.CreateTask(s.paste)
		if err = task.Execute(task.Rsync); err != nil {
			klog.Errorf("External copyToExternal error: %v", err)
		}
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromFiles); err != nil {
		klog.Errorf("External copyToExternal error: %v", err)
	}

	return
}

/**
 * ~ copyToCache
 */
func (s *ExternalStorage) copyToCache() (task *tasks.Task, err error) {
	klog.Infof("External copyToCache, currentnode: %s", global.CurrentNodeName)

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("External copyToCache, not master node")
		return
	}

	if srcNode == dstNode {
		task = tasks.TaskManager.CreateTask(s.paste)
		if err = task.Execute(task.Rsync); err != nil {
			klog.Errorf("External copyToCache error: %v", err)
		}
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromFiles); err != nil {
		klog.Errorf("External copyToCache error: %v", err)
	}

	return
}

/**
 * ~ copyToSync
 */
func (s *ExternalStorage) copyToSync() (task *tasks.Task, err error) {
	klog.Info("External copyToSync")

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("External copyToSync, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.UploadToSync); err != nil {
		klog.Errorf("External copyToCloud error: %v", err)
	}

	return
}

/**
 * ~ copyToCloud
 */
func (s *ExternalStorage) copyToCloud() (task *tasks.Task, err error) {
	klog.Info("External copyToCloud")

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("External copyToCloud, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.UploadToCloud); err != nil {
		klog.Errorf("External copyToCloud error: %v", err)
	}

	return
}
