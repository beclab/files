package handle_func

import (
	"bytes"
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
	"strings"
	"time"
)

type previewHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error)

func PreviewHandler(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	return handler.Preview(contextArgs)
}

func PreviewHandle(ctx context.Context, c *app.RequestContext, _ interface{}, fn previewHandlerFunc, prefix string) []byte {
	contextArg, err := NewHttpContextArgs(ctx, c, prefix, false, false)
	if err != nil {
		klog.Errorf("context args error: %v, path: %s", err, string(c.Path()))
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil
	}

	klog.Infof("[Incoming] preview, user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, c.Method(), common.ToJson(contextArg))

	var handlerParam = &base.HandlerParam{
		Ctx:   ctx,
		Owner: contextArg.FileParam.Owner,
	}

	var fileType = contextArg.FileParam.FileType
	var handler = drivers.Adaptor.NewFileHandler(fileType, handlerParam)

	if handler == nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("handler not found, type: %s", contextArg.FileParam.FileType)})
		return nil
	}

	if contextArg.FileParam.FileType == common.AwsS3 ||
		contextArg.FileParam.FileType == common.DropBox ||
		contextArg.FileParam.FileType == common.GoogleDrive {
		if contextArg.QueryParam.PreviewSize == "thumb" {
			c.SetStatusCode(consts.StatusOK)
			return nil
		}
	}

	fileData, err := fn(handler, contextArg)
	if err != nil {
		klog.Errorf("preview error: %v, user: %s, url: %s", err, contextArg.FileParam.Owner, strings.TrimPrefix(string(c.Path()), prefix))
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return nil
	}

	c.Header("Content-Disposition", "inline")

	if !fileData.IsCloud {
		c.Header("Content-Disposition", "attachment; filename="+fileData.FileName)
		c.Header("Last-Modified", fileData.FileModified.UTC().Format(time.RFC1123))
		ifMatch := string(c.GetHeader("If-Modified-Since"))
		if ifMatch != "" {
			t, _ := time.Parse(time.RFC1123, ifMatch)
			if !fileData.FileModified.After(t) {
				c.AbortWithStatus(consts.StatusNotModified)
				return nil
			}
		}
		klog.Infof("~~~Debug log: file.FileLength=%d", len(fileData.Data))
		c.SetBodyStream(bytes.NewReader(fileData.Data), len(fileData.Data))
	} else {
		for k, vs := range fileData.RespHeader {
			for _, v := range vs {
				c.Header(k, v)
			}
		}
		c.SetStatusCode(fileData.StatusCode)
		c.SetBodyStream(bytes.NewReader(fileData.Data), len(fileData.Data))
	}
	return nil
}
