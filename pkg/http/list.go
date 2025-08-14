package http

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

var wrapperFilesResourcesArgs = func(fn fileHandlerFunc, prefix string) http.Handler {
	return fileHandle(fn, prefix)
}

type fileHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error)

/**
 * list
 * create
 * rename
 */
func listHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.List(contextArgs)
}

func createHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.Create(contextArgs)
}

func renameHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.Rename(contextArgs)
}

func fileHandle(fn fileHandlerFunc, prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		contextArg, err := models.NewHttpContextArgs(r, prefix, false, false)
		if err != nil {
			klog.Errorf("context args error: %v, path: %s", err, r.URL.Path)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		klog.Infof("[Incoming-Resource] user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, r.Method, common.ToJson(contextArg))

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          contextArg.FileParam.Owner,
			ResponseWriter: w,
			Request:        r,
		}

		var handler = drivers.Adaptor.NewFileHandler(contextArg.FileParam.FileType, handlerParam)
		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", contextArg.FileParam.FileType), http.StatusBadRequest)
			return
		}

		res, err := fn(handler, contextArg)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
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
var wrapperFilesDeleteArgs = func(fn fileDeleteHandlerFunc, prefix string) http.Handler {
	return fileDeleteHandle(fn, prefix)
}

type fileDeleteHandlerFunc func(handler base.Execute, fileDeleteArgs *models.FileDeleteArgs) ([]byte, error)

func deleteHandler(handler base.Execute, fileDeleteArgs *models.FileDeleteArgs) ([]byte, error) {
	return handler.Delete(fileDeleteArgs)
}

func fileDeleteHandle(fn fileDeleteHandlerFunc, prefix string) http.Handler {
	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		deleteArg, err := models.NewFileDeleteArgs(r, prefix)
		if err != nil {
			klog.Errorf("delete args error: %v, path: %s", err, r.URL.Path)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		klog.Infof("[Incoming-Resource] user: %s, fsType: %s, method: %s, args: %s", deleteArg.FileParam.Owner, deleteArg.FileParam.FileType, r.Method, common.ToJson(deleteArg))

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          deleteArg.FileParam.Owner,
			ResponseWriter: w,
			Request:        r,
		}
		var handler = drivers.Adaptor.NewFileHandler(deleteArg.FileParam.FileType, handlerParam)
		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", deleteArg.FileParam.FileType), http.StatusBadRequest)
			return
		}

		res, err := fn(handler, deleteArg)
		if err != nil {
			var deleteFailedPaths []string
			if res != nil {
				json.Unmarshal(res, &deleteFailedPaths)
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"data":    deleteFailedPaths,
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
