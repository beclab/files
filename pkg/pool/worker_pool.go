package pool

import (
	"github.com/alitto/pond/v2"
	"sync"
)

var (
	TaskManager = sync.Map{} // 存储任务状态
	WorkerPool  pond.Pool
)

type Task struct {
	ID       string
	Source   string
	Dest     string
	Status   string
	Progress int
	Log      []string
}
