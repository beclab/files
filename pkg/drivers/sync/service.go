package sync

import (
	"files/pkg/utils"
	"net/http"
)

type Service struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

func NewService(w http.ResponseWriter, r *http.Request) *Service {
	return &Service{
		ResponseWriter: w,
		Request:        r,
	}
}

func (s *Service) Get(u string, method string, data []byte) (any, error) {
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[any](u, method, &header, data)
}
