package handler

import (
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
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
