package base

import (
	"files/pkg/common"
	"files/pkg/drivers/interfaces"
	"net/http"
)

var _ interfaces.Execute = &BaseStorage{}

type BaseStorage struct {
	Owner          string
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Data           *common.Data
}

func NewBaseStorage(w http.ResponseWriter, r *http.Request, d *common.Data) *BaseStorage {
	return &BaseStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
		Data:           d,
	}
}

func (s *BaseStorage) List() (int, error) {
	panic("not implemented")
}
