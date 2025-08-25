package handler

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

/**
 * upload
 */
var WrapperFilesUploadArgs = func(fn fileUploadHandlerFunc) http.Handler {
	return fileUploadHandle(fn)
}

type fileUploadHandlerFunc func(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error)

func FileUploadLinkHandler(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error) {
	return handler.UploadLink(fileUploadArgs)
}

func FileUploadedBytesHandler(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error) {
	return handler.UploadedBytes(fileUploadArgs)
}

func FileUploadChunksHandler(handler base.Execute, fileUploadArgs *models.FileUploadArgs) ([]byte, error) {
	return handler.UploadChunks(fileUploadArgs)
}

func fileUploadHandle(fn fileUploadHandlerFunc) http.Handler {
	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		uploadArg, err := models.NewFileUploadArgs(r)
		if err != nil {
			klog.Errorf("upload args error: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		klog.Infof("[Incoming-Resource] user: %s, fsType: %s, method: %s, args: %s", uploadArg.FileParam.Owner, uploadArg.FileParam.FileType, r.Method, common.ToJson(uploadArg))

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          uploadArg.FileParam.Owner,
			ResponseWriter: w,
			Request:        r,
		}

		var handler = drivers.Adaptor.NewFileHandler(uploadArg.FileParam.FileType, handlerParam)
		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", uploadArg.FileParam.FileType), http.StatusBadRequest)
			return
		}

		res, err := fn(handler, uploadArg)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"data":    res,
				"message": err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(res)
		return

	})

	return handler
}
