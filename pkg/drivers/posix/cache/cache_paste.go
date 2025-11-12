package cache

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *CacheStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {
	s.paste = pasteParam

	var dstType = pasteParam.Dst.FileType

	klog.Infof("Cache - Paste, dst: %s, param: %s", dstType, common.ToJson(pasteParam))

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
func (s *CacheStorage) copyToDrive() (task *tasks.Task, err error) {
	klog.Info("Cache copyToDrive")

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("Cache copyToDrive, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("Cache copyToDrive error: %v", err)
	}

	return
}

/**
 * ~ copyToExternal
 */
func (s *CacheStorage) copyToExternal() (task *tasks.Task, err error) {
	klog.Info("Cache copyToExternal")

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Cache copyToExternal, not master node")
		return
	}

	if srcNode == dstNode {
		task = tasks.TaskManager.CreateTask(s.paste)
		if err = task.Execute(task.Rsync); err != nil {
			klog.Errorf("Cache copyToExternal error: %v", err)
		}
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromFiles); err != nil {
		klog.Errorf("Cache copyToExternal error: %v", err)
	}

	return
}

/**
 * ~ copyToCache
 */
func (s *CacheStorage) copyToCache() (task *tasks.Task, err error) {
	klog.Info("Cache copyToCache")

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Cache copyToCache, not master node")
	}

	if srcNode == dstNode {
		task = tasks.TaskManager.CreateTask(s.paste)
		if err = task.Execute(task.Rsync); err != nil {
			klog.Errorf("Cache copyToCache error: %v", err)
		}
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.DownloadFromFiles); err != nil {
		klog.Errorf("Cache copyToCache error: %v", err)
	}

	return
}

/**
 * ~ copyToSync
 */
func (s *CacheStorage) copyToSync() (task *tasks.Task, err error) {
	klog.Info("Cache copyToSync")

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("Cache copyToSync, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.UploadToSync); err != nil {
		klog.Errorf("Posix copyToSync error: %v", err)
	}

	return
}

/**
 * ~ copyToCloud
 */
func (s *CacheStorage) copyToCloud() (task *tasks.Task, err error) {
	klog.Info("Cache copyToCloud")

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("Cache copyToCloud, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.UploadToCloud); err != nil {
		klog.Errorf("Cache copyToCloud error: %v", err)
	}

	return
}
