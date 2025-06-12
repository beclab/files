package sync

import (
	"files/pkg/drivers/base"
)

type SyncStorage struct {
	Handler *base.HandlerParam
	Service *Service
}

func NewSyncStorage(handlerParam *base.HandlerParam) base.Execute {
	ss := &SyncStorage{
		Handler: handlerParam,
		Service: NewService(handlerParam),
	}

	return ss
}
