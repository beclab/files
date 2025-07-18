package posix

import (
	"errors"
	"files/pkg/constant"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/workers"

	"k8s.io/klog/v2"
)

func (s *PosixStorage) Paste(pasteParam *models.PasteParam) (*workers.Task, error) {

	s.paste = pasteParam

	var dstType = pasteParam.Dst.FileType

	if dstType == constant.Drive || dstType == constant.External || dstType == constant.Cache {
		// todo check disk space
	}

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

func (s *PosixStorage) copyToDrive() (task *workers.Task, err error) {

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyDriveToDrive: not master node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.Rsync, s.paste)
	if err != nil {
		return
	}
	return
}

func (s *PosixStorage) copyToExternal() (task *workers.Task, err error) {

	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyDriveToExternal: not dst node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.Rsync, s.paste)
	if err != nil {
		return
	}
	return
}

func (s *PosixStorage) copyToCache() (task *workers.Task, err error) {

	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("copyDriveToCache: not dst node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.Rsync, s.paste)
	if err != nil {
		return
	}
	return
}

func (s *PosixStorage) copyToSync() (task *workers.Task, err error) {

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyDriveToSync: not master node")
		return
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.UploadToSync, s.paste)
	if err != nil {
		return
	}
	return

}

func (s *PosixStorage) copyToCloud() (task *workers.Task, err error) {

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("copyPosixToCloud: not master node")
	}

	task, err = workers.SubmitTask(workers.NewTaskId(), workers.UploadToCloud, s.paste)
	if err != nil {
		return
	}
	return
}
