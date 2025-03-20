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
		//start := time.Now()
		//klog.Infoln("Function previewHandler starts at", start)
		//defer func() {
		//	elapsed := time.Since(start)
		//	klog.Infof("Function previewHandler execution time: %v\n", elapsed)
		//}()

		srcType := r.URL.Query().Get("src")
		handler, err := drives.GetResourceService(srcType)
		if err != nil {
			return http.StatusBadRequest, err
		}

		return handler.PreviewHandler(imgSvc, fileCache, enableThumbnails, resizePreview)(w, r, d)
	}
}
