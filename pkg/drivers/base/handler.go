package base

import (
	"context"
	"net/http"
)

type HandlerParam struct {
	Ctx            context.Context
	Owner          string `json:"owner"`
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}
