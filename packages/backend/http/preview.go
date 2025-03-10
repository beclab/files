//go:generate go-enum --sql --marshal --names --file $GOFILE
package http

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/img"
	"github.com/filebrowser/filebrowser/v2/my_redis"
	"github.com/gorilla/mux"
	"io"
	"net/http"
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

type FileCache interface {
	Store(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, key string) error
}

var (
	maxConcurrentRequests = 10 // 每次处理的最大请求数
	sem                   = make(chan struct{}, maxConcurrentRequests)
)

func previewHandler(imgSvc ImgService, fileCache FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Download {
			return http.StatusAccepted, nil
		}
		vars := mux.Vars(r)

		previewSize, err := ParsePreviewSize(vars["size"])
		if err != nil {
			return http.StatusBadRequest, err
		}
		path := "/" + vars["path"]

		srcType := r.URL.Query().Get("src")
		if srcType == "sync" {
			return http.StatusNotImplemented, nil
		} else if srcType == "google" {
			return previewGetGoogle(w, r, previewSize, path, imgSvc, fileCache, enableThumbnails, resizePreview)
		} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			return previewGetCloudDrive(w, r, previewSize, path, imgSvc, fileCache, enableThumbnails, resizePreview)
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         d.user.Fs,
			Path:       path,
			Modify:     d.user.Perm.Modify,
			Expand:     true,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err != nil {
			return errToStatus(err), err
		}

		setContentDisposition(w, r, file)

		switch file.Type {
		case "image":
			return handleImagePreview(w, r, imgSvc, fileCache, file, previewSize, enableThumbnails, resizePreview)
		default:
			return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", file.Type)
		}
	})
}

func handleImagePreview(
	w http.ResponseWriter,
	r *http.Request,
	imgSvc ImgService,
	fileCache FileCache,
	file *files.FileInfo,
	previewSize PreviewSize,
	enableThumbnails, resizePreview bool,
) (int, error) {
	if (previewSize == PreviewSizeBig && !resizePreview) ||
		(previewSize == PreviewSizeThumb && !enableThumbnails) {
		return rawFileHandler(w, r, file)
	}

	format, err := imgSvc.FormatFromExtension(file.Extension)
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return rawFileHandler(w, r, file)
	}
	if err != nil {
		return errToStatus(err), err
	}

	cacheKey := previewCacheKey(file, previewSize)
	fmt.Println("cacheKey:", cacheKey)
	fmt.Println("f.RealPath:", file.RealPath())
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return errToStatus(err), err
	}
	if !ok {
		resizedImage, err = createPreview(imgSvc, fileCache, file, previewSize, 1)
		if err != nil {
			fmt.Println("first method failed!")
			resizedImage, err = createPreview(imgSvc, fileCache, file, previewSize, 2)
			if err != nil {
				fmt.Println("second method failed!")
				return rawFileHandler(w, r, file)
			}
			//return errToStatus(err), err
		}
	}

	if diskcache.CacheDir != "" {
		my_redis.UpdateFileAccessTimeToRedis(my_redis.GetFileName(cacheKey))
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, bytes.NewReader(resizedImage))

	return 0, nil
}

func createPreview(imgSvc ImgService, fileCache FileCache,
	file *files.FileInfo, previewSize PreviewSize, method int) ([]byte, error) {
	fmt.Println("!!!!CreatePreview:", previewSize)
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
			fmt.Println(err)
			return nil, err
		}
	} else if method == 2 {
		if err := imgSvc.Resize2(context.Background(), fd, width, height, buf, options...); err != nil {
			fmt.Println(err)
			return nil, err
		}
	}

	go func() {
		cacheKey := previewCacheKey(file, previewSize)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			fmt.Printf("failed to cache resized image: %v", err)
		}
	}()

	return buf.Bytes(), nil
}

func previewCacheKey(f *files.FileInfo, previewSize PreviewSize) string {
	//return stringMD5(fmt.Sprintf("%s%d%s", f.RealPath(), f.ModTime.Unix(), previewSize))
	return fmt.Sprintf("%x%x%x", f.RealPath(), f.ModTime.Unix(), previewSize)
}
