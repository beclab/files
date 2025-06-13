package sync

import (
	"files/pkg/common"
	"files/pkg/drivers/base"
	"net/http"
)

type SyncStorage struct {
	Base    *base.BaseStorage
	Service *Service
}

func NewSyncStorage(w http.ResponseWriter, r *http.Request, d *common.Data) base.Execute {
	ss := &SyncStorage{
		Base:    base.NewBaseStorage(w, r, d),
		Service: NewService(w, r),
	}

	return ss
}
