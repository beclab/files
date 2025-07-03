package base

import (
	"context"
	"files/pkg/common"
	"net/http"
)

type HandlerParam struct {
	Ctx            context.Context
	Owner          string `json:"owner"`
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Data           *common.Data
}
