//go:generate go-enum --sql --marshal --names --file $GOFILE
package http

import (
	"bytes"
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"fmt"
	"io"
	"net/http"

	"k8s.io/klog/v2"
)

// WrapperPreviewArgs
var WrapperPreviewArgs = func(fn previewHandlerFunc, prefix string) http.Handler {
	return previewHandle(fn, prefix)
}

type previewHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error)

func PreviewHandler(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	return handler.Preview(contextArgs)
}

func previewHandle(fn previewHandlerFunc, prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		contextArg, err := models.NewHttpContextArgs(r, prefix, true, true)
		if err != nil {
			klog.Errorf("context args error: %v, path: %s", err, r.URL.Path)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		klog.Infof("[Incoming] preview, user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, r.Method, common.ToJson(contextArg))

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          contextArg.FileParam.Owner,
			ResponseWriter: w,
			Request:        r,
		}

		var fileType = contextArg.FileParam.FileType
		var handler = drivers.Adaptor.NewFileHandler(fileType, handlerParam)

		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", fileType), http.StatusBadRequest)
			return
		}

		if contextArg.FileParam.FileType == common.AwsS3 ||
			contextArg.FileParam.FileType == common.DropBox ||
			contextArg.FileParam.FileType == common.GoogleDrive {
			if contextArg.QueryParam.PreviewSize == "thumb" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		fileData, err := fn(handler, contextArg)
		if err != nil {
			klog.Errorf("preview error: %v, user: %s, url: %s", err, contextArg.FileParam.Owner, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			})
			return
		}

		w.Header().Set("Content-Disposition", "inline")

		if !fileData.IsCloud {
			http.ServeContent(w, r, fileData.FileName, fileData.FileModified, bytes.NewReader(fileData.Data))
		} else {
			for k, vs := range fileData.RespHeader {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(fileData.StatusCode)
			io.Copy(w, fileData.ReadCloser)
		}

	})

	return handler
}
