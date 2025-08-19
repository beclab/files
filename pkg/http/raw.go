package http

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"fmt"
	"io"
	"mime"
	"net/http"

	"k8s.io/klog/v2"
)

var wrapperRawArgs = func(fn rawHandlerFunc, prefix string) http.Handler {
	return rawHandle(fn, prefix)
}

type rawHandlerFunc func(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error)

func rawHandler(handler base.Execute, contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {
	return handler.Raw(contextArgs)
}

func rawHandle(fn rawHandlerFunc, prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		contextArg, err := models.NewHttpContextArgs(r, prefix, false, false)
		if err != nil {
			klog.Errorf("context args error: %v, path: %s", err, r.URL.Path)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		klog.Infof("[Incoming] raw, user: %s, fsType: %s, method: %s, args: %s", contextArg.FileParam.Owner, contextArg.FileParam.FileType, r.Method, common.ToJson(contextArg))

		var handlerParam = &base.HandlerParam{
			Ctx:            r.Context(),
			Owner:          contextArg.FileParam.Owner,
			ResponseWriter: w,
			Request:        r,
		}

		var rawInline = contextArg.QueryParam.RawInline
		var rawMeta = contextArg.QueryParam.RawMeta
		var fileType = contextArg.FileParam.FileType

		var handler = drivers.Adaptor.NewFileHandler(fileType, handlerParam)
		if handler == nil {
			http.Error(w, fmt.Sprintf("handler not found, type: %s", fileType), http.StatusBadRequest)
			return
		}

		file, err := fn(handler, contextArg)
		if err != nil {
			klog.Errorf("raw error: %v, user: %s, url: %s", err, contextArg.FileParam.Owner, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			})
			return
		}

		if rawInline == "true" {
			if rawMeta == "true" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
			}
			w.Header().Set("Cache-Control", "private")
			w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{
				"filename": file.FileName,
			}))

		} else {
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
				"filename": file.FileName,
			}))
		}

		if !file.IsCloud {
			http.ServeContent(w, r, file.FileName, file.FileModified, file.Reader)
		} else {
			for k, vs := range file.RespHeader {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}

			if rawInline == "true" {
				w.Header().Set("Cache-Control", "private")
				w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{
					"filename": file.FileName,
				}))
				w.Header().Set("Content-Type", common.MimeTypeByExtension(file.FileName))
			}

			w.WriteHeader(file.StatusCode)
			io.Copy(w, file.ReadCloser)
		}
	})

	return handler
}
