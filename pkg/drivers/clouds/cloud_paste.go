package clouds

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

func (s *CloudStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {

	s.paste = pasteParam

	var dstType = s.paste.Dst.FileType

	klog.Infof("Cloud - Paste, dst: %s, param: %s", dstType, utils.ToJson(pasteParam))

	if dstType == constant.Drive {
		return s.copyToDrive()

	} else if dstType == constant.External {
		return s.copyToExternal()

	} else if dstType == constant.Cache {
		return s.copyToCache()

	} else if dstType == constant.Sync {
		return s.copyToSync()

	} else if dstType == constant.AwsS3 || dstType == constant.TencentCos || dstType == constant.GoogleDrive || dstType == constant.DropBox {
		return s.copyToCloud()
	}

	return nil, fmt.Errorf("invalid paste dst fileType: %s", dstType)
}

func (s *CloudStorage) copyToDrive() (task *tasks.Task, err error) {
	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyCloudToDrive: not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromCloud, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return

}

func (s *CloudStorage) copyToExternal() (task *tasks.Task, err error) {

	var dstNode = s.paste.Dst.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyCloudToExternal: not dst node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromCloud, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return

}

func (s *CloudStorage) copyToCache() (task *tasks.Task, err error) {

	var dstNode = s.paste.Dst.Extend
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyCloudToCache: not dst node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromCloud, s.paste)
	if err = task.Run(); err != nil {
		return
	}
	return
}

func (s *CloudStorage) copyToSync() (task *tasks.Task, err error) {
	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyCloudToSync: not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.DownloadFromCloud, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}

func (s *CloudStorage) copyToCloud() (task *tasks.Task, err error) {
	klog.Info("Cloud - Paste, copytocloud")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyCloudToCloud: not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(tasks.CloudCopy, s.paste)
	if err = task.Run(); err != nil {
		return
	}

	return
}
