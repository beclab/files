package workers

import (
	"context"
	"files/pkg/models"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
)

var (
	TaskManager = sync.Map{}
	WorkerPool  pond.ResultPool[string]
)

func init() {
	WorkerPool = pond.NewResultPool[string](1)
}

type Task struct {
	Id   string `json:"id"`
	Type string `json:"type"` // rsync

	Src *models.FileParam
	Dst *models.FileParam

	State string `json:"state"` // running pending failed

	CreateAt time.Time `json:"createAt"`

	Ctx        context.Context    `json:"-"`
	CancelFunc context.CancelFunc `json:"-"`
}
