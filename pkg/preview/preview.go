package preview

import (
	"bytes"
	"context"
	"files/pkg/diskcache"
	"files/pkg/files"
	"files/pkg/img"
	"files/pkg/redisutils"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"k8s.io/klog/v2"
)

/*
ENUM(
thumb
big
)
*/
type PreviewSize int

type ImgService interface {
	FormatFromExtension(ext string) (img.Format, error)
	Resize(ctx context.Context, in io.Reader, width, height int, out io.Writer, options ...img.Option) error
	Resize2(ctx context.Context, in io.Reader, width, height int, out io.Writer, options ...img.Option) error
}

func SetContentDisposition(w http.ResponseWriter, r *http.Request, file *files.FileInfo) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// As per RFC6266 section 4.3
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(file.Name))
	}
}

func CreatePreview(imgSvc ImgService, fileCache files.FileCache,
	file *files.FileInfo, previewSize PreviewSize, method int) ([]byte, error) {
	klog.Infoln("!!!!CreatePreview:", previewSize)
	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return nil, err
	}
	fd.Seek(0, 0)
	defer fd.Close()

	var (
		width   int
		height  int
		options []img.Option
	)

	switch {
	case previewSize == PreviewSizeBig:
		width = 1080
		height = 1080
		options = append(options, img.WithMode(img.ResizeModeFit), img.WithQuality(img.QualityMedium))
	case previewSize == PreviewSizeThumb:
		width = 256
		height = 256
		options = append(options, img.WithMode(img.ResizeModeFill), img.WithQuality(img.QualityLow), img.WithFormat(img.FormatJpeg))
	default:
		return nil, img.ErrUnsupportedFormat
	}

	buf := &bytes.Buffer{}
	if method == 1 {
		if err := imgSvc.Resize(context.Background(), fd, width, height, buf, options...); err != nil {
			klog.Error(err)
			return nil, err
		}
	} else if method == 2 {
		if err := imgSvc.Resize2(context.Background(), fd, width, height, buf, options...); err != nil {
			klog.Error(err)
			return nil, err
		}
	}

	go func() {
		cacheKey := PreviewCacheKey(file, previewSize)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			klog.Errorf("failed to cache resized image: %v", err)
		}
	}()

	return buf.Bytes(), nil
}

func PreviewCacheKey(f *files.FileInfo, previewSize PreviewSize) string {
	return fmt.Sprintf("%x%x%x", f.RealPath(), f.ModTime.Unix(), previewSize)
}

func DelThumbs(ctx context.Context, fileCache files.FileCache, file *files.FileInfo) error {
	for _, previewSizeName := range PreviewSizeNames() {
		size, _ := ParsePreviewSize(previewSizeName)
		cacheKey := PreviewCacheKey(file, size)
		if err := fileCache.Delete(ctx, cacheKey); err != nil {
			return err
		}
		if diskcache.CacheDir != "" {
			err := redisutils.DelThumbRedisKey(redisutils.GetFileName(cacheKey))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
