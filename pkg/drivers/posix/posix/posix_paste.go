package posix

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"
	"fmt"

	"k8s.io/klog/v2"
)

func (s *PosixStorage) Paste(pasteParam *models.PasteParam) (*tasks.Task, error) {
	s.paste = pasteParam

	var dstType = pasteParam.Dst.FileType

	klog.Infof("Posix - Paste, dst: %s, param: %s", dstType, common.ToJson(pasteParam))

	// drive/Common is a cluster-wide RWX volume; reachable from any
	// node, so its own paste path skips the master-only routing
	// that copyToDrive enforces for Home/Data.
	if pasteParam.Dst.IsDriveCommon() {
		return s.copyToCommon()
	}

	// src=drive/Common: any node can read /appcommon. Pulling the
	// dispatch out so we don't carry master-only assumptions into
	// copyToCloud / copyToSync. dst=drive/Common is already handled
	// by the branch above.
	if pasteParam.Src.IsDriveCommon() {
		return s.pasteFromCommon()
	}

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
 * pasteFromCommon — src is drive/Common, dst is anything except drive/Common
 * (which is already routed to copyToCommon).
 *
 * /appcommon is RWX and visible on every node so reads don't require
 * master. Where the dst-side legacy logic does its own node routing
 * (cache/external -> dst node) we reuse it; for cloud/sync we build
 * the task inline so the existing master-only guards in copyToCloud
 * and copyToSync don't trip on us.
 */
func (s *PosixStorage) pasteFromCommon() (*tasks.Task, error) {
	var dstType = s.paste.Dst.FileType

	klog.Infof("Posix pasteFromCommon, dst: %s", dstType)

	switch dstType {
	case common.Drive:
		// dst is Home/Data (Common was diverted above); writing
		// userspace_pvc still requires master.
		return s.copyToDrive()
	case common.Cache:
		// Existing path: route to dst node, rsync; dst node has
		// /appcommon mounted for the read side.
		return s.copyToCache()
	case common.External:
		return s.copyToExternal()
	case common.Sync:
		task := tasks.TaskManager.CreateTask(s.paste)
		if err := task.Execute(task.UploadToSync); err != nil {
			klog.Errorf("Posix pasteFromCommon (sync) error: %v", err)
			return task, err
		}
		return task, nil
	case common.AwsS3, common.TencentCos, common.GoogleDrive, common.DropBox:
		task := tasks.TaskManager.CreateTask(s.paste)
		if err := task.Execute(task.UploadToCloud); err != nil {
			klog.Errorf("Posix pasteFromCommon (cloud) error: %v", err)
			return task, err
		}
		return task, nil
	}

	return nil, fmt.Errorf("invalid paste dst fileType: %s", dstType)
}

/**
 * copyToCommon — src is drive (Home/Data/Common), dst is drive/Common.
 *
 * When src is Home/Data the read still needs userspace_pvc (master-only).
 * When src is Common both sides are on /appcommon (RWX) and any node works.
 * Either way the copy itself is a local rsync.
 */
func (s *PosixStorage) copyToCommon() (task *tasks.Task, err error) {
	klog.Info("Posix copyToCommon")

	if !s.paste.Src.IsDriveCommon() {
		var currentNodeName = global.CurrentNodeName
		if !global.GlobalNode.IsMasterNode(currentNodeName) {
			klog.Error("not master node")
			err = errors.New("Posix copyToCommon, not master node")
			return
		}
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("Posix copyToCommon error: %v", err)
	}

	return
}

/**
 * copyToDrive
 */
func (s *PosixStorage) copyToDrive() (task *tasks.Task, err error) {
	klog.Info("Posix copyToDrive")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Posix copyToDrive, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("Posix copyToDrive error: %v", err)
	}

	return
}

/**
 * copyToExternal
 */
func (s *PosixStorage) copyToExternal() (task *tasks.Task, err error) {
	klog.Info("Posix copyToExternal")

	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Posix copyToExternal, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("Posix copyToExternal error: %v", err)
	}

	return
}

/**
 * copyToCache
 */
func (s *PosixStorage) copyToCache() (task *tasks.Task, err error) {
	klog.Info("Posix copyToCache")

	var dstNode = s.paste.Dst.Extend

	// Route to the dst node
	if dstNode != global.CurrentNodeName {
		klog.Errorf("not dst node")
		err = errors.New("Posix copyToCache, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.Rsync); err != nil {
		klog.Errorf("Posix copyToCache error: %v", err)
	}

	return
}

/**
 * copyToSync
 */
func (s *PosixStorage) copyToSync() (task *tasks.Task, err error) {
	klog.Info("Posix copyToSync")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Posix copyToSync, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.UploadToSync); err != nil {
		klog.Errorf("Posix copyToSync error: %v", err)
	}

	return

}

/**
 * copyToCloud
 */
func (s *PosixStorage) copyToCloud() (task *tasks.Task, err error) {
	klog.Info("Posix copyToCloud")

	var currentNodeName = global.CurrentNodeName
	var isCurrentNodeMaster = global.GlobalNode.IsMasterNode(currentNodeName)

	// Bail out before creating a task if we aren't the master node.
	// (Previously the err was set then immediately overwritten by
	// task.Execute below, so the "not master node" error was never
	// surfaced to the caller.)
	if !isCurrentNodeMaster {
		klog.Error("not master node")
		err = errors.New("Posix copyToCloud, not master node")
		return
	}

	task = tasks.TaskManager.CreateTask(s.paste)
	if err = task.Execute(task.UploadToCloud); err != nil {
		klog.Errorf("Posix copyToCloud error: %v", err)
	}

	return
}
