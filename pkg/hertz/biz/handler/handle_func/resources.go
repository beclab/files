package handle_func

import (
	"context"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/hertz/biz/model/api/resources"
	"files/pkg/models"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"strings"

	"k8s.io/klog/v2"
)

func NewHttpContextArgs(ctx context.Context, c *app.RequestContext, prefix string, enableThumbnails bool, resizePreview bool) (*models.HttpContextArgs, error) {
	var p = string(c.Path())
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, errors.New("path invalid")
	}

	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return nil, err
	}

	var queryParam = CreateQueryParam(owner, ctx, c, enableThumbnails, resizePreview)

	return &models.HttpContextArgs{
		FileParam:  fileParam,
		QueryParam: queryParam,
	}, nil
}

/**
 * list
 * create
 * rename
 */

type fileHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error)

func ListHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.List(contextArgs)
}

func CreateHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.Create(contextArgs)
}

func RenameHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.Rename(contextArgs)
}

func FileHandle(ctx context.Context, c *app.RequestContext, _ interface{}, fn fileHandlerFunc, prefix string) []byte {
	contextArg, err := NewHttpContextArgs(ctx, c, prefix, false, false)
	if err != nil {
		klog.Errorf("context args error: %v, path: %s", err, string(c.Path()))
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil
	}

	klog.Infof("[Incoming-Resource] user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, c.Method(), common.ToJson(contextArg))

	var handlerParam = &base.HandlerParam{
		Ctx:   ctx,
		Owner: contextArg.FileParam.Owner,
	}

	var handler = drivers.Adaptor.NewFileHandler(contextArg.FileParam.FileType, handlerParam)
	if handler == nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("handler not found, type: %s", contextArg.FileParam.FileType)})
		return nil
	}

	res, err := fn(handler, contextArg)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return nil
	}

	return res
}

/**
 * edit
 */

type fileEditHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error)

func EditHandler(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error) {
	return handler.Edit(contextArgs)
}

func FileEditHandle(ctx context.Context, c *app.RequestContext, _ interface{}, fn fileEditHandlerFunc, prefix string) []byte {
	contextArg, err := NewHttpContextArgs(ctx, c, prefix, false, false)
	if err != nil {
		klog.Errorf("context args error: %v, path: %s", err, string(c.Path()))
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil
	}

	klog.Infof("[Incoming-Resource] user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, c.Method(), common.ToJson(contextArg))

	var handlerParam = &base.HandlerParam{
		Ctx:   ctx,
		Owner: contextArg.FileParam.Owner,
	}

	var handler = drivers.Adaptor.NewFileHandler(contextArg.FileParam.FileType, handlerParam)
	if handler == nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("handler not found, type: %s", contextArg.FileParam.FileType)})
		return nil
	}

	res, err := fn(handler, contextArg)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return nil
	}

	c.Header("Etag", res.Etag)
	return nil
}

/**
 * delete
 */

func NewFileDeleteArgs(_ context.Context, c *app.RequestContext, req resources.DeleteResourcesReq, prefix string) (*models.FileDeleteArgs, error) {
	var fileDeleteArgs = &models.FileDeleteArgs{
		FileParam: &models.FileParam{},
	}
	var err error

	var p = string(c.Path())
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, errors.New("path invalid")
	}

	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, errors.New("user not found")
	}

	fileDeleteArgs.FileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return nil, err
	}

	fileDeleteArgs.Dirents = req.Dirents

	return fileDeleteArgs, nil
}

type fileDeleteHandlerFunc func(handler base.Execute, fileDeleteArgs *models.FileDeleteArgs) ([]byte, error)

func DeleteHandler(handler base.Execute, fileDeleteArgs *models.FileDeleteArgs) ([]byte, error) {
	return handler.Delete(fileDeleteArgs)
}

func FileDeleteHandle(ctx context.Context, c *app.RequestContext, req interface{}, fn fileDeleteHandlerFunc, prefix string) []byte {
	deleteArg, err := NewFileDeleteArgs(ctx, c, req.(resources.DeleteResourcesReq), prefix)
	if err != nil {
		klog.Errorf("delete args error: %v, path: %s", err, string(c.Path()))
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil
	}

	klog.Infof("[Incoming-Resource] user: %s, fsType: %s, method: %s, args: %s", deleteArg.FileParam.Owner, deleteArg.FileParam.FileType, c.Method(), common.ToJson(deleteArg))

	var handlerParam = &base.HandlerParam{
		Ctx:   ctx,
		Owner: deleteArg.FileParam.Owner,
	}
	var handler = drivers.Adaptor.NewFileHandler(deleteArg.FileParam.FileType, handlerParam)
	if handler == nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("handler not found, type: %s", deleteArg.FileParam.FileType)})
		return nil
	}

	res, err := fn(handler, deleteArg)
	if err != nil {
		var deleteFailedPaths []string
		if res != nil {
			json.Unmarshal(res, &deleteFailedPaths)
		}
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{
			"code":    1,
			"data":    deleteFailedPaths,
			"message": err.Error(),
		})
		return nil
	}

	return res
}
