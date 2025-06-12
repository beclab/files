package sync

import (
	"files/pkg/drivers/base"
	"files/pkg/utils"
	"net/http"
)

type Service struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

func NewService(param *base.HandlerParam) *Service {
	return &Service{
		ResponseWriter: param.ResponseWriter,
		Request:        param.Request,
	}
}

func (s *Service) Get(u string, method string, data []byte) (any, error) {
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[any](u, method, &header, data)
}
