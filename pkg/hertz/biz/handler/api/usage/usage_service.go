// Package usage implements GET /api/usage/*path: a streaming endpoint
// that reports recursive file count and total byte size for a directory
// (or a single file's size). It reuses the shared NDJSON stream writer
// and wire protocol from the archive entries endpoint.
package usage

import (
	"context"
	"errors"
	"strings"

	"files/pkg/common"
	bizhandler "files/pkg/hertz/biz/handler"
	"files/pkg/hertz/biz/handler/stream"
	"files/pkg/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
)

const routePrefix = "/api/usage"

// usageProgress is one NDJSON progress/terminal payload line.
type usageProgress struct {
	Count int64 `json:"count"`
	Size  int64 `json:"size"`
}

// UsageMethod handles GET /api/usage/*path.
//
// Response is NDJSON: zero or more {"count":N,"size":B} progress lines
// while the driver recurses, then a terminal {"_done":true,"count":N,
// "size":B} line. Errors surface as {"_error":"...","code":"..."}. The
// client closing the connection cancels the walk via ctx.
func UsageMethod(ctx context.Context, c *app.RequestContext) {
	if strings.TrimPrefix(string(c.Path()), routePrefix) == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "path invalid"})
		return
	}
	owner := string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "user not found"})
		return
	}

	contextArg, handler, ok := bizhandler.ResolveFileHandler(ctx, c, routePrefix, owner, consts.StatusBadRequest)
	if !ok {
		return
	}

	if !bizhandler.Gate(ctx, c, contextArg.FileParam, models.ActionRead, false, "usage") {
		return
	}

	w := stream.NewWriter(c)
	count, size, err := handler.DirUsage(ctx, contextArg, func(count, size int64) error {
		return w.Emit(usageProgress{Count: count, Size: size})
	})
	if err != nil {
		w.Fail(err, classifyUsageError(err))
		klog.V(2).Infof("[usage] stream error: %v", err)
		return
	}
	w.Done(map[string]any{"count": count, "size": size})
}

// classifyUsageError maps a DirUsage failure to a FE-stable code.
func classifyUsageError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "not found") {
		return "not_found"
	}
	return "internal"
}
