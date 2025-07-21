package external

import (
	"errors"
	"files/pkg/constant"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/utils"
	"files/pkg/workers"

	"k8s.io/klog/v2"
)

func (s *ExternalStorage) Paste(pasteParam *models.PasteParam) (*workers.Task, error) {
	s.paste = pasteParam

	var dstType = s.paste.Dst.FileType

	klog.Infof("External - Paste, dst: %s, param: %s", dstType, utils.ToJson(pasteParam))

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

func (s *ExternalStorage) copyToDrive() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Error("not src node")
		err = errors.New("copyExternalToDrive: not src node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.Rsync, s.paste)
	if err != nil {
		return
	}
	return
}

func (s *ExternalStorage) copyToExternal() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyExternalToExternal: not dst node")
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

func (s *ExternalStorage) copyToCache() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyExternalToCache: not dst node")
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

func (s *ExternalStorage) copyToSync() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyExternalToSync: not src node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.UploadToSync, s.paste)
	if err != nil {
		return
	}

	return
}

func (s *ExternalStorage) copyToCloud() (task *workers.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyExternalToCloud: not src node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.UploadToCloud, s.paste)
	if err != nil {
		return
	}
	return
}
