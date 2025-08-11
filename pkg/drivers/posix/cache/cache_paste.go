package cache

import (
	"errors"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"
	"files/pkg/utils"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *CacheStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {
	s.paste = pasteParam

	var dstType = pasteParam.Dst.FileType

	klog.Infof("Cache - Paste, dst: %s, param: %s", dstType, utils.ToJson(pasteParam))

	if dstType == utils.Drive {
		return s.copyToDrive()

	} else if dstType == utils.External {
		return s.copyToExternal()

	} else if dstType == utils.Cache {
		return s.copyToCache()

	} else if dstType == utils.Sync {
		return s.copyToSync()

	} else if dstType == utils.AwsS3 || dstType == utils.TencentCos || dstType == utils.GoogleDrive || dstType == utils.DropBox {
		return s.copyToCloud()

	}

	return nil, fmt.Errorf("invalid paste dst fileType: %s", dstType)

}

func (s *CacheStorage) copyToDrive() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyCacheToDrive: not src node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.Rsync, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}

func (s *CacheStorage) copyToExternal() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyCacheToExternal: not dst node")
		return
	}

	if srcNode == dstNode {
		task = tasks.TaskManager.CreateTask(tasks.Rsync, s.paste)
		if err = task.Run(); err != nil {
			return
		}
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromFiles, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}

func (s *CacheStorage) copyToCache() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyCacheToCache: not dst node")
	}

	if srcNode == dstNode {
		task = tasks.TaskManager.CreateTask(tasks.Rsync, s.paste)
		if err = task.Run(); err != nil {
			return
		}
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromFiles, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}

func (s *CacheStorage) copyToSync() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyCacheToCache: not src node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.UploadToSync, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}

func (s *CacheStorage) copyToCloud() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyCacheToCloud: not src node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.UploadToCloud, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}
