package base

import (
	"context"
)

type HandlerParam struct {
	Ctx   context.Context
	Owner string `json:"owner"`
}
