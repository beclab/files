package external

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"

	"k8s.io/klog/v2"
)

func (s *ExternalStorage) Compress(p *models.PasteParam) (*tasks.Task, error) {
	s.paste = p
	klog.Infof("External - Compress, owner: %s, dst: %s", p.Owner, common.ToJson(p.Dst))

	if len(p.Srcs) == 0 {
		return nil, errors.New("external Compress: at least one source required")
	}
	srcNode := p.Srcs[0].Extend
	for _, sr := range p.Srcs[1:] {
		if sr.Extend != srcNode {
			return nil, errors.New("external Compress: all sources must live on the same node")
		}
	}
	if p.Dst.Extend != srcNode {
		return nil, errors.New("external Compress: src and dst must live on the same node")
	}
	if srcNode != global.CurrentNodeName {
		return nil, errors.New("external Compress: not the storage node")
	}

	task := tasks.TaskManager.CreateTask(p)
	if err := task.Execute(task.Compress); err != nil {
		klog.Errorf("External Compress error: %v", err)
		return nil, err
	}
	return task, nil
}

func (s *ExternalStorage) Extract(p *models.PasteParam) (*tasks.Task, error) {
	s.paste = p
	klog.Infof("External - Extract, owner: %s, src: %s, dst: %s",
		p.Owner, common.ToJson(p.Src), common.ToJson(p.Dst))

	if p.Src.Extend != p.Dst.Extend {
		return nil, errors.New("external Extract: src and dst must live on the same node")
	}
	if p.Src.Extend != global.CurrentNodeName {
		return nil, errors.New("external Extract: not the storage node")
	}

	task := tasks.TaskManager.CreateTask(p)
	if err := task.Execute(task.Extract); err != nil {
		klog.Errorf("External Extract error: %v", err)
		return nil, err
	}
	return task, nil
}
