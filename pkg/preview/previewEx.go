package preview

import (
	"bytes"
	"context"
	"files/pkg/diskcache"
	"files/pkg/img"
	"fmt"
	"os"
)

var (
	W1080 = 1080
	H1080 = 1080

	W256 = 256
	H256 = 256
)

func GeneratePreviewCacheKey(filePath string, fileModified string, previewSize string) string {
	return fmt.Sprintf("%s%s%s", filePath, fileModified, previewSize)
}

func GetCache(cacheKey string) ([]byte, bool, error) {
	var fileCache = diskcache.GetFileCache()
	resizedImage, ok, err := fileCache.Load(context.Background(), cacheKey)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return resizedImage, true, nil
}

func SetCache(cacheKey string, data []byte) error {
	var fileCache = diskcache.GetFileCache()
	return fileCache.Store(context.Background(), cacheKey, data)
}

func OpenFile(ctx context.Context, imagePath string, size string) ([]byte, error) {
	fd, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}

	defer fd.Close()

	var w, h int
	var options []img.Option
	if size == "thumb" {
		w = W256
		h = H256
		options = append(options, img.WithMode(img.ResizeModeFill), img.WithQuality(img.QualityLow), img.WithFormat(img.FormatJpeg))
	} else if size == "big" {
		w = W1080
		h = H1080
		options = append(options, img.WithMode(img.ResizeModeFit), img.WithQuality(img.QualityMedium))
	} else {
		return nil, img.ErrUnsupportedFormat
	}

	var imgSvc = img.GetImageService()
	buf := &bytes.Buffer{}
	if err := imgSvc.Resize(ctx, fd, w, h, buf, options...); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
