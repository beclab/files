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
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
)

type treeHandlerFunc func(handler base.Execute, fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error

func TreeHandler(handler base.Execute, fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	return handler.Tree(fileParam, stopChan, dataChan)
}

func TreeHandle(ctx context.Context, c *app.RequestContext, _ interface{}, fn treeHandlerFunc, prefix string) []byte {
	var path = strings.TrimPrefix(string(c.Path()), prefix)
	if path == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "path invalid"})
		return nil
	}
	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "user not found"})
		return nil
	}

	fileParam, err := models.CreateFileParam(owner, path)
	if err != nil {
		klog.Errorf("file param error: %v, owner: %s", err, owner)
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("file param error: %v", err)})
		return nil
	}

	klog.Infof("[Incoming] tree, user: %s, fsType: %s, method: %s, args: %s", owner, fileParam.FileType, c.Method(), fileParam.Json())

	var handlerParam = &base.HandlerParam{
		Ctx:   ctx,
		Owner: owner,
	}

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	var handler = drivers.Adaptor.NewFileHandler(fileParam.FileType, handlerParam)
	if handler == nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("handler not found, type: %s", fileParam.FileType)})
		return nil
	}

	err = fn(handler, fileParam, stopChan, dataChan)
	if err != nil {
		klog.Errorf("tree error: %v, user: %s, url: %s", err, owner, strings.TrimPrefix(string(c.Path()), prefix))
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return nil
	}

	c.SetContentType("text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	klog.Info("Server Got LastEventID", sse.GetLastEventID(&c.Request))
	w := sse.NewWriter(c)
	defer w.Close()
	idNo := 0

	for {
		select {
		case event, ok := <-dataChan:
			if !ok {
				return nil
			}
			err = w.WriteEvent(strconv.Itoa(idNo), "message", []byte(event))
			if err != nil {
				klog.Error(err)
				return nil
			}
			idNo += 1

		case <-ctx.Done():
			close(stopChan)
			return nil
		}
	}
}
