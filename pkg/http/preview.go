//go:generate go-enum --sql --marshal --names --file $GOFILE
package http

import (
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/fileutils"
	"files/pkg/preview"
	"net/http"
)

var (
	maxConcurrentRequests = 10
	sem                   = make(chan struct{}, maxConcurrentRequests)
)

func previewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		srcType := r.URL.Query().Get("src")
		handler, err := drives.GetResourceService(srcType)
		if err != nil {
			return http.StatusBadRequest, err
		}

		return handler.PreviewHandler(imgSvc, fileCache, enableThumbnails, resizePreview)(w, r, d)
	}
}
