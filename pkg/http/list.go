package http

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"files/pkg/settings"
	"files/pkg/utils"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

type fileHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error)

func listHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.List(contextArgs)
}

func createHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.Create(contextArgs)
}

func fileHandle(fn fileHandlerFunc, prefix string, server *settings.Server) http.Handler {
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
			Data: &common.Data{
				Server: server,
			},
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
