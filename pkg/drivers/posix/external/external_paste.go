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

func (s *ExternalStorage) copyToDrive() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Error("not src node")
		err = errors.New("copyExternalToDrive: not src node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.Rsync, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}

func (s *ExternalStorage) copyToExternal() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyExternalToExternal: not dst node")
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

func (s *ExternalStorage) copyToCache() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend
	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyExternalToCache: not dst node")
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

func (s *ExternalStorage) copyToSync() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyExternalToSync: not src node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.UploadToSync, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}

func (s *ExternalStorage) copyToCloud() (task *tasks.Task, err error) {

	var srcNode = s.paste.Src.Extend

	// Route to the src node
	if srcNode != global.CurrentNodeName {
		klog.Errorf("not src node")
		err = errors.New("copyExternalToCloud: not src node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.UploadToCloud, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}
