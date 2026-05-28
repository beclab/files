package posix

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"

	"k8s.io/klog/v2"
)

// Compress is the drive (drive/Home, drive/Data) implementation. The
// drive storage lives on the master node so we route there.
func (s *PosixStorage) Compress(p *models.PasteParam) (*tasks.Task, error) {
	s.paste = p
	klog.Infof("Posix - Compress, owner: %s, dst: %s, srcs: %d",
		p.Owner, common.ToJson(p.Dst), len(p.Srcs))

	if !global.GlobalNode.IsMasterNode(global.CurrentNodeName) {
		return nil, errors.New("posix Compress: not master node")
	}
	if len(p.Srcs) == 0 {
		return nil, errors.New("posix Compress: at least one source required")
	}
	task := tasks.TaskManager.CreateTask(p)
	if err := task.Execute(task.Compress); err != nil {
		klog.Errorf("Posix Compress error: %v", err)
		return nil, err
	}
	return task, nil
}

// Extract is the drive (drive/Home, drive/Data) implementation.
func (s *PosixStorage) Extract(p *models.PasteParam) (*tasks.Task, error) {
	s.paste = p
	klog.Infof("Posix - Extract, owner: %s, src: %s, dst: %s",
		p.Owner, common.ToJson(p.Src), common.ToJson(p.Dst))

	if !global.GlobalNode.IsMasterNode(global.CurrentNodeName) {
		return nil, errors.New("posix Extract: not master node")
	}
	task := tasks.TaskManager.CreateTask(p)
	if err := task.Execute(task.Extract); err != nil {
		klog.Errorf("Posix Extract error: %v", err)
		return nil, err
	}
	return task, nil
}
