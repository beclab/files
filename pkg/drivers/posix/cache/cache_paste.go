package cache

import (
	"errors"
	"files/pkg/constant"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/workers"

	"k8s.io/klog/v2"
)

func (s *CacheStorage) Paste(pasteParam *models.PasteParam) (*workers.Task, error) {
	s.paste = pasteParam

	var dstType = pasteParam.Dst.FileType

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

	return nil, errors.New("")

}

func (s *CacheStorage) copyToDrive() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyCacheToDrive: not src node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.Rsync, s.paste)
	if err != nil {
		return
	}
	return
}

func (s *CacheStorage) copyToExternal() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyCacheToExternal: not dst node")
		return
	}

	if srcNode == dstNode {
		task, err = workers.SubmitTask(workers.NewTaskId(), workers.Rsync, s.paste)
		if err != nil {
			return
		}
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.DownloadFromFiles, s.paste)
	if err != nil {
		return
	}

	return
}

func (s *CacheStorage) copyToCache() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyCacheToCache: not dst node")
	}

	if srcNode == dstNode {
		task, err = workers.SubmitTask(workers.NewTaskId(), workers.Rsync, s.paste)
		if err != nil {
			return
		}
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.DownloadFromFiles, s.paste)
	if err != nil {
		return
	}

	return
}

func (s *CacheStorage) copyToSync() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyCacheToCache: not src node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.UploadToSync, s.paste)
	if err != nil {
		return
	}

	return
}

func (s *CacheStorage) copyToCloud() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyCacheToCloud: not src node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.UploadToCloud, s.paste)
	if err != nil {
		return
	}
	return
}
