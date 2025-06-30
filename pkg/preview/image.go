package preview

import (
	"files/pkg/diskcache"
	"files/pkg/files"
	"files/pkg/img"
	"files/pkg/models"
	"files/pkg/redisutils"
	"io"

	"k8s.io/klog/v2"
)

func HandleImagePreview(
	file *files.FileInfo,
	queryParam *models.QueryParam,
) ([]byte, error) {
	var fileCache = diskcache.GetFileCache()
	var imgSvc = img.GetImageService()
	var size = queryParam.Size

	previewSize, err := ParsePreviewSize(size)
	if err != nil {
		return nil, err
	}

	var enableThumbnails = queryParam.EnableThumbnails
	var resizePreview = queryParam.ResizePreview

	if (previewSize == PreviewSizeBig && !resizePreview) ||
		(previewSize == PreviewSizeThumb && !enableThumbnails) {
		return RawFileHandler(file)
	}

	format, err := imgSvc.FormatFromExtension(file.Extension)
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return RawFileHandler(file)
	}
	if err != nil {
		return nil, err
	}

	cacheKey := PreviewCacheKey(file, previewSize)
	klog.Infoln("cacheKey:", cacheKey)
	klog.Infoln("f.RealPath:", file.RealPath())
	resizedImage, ok, err := fileCache.Load(queryParam.Ctx, cacheKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		resizedImage, err = CreatePreview(imgSvc, fileCache, file, previewSize, 1)
		if err != nil {
			klog.Errorf("create preview error: %v", err)
			return nil, err
		}
	}

	if diskcache.CacheDir != "" {
		redisutils.UpdateFileAccessTimeToRedis(redisutils.GetFileName(cacheKey))
	}

	return resizedImage, nil
}

func RawFileHandler(file *files.FileInfo) ([]byte, error) {
	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	data, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	return data, nil
}
