package upload

import (
	"context"
	"errors"
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
)

func FileUploadLinkHandler(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error) {
	return handler.UploadLink(fileUploadArgs)
}

type fileUploadHandlerFunc func(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error)

func NewFileUploadArgs(c *app.RequestContext) (*models.FileUploadArgs, error) {
	var fileUploadArgs = &models.FileUploadArgs{
		FileParam: &models.FileParam{},
	}
	var err error

	node := strings.TrimSuffix(c.Param("node"), "/")

	fileUploadArgs.Node = node

	p := c.Query("file_path")
	if p == "" {
		p = c.Query("parent_dir")
		if p == "" {
			fileUploadArgs.UploadId = c.Param("uid")

			fileUploadArgs.ChunkInfo = &models.ResumableInfo{}
			if c.PostForm("parent_dir") == "" {
				//if err = ParseFormData(c, fileUploadArgs.ChunkInfo); err != nil {
				klog.Warningf("err:%v", err)
				return nil, errors.New("missing parent dir query parameter in resumable info")
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

	if c.Query("file_name") != "" {
		fileUploadArgs.FileName = c.Query("file_name")
	}

	if c.Query("from") != "" {
		fileUploadArgs.From = c.Query("from")
	}

	return fileUploadArgs, nil
}

func fileUploadHandle(ctx context.Context, c *app.RequestContext, handleFunc fileUploadHandlerFunc) []byte {
	uploadArg, err := NewFileUploadArgs(c)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil
	}
	var handlerParam = &base.HandlerParam{
		Ctx:            ctx,
		Owner:          uploadArg.FileParam.Owner,
		ResponseWriter: nil,
		Request:        nil,
	}
	var fileHandler = drivers.Adaptor.NewFileHandler(uploadArg.FileParam.FileType, handlerParam)
	if fileHandler == nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("handler not found, type: %s", uploadArg.FileParam.FileType)})
		return nil
	}
	respBody, err := handleFunc(fileHandler, uploadArg)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return nil
	}
	return respBody
}
