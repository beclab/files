//go:generate go-enum --sql --marshal --names --file $GOFILE
package http

import (
	"bytes"
	"encoding/json"
	"files/pkg/constant"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type previewHandlerFunc func(handler base.Execute, fileParam *models.FileParam, queryParam *models.QueryParam) (*models.PreviewHandlerResponse, error)

func previewHandler(handler base.Execute, fileParam *models.FileParam, queryParam *models.QueryParam) (*models.PreviewHandlerResponse, error) {
	return handler.Preview(fileParam, queryParam)
}

func previewHandle(fn previewHandlerFunc, prefix string, driverHandler *drivers.DriverHandler) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var enableThumbnails = true
		var resizePreview = true

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

		klog.Infof("Incoming Path: %s, user: %s, method: %s", path, owner, r.Method)

		queryParam := models.CreateQueryParam(owner, r, enableThumbnails, resizePreview)

		fileParam, err := models.CreateFileParam(owner, path)
		if err != nil {
			klog.Errorf("file param error: %v, owner: %s", err, owner)
			http.Error(w, fmt.Sprintf("file param error: %v"), http.StatusBadRequest)
			return
		}

		klog.Infof("srcType: %s, url: %s, param: %s, query: %s", fileParam.FileType, r.URL.Path, fileParam.Json(), queryParam.Json())
		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          owner,
			ResponseWriter: w,
			Request:        r,
		}

		fileData, err := fn(driverHandler.NewFileHandler(fileParam.FileType, handlerParam), fileParam, queryParam)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			})
			return
		}

		if queryParam.RawInline == "true" {
			w.Header().Set("Content-Disposition", "inline")
		}
		w.Header().Set("Cache-Control", "private")
		http.ServeContent(w, r, "", time.Now(), bytes.NewReader(fileData.Data))
		return
	})

	return handler
}
