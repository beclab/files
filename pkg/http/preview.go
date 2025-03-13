//go:generate go-enum --sql --marshal --names --file $GOFILE
package http

import (
	"bytes"
	"files/pkg/common"
	"files/pkg/diskcache"
	"files/pkg/drives"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/img"
	"files/pkg/preview"
	"files/pkg/redisutils"
	"fmt"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
	"net/http"
)

var (
	maxConcurrentRequests = 10
	sem                   = make(chan struct{}, maxConcurrentRequests)
)

func previewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		vars := mux.Vars(r)

		previewSize, err := preview.ParsePreviewSize(vars["size"])
		if err != nil {
			return http.StatusBadRequest, err
		}
		path := "/" + vars["path"]

		srcType := r.URL.Query().Get("src")
		if srcType == "sync" {
			return http.StatusNotImplemented, nil
		} else if srcType == "google" {
			return drives.PreviewGetGoogle(w, r, previewSize, path, imgSvc, fileCache, enableThumbnails, resizePreview)
		} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			return drives.PreviewGetCloudDrive(w, r, previewSize, path, imgSvc, fileCache, enableThumbnails, resizePreview)
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.Server.TypeDetectionByHeader,
		})
		if err != nil {
			return common.ErrToStatus(err), err
		}

		setContentDisposition(w, r, file)

		switch file.Type {
		case "image":
			return HandleImagePreview(w, r, imgSvc, fileCache, file, previewSize, enableThumbnails, resizePreview)
		default:
			return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", file.Type)
		}
	}
}

func HandleImagePreview(
	w http.ResponseWriter,
	r *http.Request,
	imgSvc preview.ImgService,
	fileCache fileutils.FileCache,
	file *files.FileInfo,
	previewSize preview.PreviewSize,
	enableThumbnails, resizePreview bool,
) (int, error) {
	if (previewSize == preview.PreviewSizeBig && !resizePreview) ||
		(previewSize == preview.PreviewSizeThumb && !enableThumbnails) {
		return rawFileHandler(w, r, file)
	}

	format, err := imgSvc.FormatFromExtension(file.Extension)
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return rawFileHandler(w, r, file)
	}
	if err != nil {
		return common.ErrToStatus(err), err
	}

	cacheKey := preview.PreviewCacheKey(file, previewSize)
	klog.Infoln("cacheKey:", cacheKey)
	klog.Infoln("f.RealPath:", file.RealPath())
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	if !ok {
		resizedImage, err = preview.CreatePreview(imgSvc, fileCache, file, previewSize, 1)
		if err != nil {
			klog.Infoln("first method failed!")
			resizedImage, err = preview.CreatePreview(imgSvc, fileCache, file, previewSize, 2)
			if err != nil {
				klog.Infoln("second method failed!")
				return rawFileHandler(w, r, file)
			}
		}
	}

	if diskcache.CacheDir != "" {
		redisutils.UpdateFileAccessTimeToRedis(redisutils.GetFileName(cacheKey))
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, bytes.NewReader(resizedImage))

	return 0, nil
}
