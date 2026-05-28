package cache

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/tasks"

	"k8s.io/klog/v2"
)

// nodeOfFirstSrc returns the FileParam Extend of the first source in
// the compress request; cache is per-node, so all sources must live on
// the same node and we route to that node.
func nodeOfFirstSrc(p *models.PasteParam) string {
	if len(p.Srcs) == 0 {
		return ""
	}
	return p.Srcs[0].Extend
}

func (s *CacheStorage) Compress(p *models.PasteParam) (*tasks.Task, error) {
	s.paste = p
	klog.Infof("Cache - Compress, owner: %s, dst: %s", p.Owner, common.ToJson(p.Dst))

	srcNode := nodeOfFirstSrc(p)
	if srcNode == "" {
		return nil, errors.New("cache Compress: at least one source required")
	}
	for _, s := range p.Srcs[1:] {
		if s.Extend != srcNode {
			return nil, errors.New("cache Compress: all sources must live on the same node")
		}
	}
	if p.Dst.Extend != srcNode {
		return nil, errors.New("cache Compress: src and dst must live on the same node")
	}
	if srcNode != global.CurrentNodeName {
		return nil, errors.New("cache Compress: not the storage node")
	}

	task := tasks.TaskManager.CreateTask(p)
	if err := task.Execute(task.Compress); err != nil {
		klog.Errorf("Cache Compress error: %v", err)
		return nil, err
	}
	return task, nil
}

func (s *CacheStorage) Extract(p *models.PasteParam) (*tasks.Task, error) {
	s.paste = p
	klog.Infof("Cache - Extract, owner: %s, src: %s, dst: %s",
		p.Owner, common.ToJson(p.Src), common.ToJson(p.Dst))

	if p.Src.Extend != p.Dst.Extend {
		return nil, errors.New("cache Extract: src and dst must live on the same node")
	}
	if p.Src.Extend != global.CurrentNodeName {
		return nil, errors.New("cache Extract: not the storage node")
	}

	task := tasks.TaskManager.CreateTask(p)
	if err := task.Execute(task.Extract); err != nil {
		klog.Errorf("Cache Extract error: %v", err)
		return nil, err
	}
	return task, nil
}
