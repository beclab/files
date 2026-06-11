package handler

import (
	"context"
	"encoding/json"
	"files/pkg/access"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
)

// NodeGuard rejects requests whose :node segment is not a known cluster node.
func NodeGuard() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		node := c.Param("node")
		if node != global.CurrentNodeName && !global.GlobalNode.CheckNodeExists(node) {
			c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "invalid node"})
			return
		}
		c.Next(ctx)
	}
}

func RespSuccess(c *app.RequestContext, data interface{}) {
	c.JSON(http.StatusOK, utils.H{"code": 0, "data": data})
	c.Abort()
}

func RespError(c *app.RequestContext, msg string) {
	c.JSON(http.StatusOK, utils.H{"code": 1, "message": msg})
	c.Abort()
}

func RespErrorExpired(c *app.RequestContext, code int, msg string, expire int64) {
	c.JSON(code, utils.H{"code": 1, "message": msg, "expire": expire})
	c.Abort()
}

func RespBadRequest(c *app.RequestContext, msg string) {
	c.JSON(http.StatusBadRequest, utils.H{"code": 1, "message": msg})
	c.Abort()
}

func RespForbidden(c *app.RequestContext, msg string) {
	c.JSON(http.StatusForbidden, utils.H{"code": 1, "message": msg})
	c.Abort()
}

func RespStatusInternalServerError(c *app.RequestContext, msg string) {
	c.JSON(http.StatusInternalServerError, utils.H{"code": 1, "message": msg})
	c.Abort()
}

func DecodeResponse(c *app.RequestContext, data []byte, dst interface{}) bool {
	if err := json.Unmarshal(data, dst); err != nil {
		klog.Errorf("Failed to unmarshal response body: %v", err)
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "Failed to unmarshal response body"})
		return false
	}
	return true
}

// RequireInternalShareToken verifies the per-process internal share token
// that the share proxy attaches to its loopback forwards (share=1). It
// aborts with 403 and returns false on mismatch, so a direct share=1 call
// lacking the token is rejected rather than bypassing authorization. tag
// identifies the caller in the denial log.
func RequireInternalShareToken(c *app.RequestContext, tag string) bool {
	if common.EqualInternalShareToken(string(c.GetHeader(common.HeaderInternalShareToken))) {
		return true
	}
	klog.Warningf("[%s] share=1 without valid internal token, path: %s", tag, string(c.Path()))
	c.AbortWithStatusJSON(consts.StatusForbidden, utils.H{"error": common.ErrorMessagePermissionDenied})
	return false
}

// Gate is the unified authorization check for the file handlers. It
// resolves fp's owner Level via access.CheckAccessParam and aborts with a
// 403 when the level does not permit action; tag identifies the caller in
// the denial log. When skipShare is true a request carrying the share=1
// query is allowed through: those paths have already been vetted by the
// share middleware and are intentionally not routed through CheckAccess.
// The share=1 query is produced only by the share proxy loopback, so it
// is trusted only when accompanied by the process-local internal token;
// a forged share=1 without the token falls through to a real 403 rather
// than bypassing CheckAccess. Returns true if the request may proceed.
func Gate(ctx context.Context, c *app.RequestContext, fp *models.FileParam, action models.Action, skipShare bool, tag string) bool {
	if skipShare && string(c.Query("share")) == "1" {
		return RequireInternalShareToken(c, tag)
	}
	lvl, err := access.CheckAccessParam(ctx, fp.Owner, fp)
	if err != nil || !lvl.Allow(action) {
		klog.Warningf("[%s] permission denied: owner=%s, type=%s, extend=%s, path=%s, action=%d, level=%v, err=%v",
			tag, fp.Owner, fp.FileType, fp.Extend, fp.Path, action, lvl, err)
		c.AbortWithStatusJSON(consts.StatusForbidden, utils.H{"error": common.ErrorMessagePermissionDenied})
		return false
	}
	return true
}

func ResolveFileHandler(ctx context.Context, c *app.RequestContext, routePrefix string, owner string, missingHandlerStatus int) (*models.HttpContextArgs, base.Execute, bool) {
	contextArg, err := models.NewHttpContextArgs(ctx, c, routePrefix, false, false)
	if err != nil {
		klog.Errorf("context args error: %v, path: %s", err, string(c.Path()))
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil, nil, false
	}

	if owner == "" {
		owner = contextArg.FileParam.Owner
	}
	handlerParam := &base.HandlerParam{
		Ctx:   ctx,
		Owner: owner,
	}

	fileHandler := drivers.Adaptor.NewFileHandler(contextArg.FileParam.FileType, handlerParam)
	if fileHandler == nil {
		c.AbortWithStatusJSON(missingHandlerStatus, utils.H{"error": fmt.Sprintf("handler not found, type: %s", contextArg.FileParam.FileType)})
		return nil, nil, false
	}

	return contextArg, fileHandler, true
}
