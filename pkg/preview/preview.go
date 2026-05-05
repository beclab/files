package preview

import (
	"bytes"
	"context"
	"files/pkg/common"
	"files/pkg/diskcache"
	"files/pkg/files"
	"files/pkg/img"
	"files/pkg/models"
	"io"

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
}

var (
	W1000 = 1000
	H1000 = 1000

	W256 = 256
	H256 = 256
)

func GetPreviewCache(owner string, key string, tag string) ([]byte, bool, error) {
	var fileCache = diskcache.GetFileCache()

	return fileCache.Load(context.Background(), owner, key, tag)
}

func CreatePreview(owner string, key string,
	bufferFile *files.FileInfo,
	queryParam *models.QueryParam) ([]byte, error) {
	var fileCache = diskcache.GetFileCache()
	var imgSvc = img.GetImageService()
	var size = queryParam.PreviewSize

	klog.Infof("[preview] file: %s, key: %s, size: %s", bufferFile.Path, key, size)

	var previewSize, err = ParsePreviewSize(size)
	if err != nil {
		return nil, err
	}

	fileFormat, err := imgSvc.FormatFromExtension(bufferFile.Extension)
	klog.Infof("[preview] fileFormat: %s", fileFormat)
	if err == img.ErrUnsupportedFormat || fileFormat == img.FormatGif {
		fd, err := bufferFile.Fs.Open(bufferFile.Path)
		if err != nil {
			return nil, err
		}
		defer fd.Close()

		data, err := io.ReadAll(fd)
		if err != nil {
			return nil, err
		}

		return data, err
	}

	fd, err := bufferFile.Fs.Open(bufferFile.Path)
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
		width = W1000
		height = H1000
		options = append(options, img.WithMode(img.ResizeModeFit), img.WithQuality(img.QualityMedium))
	case previewSize == PreviewSizeThumb:
		width = W256
		height = H256
		options = append(options, img.WithMode(img.ResizeModeFill), img.WithQuality(img.QualityLow), img.WithFormat(img.FormatJpeg))
	default:
		return nil, img.ErrUnsupportedFormat
	}

	buf := &bytes.Buffer{}
	if err = imgSvc.Resize(context.TODO(), fd, width, height, buf, options...); err != nil {
		return nil, err
	}

	// Cache the rendered preview. Failures here are intentionally
	// non-fatal: we still have the bytes in `buf` and can serve them
	// to the caller, the next request will just have to render again.
	// Use a separate variable so a later call does not overwrite this
	// error reference (the previous code shared `err` across Store /
	// Chown / return, making it easy to lose information silently in
	// future edits).
	if cerr := fileCache.Store(context.TODO(), owner, key, common.CacheThumb, buf.Bytes()); cerr != nil {
		klog.Errorf("preview store failed, user: %s, key: %s, error: %v", owner, key, cerr)
	}

	if cerr := files.Chown(bufferFile.Fs, bufferFile.Path, 1000, 1000); cerr != nil {
		klog.Errorf("can't chown file %s to user %d: %s", bufferFile.Path, 1000, cerr)
	}

	return buf.Bytes(), nil

}
