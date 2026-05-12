package handler

import (
	"context"
	"encoding/json"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"fmt"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
)

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
