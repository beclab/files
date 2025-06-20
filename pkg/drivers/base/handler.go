package base

import (
	"files/pkg/common"
	"net/http"
)

type HandlerParam struct {
	Owner          string `json:"owner"`
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Data           *common.Data
}
