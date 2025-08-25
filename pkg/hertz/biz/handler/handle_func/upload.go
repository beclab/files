package handle_func

import (
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/hertz/biz/model/upload"
	"files/pkg/models"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
	"strings"
)

/**
 * upload
 */
func FileUploadLinkHandler(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error) {
	return handler.UploadLink(fileUploadArgs)
}

func FileUploadedBytesHandler(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error) {
	return handler.UploadedBytes(fileUploadArgs)
}

func FileUploadChunksHandler(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error) {
	return handler.UploadChunks(fileUploadArgs)
}

type fileUploadHandlerFunc func(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error)

func NewFileUploadArgs(c *app.RequestContext, req interface{}) (*models.FileUploadArgs, error) {
	var fileUploadArgs = &models.FileUploadArgs{
		FileParam: &models.FileParam{},
	}
	var err error

	node := c.Param("node")

	fileUploadArgs.Node = node

	p := ""
	switch req.(type) {
	case upload.UploadLinkReq:
		p = req.(upload.UploadLinkReq).FilePath
		fileUploadArgs.From = req.(upload.UploadLinkReq).From
	case upload.UploadedBytesReq:
		fileUploadArgs.FileName = req.(upload.UploadedBytesReq).FileName
		p = req.(upload.UploadedBytesReq).ParentDir
	case upload.UploadChunksReq:
		fileUploadArgs.UploadId = c.Param("uid")
		fileUploadArgs.ChunkInfo = new(models.ResumableInfo)
		if err = c.BindAndValidate(fileUploadArgs.ChunkInfo); err != nil {
			klog.Warningf("Bind error: %v", err)
			return nil, errors.New("bind error")
		}

		header, err := c.FormFile("file")
		if err != nil {
			klog.Warningf("uploadID:%s, Failed to parse file: %v\n", fileUploadArgs.UploadId, err)
			return nil, errors.New("param invalid")
		}

		fileUploadArgs.ChunkInfo.File = header

		p = fileUploadArgs.ChunkInfo.ParentDir
		if p == "" {
			return nil, errors.New("path invalid")
		}

		fileUploadArgs.Ranges = string(c.GetHeader("Content-Range"))
	}

	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	owner := string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, errors.New("user not found")
	}

	fileUploadArgs.FileParam, err = models.CreateFileParam(owner, p)
	if err != nil {
		return nil, err
	}

	return fileUploadArgs, nil
}

func FileUploadHandle(ctx context.Context, c *app.RequestContext, req interface{}, handleFunc fileUploadHandlerFunc) []byte {
	uploadArg, err := NewFileUploadArgs(c, req)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil
	}
	var handlerParam = &base.HandlerParam{
		Ctx:   ctx,
		Owner: uploadArg.FileParam.Owner,
	}
	var fileHandler = drivers.Adaptor.NewFileHandler(uploadArg.FileParam.FileType, handlerParam)
	if fileHandler == nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("handler not found, type: %s", uploadArg.FileParam.FileType)})
		return nil
	}
	res, err := handleFunc(fileHandler, uploadArg)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{
			"code":    1,
			"data":    res,
			"message": err.Error(),
		})
		return nil
	}
	return res
}
