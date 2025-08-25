package handle_func

import (
	"context"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
	"mime"
	"strings"
	"time"
)

type rawHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error)

func RawHandler(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {
	return handler.Raw(contextArgs)
}

func RawHandle(ctx context.Context, c *app.RequestContext, _ interface{}, fn rawHandlerFunc, prefix string) []byte {
	contextArg, err := NewHttpContextArgs(ctx, c, prefix, false, false)
	if err != nil {
		klog.Errorf("context args error: %v, path: %s", err, string(c.Path()))
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil
	}

	klog.Infof("[Incoming] raw, user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, c.Method(), common.ToJson(contextArg))

	var handlerParam = &base.HandlerParam{
		Ctx:   ctx,
		Owner: contextArg.FileParam.Owner,
	}

	var rawInline = contextArg.QueryParam.RawInline
	var rawMeta = contextArg.QueryParam.RawMeta
	var fileType = contextArg.FileParam.FileType

	var handler = drivers.Adaptor.NewFileHandler(fileType, handlerParam)
	if handler == nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("handler not found, type: %s", contextArg.FileParam.FileType)})
		return nil
	}

	file, err := fn(handler, contextArg)
	if err != nil {
		klog.Errorf("raw error: %v, user: %s, url: %s", err, contextArg.FileParam.Owner, strings.TrimPrefix(string(c.Path()), prefix))
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return nil
	}

	if rawInline == "true" {
		if rawMeta == "true" {
			c.SetContentType("application/json; charset=utf-8")
		}
		c.Header("Cache-Control", "private")
		c.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{
			"filename": file.FileName,
		}))

	} else {
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
			"filename": file.FileName,
		}))
	}

	if file.Redirect {
		klog.Infof("~~~Debug log: redirect to %s", file.FileName)
		c.Redirect(consts.StatusFound, []byte(file.FileName))
		return nil
	}

	if !file.IsCloud {
		klog.Infof("~~~Debug log: is not cloud")
		c.Header("Content-Disposition", "attachment; filename="+file.FileName)
		c.Header("Last-Modified", file.FileModified.UTC().Format(time.RFC1123))
		ifMatch := string(c.GetHeader("If-Modified-Since"))
		if ifMatch != "" {
			t, _ := time.Parse(time.RFC1123, ifMatch)
			if !file.FileModified.After(t) {
				c.AbortWithStatus(consts.StatusNotModified)
				return nil
			}
		}
		klog.Infof("~~~Debug log: file.FileLength=%d", file.FileLength)
		c.SetBodyStream(file.Reader, int(file.FileLength))
	} else {
		klog.Infof("~~~Debug log: is cloud")
		for k, vs := range file.RespHeader {
			for _, v := range vs {
				c.Header(k, v)
			}
		}

		if rawInline == "true" {
			c.Header("Cache-Control", "private")
			c.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{
				"filename": file.FileName,
			}))
			c.SetContentType(common.MimeTypeByExtension(file.FileName))
		}

		c.SetStatusCode(file.StatusCode)
		c.SetBodyStream(file.ReadCloser, int(file.FileLength))
	}
	return nil
}
