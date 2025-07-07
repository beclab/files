package http

import (
	"encoding/json"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

var wrapperFilesResourcesArgs = func(fn fileHandlerFunc, prefix string) http.Handler {
	return fileHandle(fn, prefix)
}

type fileHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error)

/**
 * list
 * create
 */
func listHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.List(contextArgs)
}

func createHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.Create(contextArgs)
}

func fileHandle(fn fileHandlerFunc, prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		contextArg, err := models.NewHttpContextArgs(r, prefix, false, false)
		if err != nil {
			klog.Errorf("context args error: %v, path: %s", err, r.URL.Path)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		klog.Infof("[Incoming-Resource] user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, r.Method, utils.ToJson(contextArg))

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          contextArg.FileParam.Owner,
			ResponseWriter: w,
			Request:        r,
			// Data: &common.Data{
			// 	Server: server,
			// },
		}

		var handler = drivers.Adaptor.NewFileHandler(r.Context(), contextArg.FileParam.FileType, handlerParam)
		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", contextArg.FileParam.FileType), http.StatusBadRequest)
			return
		}

		res, err := fn(handler, contextArg)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
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

/**
 * delete
 */
var wrapperFilesDeleteArgs = func(fn fileHandlerFunc, prefix string) http.Handler {
	return fileDeleteHandle(fn, prefix)
}

func deleteHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.Delete(contextArgs)
}

func fileDeleteHandle(fn fileHandlerFunc, prefix string) http.Handler {
	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody models.DeleteFileRequest
		var errFunc = func() error {
			if e := json.NewDecoder(r.Body).Decode(&reqBody); e != nil {
				return fmt.Errorf("failed to decode request body: %v", e)
			}
			return nil
		}
		defer r.Body.Close()

		if e := errFunc(); e != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": e.Error(),
			})
			return
		}

		contextArg, err := models.NewHttpContextArgs(r, prefix, false, false)
		if err != nil {
			klog.Errorf("context args error: %v, path: %s", err, r.URL.Path)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		contextArg.DeleteParam = &reqBody

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          contextArg.FileParam.Owner,
			ResponseWriter: w,
			Request:        r,
		}
		var handler = drivers.Adaptor.NewFileHandler(r.Context(), contextArg.FileParam.FileType, handlerParam)
		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", contextArg.FileParam.FileType), http.StatusBadRequest)
			return
		}

		res, err := fn(handler, contextArg)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
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
