//go:generate go-enum --sql --marshal --names --file $GOFILE
package http

import (
	"bytes"
	"encoding/json"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

// wrapperPreviewArgs
var wrapperPreviewArgs = func(fn previewHandlerFunc, prefix string) http.Handler {
	return previewHandle(fn, prefix)
}

type previewHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error)

func previewHandler(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
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

		klog.Infof("[Incoming] preview, user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, r.Method, utils.ToJson(contextArg))

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          contextArg.FileParam.Owner,
			ResponseWriter: w,
			Request:        r,
		}

		var fileType = contextArg.FileParam.FileType
		var handler = drivers.Adaptor.NewFileHandler(r.Context(), fileType, handlerParam)

		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", fileType), http.StatusBadRequest)
			return
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
		// w.Header().Set("Cache-Control", "private")
		http.ServeContent(w, r, fileData.FileName, fileData.FileModified, bytes.NewReader(fileData.Data))
		return
	})

	return handler
}
