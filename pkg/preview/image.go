package preview

import (
	"bytes"
	"files/pkg/diskcache"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/img"
	"files/pkg/redisutils"
	"files/pkg/settings"
	"net/http"

	"k8s.io/klog/v2"
)

func HandleImagePreview(
	w http.ResponseWriter,
	r *http.Request,
	imgSvc ImgService,
	fileCache fileutils.FileCache,
	file *files.FileInfo,
	server *settings.Server,
) (int, error) {
	var size = r.URL.Query().Get("size")
	if size == "" {
		size = "thumb"
	}

	previewSize, err := ParsePreviewSize(size)
	if err != nil {
		return http.StatusBadRequest, err
	}

	var enableThumbnails = server.EnableThumbnails
	var resizePreview = server.ResizePreview
	if (previewSize == PreviewSizeBig && !resizePreview) ||
		(previewSize == PreviewSizeThumb && !enableThumbnails) {
		return RawFileHandler(w, r, file)
	}

	format, err := imgSvc.FormatFromExtension(file.Extension)
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return RawFileHandler(w, r, file)
	}
	if err != nil {
		return 400, err
	}

	cacheKey := PreviewCacheKey(file, previewSize)
	klog.Infoln("cacheKey:", cacheKey)
	klog.Infoln("f.RealPath:", file.RealPath())
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return 400, err
	}
	if !ok {
		resizedImage, err = CreatePreview(imgSvc, fileCache, file, previewSize, 1)
		if err != nil {
			klog.Errorf("create preview error: %v", err)
			// klog.Infoln("first method failed!")
			// resizedImage, err = preview.CreatePreview(imgSvc, fileCache, file, previewSize, 2)
			// if err != nil {
			// 	klog.Infoln("second method failed!")
			// 	return RawFileHandler(w, r, file)
			// }
			return 400, err
		}
	}

	if diskcache.CacheDir != "" {
		redisutils.UpdateFileAccessTimeToRedis(redisutils.GetFileName(cacheKey))
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, bytes.NewReader(resizedImage))

	return 0, nil
}

func RawFileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}
