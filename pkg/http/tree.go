package http

import (
	"encoding/json"
	"files/pkg/constant"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/klog/v2"
)

var wrapWithTreeParm = func(fn treeHandlerFunc, prefix string) http.Handler {
	return treeHandle(fn, prefix)
}

type treeHandlerFunc func(handler base.Execute, fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error

func treeHandler(handler base.Execute, fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	return handler.Tree(fileParam, stopChan, dataChan)
}

func treeHandle(fn treeHandlerFunc, prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var path = strings.TrimPrefix(r.URL.Path, prefix)
		if path == "" {
			http.Error(w, "path invalid", http.StatusBadRequest)
			return
		}
		var owner = r.Header.Get(constant.REQUEST_HEADER_OWNER)
		if owner == "" {
			http.Error(w, "user not found", http.StatusBadRequest)
			return
		}

		fileParam, err := models.CreateFileParam(owner, path)
		if err != nil {
			klog.Errorf("file param error: %v, owner: %s", err, owner)
			http.Error(w, fmt.Sprintf("file param error: %v", err), http.StatusBadRequest)
			return
		}

		klog.Infof("[Incoming] tree, user: %s, fsType: %s, method: %s, args: %s", owner, fileParam.FileType, r.Method, fileParam.Json())

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          owner,
			ResponseWriter: w,
			Request:        r,
		}

		stopChan := make(chan struct{})
		dataChan := make(chan string)

		var handler = drivers.Adaptor.NewFileHandler(fileParam.FileType, handlerParam)
		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", fileParam.FileType), http.StatusBadRequest)
			return
		}

		err = fn(handler, fileParam, stopChan, dataChan)
		if err != nil {
			klog.Errorf("tree error: %v, user: %s, url: %s", err, owner, r.URL.Path)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		for {
			select {
			case event, ok := <-dataChan:
				if !ok {
					return
				}
				_, err := w.Write([]byte(event))
				if err != nil {
					klog.Error(err)
					return
				}
				flusher.Flush()

			case <-r.Context().Done():
				close(stopChan)
				return
			}
		}
	})

	return handler
}
