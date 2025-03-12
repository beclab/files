package http

import (
	"context"
	"files/pkg/diskcache"
	"files/pkg/redisutils"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"

	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
)

func resourceGetHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	start := time.Now()
	klog.Infoln("Function resourceGetHandler starts at", start)
	defer func() {
		elapsed := time.Since(start)
		klog.Infof("Function resourceGetHandler execution time: %v\n", elapsed)
	}()

	streamStr := r.URL.Query().Get("stream")
	stream := 0
	var err error
	if streamStr != "" {
		stream, err = strconv.Atoi(streamStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	metaStr := r.URL.Query().Get("meta")
	meta := 0
	if metaStr != "" {
		meta, err = strconv.Atoi(metaStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	srcType := r.URL.Query().Get("src")

	handler, err := getResourceHandler(srcType)
	if err != nil {
		return http.StatusBadRequest, err
	}

	return handler.Handle(w, r, stream, meta, d)
}

func resourceDeleteHandler(fileCache FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.URL.Path == "/" {
			return http.StatusForbidden, nil
		}

		srcType := r.URL.Query().Get("src")
		if srcType == "google" {
			_, status, err := resourceDeleteGoogle(fileCache, "", w, r, false)
			return status, err
		} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			_, status, err := resourceDeleteCloudDrive(fileCache, "", w, r, false)
			return status, err
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
		})
		if err != nil {
			return errToStatus(err), err
		}

		// delete thumbnails
		err = delThumbs(r.Context(), fileCache, file)
		if err != nil {
			return errToStatus(err), err
		}

		err = files.DefaultFs.RemoveAll(r.URL.Path)

		if err != nil {
			return errToStatus(err), err
		}

		return http.StatusOK, nil
	}
}

func resourcePostHandler(fileCache FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		srcType := r.URL.Query().Get("src")
		if srcType == "google" {
			_, status, err := resourcePostGoogle("", w, r, false)
			return status, err
		} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			_, status, err := resourcePostCloudDrive("", w, r, false)
			return status, err
		}

		modeParam := r.URL.Query().Get("mode")

		mode, err := strconv.ParseUint(modeParam, 8, 32)
		if err != nil || modeParam == "" {
			mode = 0775
		}

		fileMode := os.FileMode(mode)

		// Directories creation on POST.
		if strings.HasSuffix(r.URL.Path, "/") {
			err := files.DefaultFs.MkdirAll(r.URL.Path, fileMode)
			return errToStatus(err), err
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
		})
		if err == nil {
			if r.URL.Query().Get("override") != "true" {
				return http.StatusConflict, nil
			}
			err = delThumbs(r.Context(), fileCache, file)
			if err != nil {
				return errToStatus(err), err
			}
		}

		info, err := writeFile(files.DefaultFs, r.URL.Path, r.Body)
		etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
		w.Header().Set("ETag", etag)

		if err != nil {
			_ = files.DefaultFs.RemoveAll(r.URL.Path)
		}

		return errToStatus(err), err
	}
}

func resourcePutHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	// Only allow PUT for files.
	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	info, err := writeFile(files.DefaultFs, r.URL.Path, r.Body)
	etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
	w.Header().Set("ETag", etag)

	return errToStatus(err), err
}

func resourcePatchHandler(fileCache FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		srcType := r.URL.Query().Get("src")
		if srcType == "google" {
			return resourcePatchGoogle(fileCache, w, r)
		} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			return resourcePatchCloudDrive(fileCache, w, r)
		}

		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		action := r.URL.Query().Get("action")
		dst, err := unescapeURLIfEscaped(dst)

		if err != nil {
			return errToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		err = checkParent(src, dst)
		if err != nil {
			return http.StatusBadRequest, err
		}

		override := r.URL.Query().Get("override") == "true"
		rename := r.URL.Query().Get("rename") == "true"
		if !override && !rename {
			if _, err = files.DefaultFs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		if rename {
			dst = addVersionSuffix(dst, files.DefaultFs, strings.HasSuffix(src, "/"))
		}

		// Permission for overwriting the file
		if override {
			return http.StatusForbidden, nil
		}

		klog.Infoln("Before patch action:", src, dst, action, override, rename)
		err = patchAction(r.Context(), action, src, dst, d, fileCache)

		return errToStatus(err), err
	}
}

func checkParent(src, dst string) error {
	rel, err := filepath.Rel(src, dst)
	if err != nil {
		return err
	}

	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") && rel != ".." && rel != "." {
		return errors.ErrSourceIsParent
	}

	return nil
}

func addVersionSuffix(source string, fs afero.Fs, isDir bool) string {
	counter := 1
	dir, name := path.Split(source)
	ext := ""
	base := name
	if !isDir {
		ext = filepath.Ext(name)
		base = strings.TrimSuffix(name, ext)
	}

	for {
		if _, err := fs.Stat(source); err != nil {
			break
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}

	return source
}

func writeFile(fs afero.Fs, dst string, in io.Reader) (os.FileInfo, error) {
	klog.Infoln("Before open ", dst)
	dir, _ := path.Split(dst)
	err := fs.MkdirAll(dir, 0775)
	if err != nil {
		return nil, err
	}

	klog.Infoln("Open ", dst)
	file, err := fs.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	klog.Infoln("Copy file!")
	_, err = io.Copy(file, in)
	if err != nil {
		return nil, err
	}

	klog.Infoln("Get stat")
	// Gets the info about the file.
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	klog.Infoln(info)
	return info, nil
}

func delThumbs(ctx context.Context, fileCache FileCache, file *files.FileInfo) error {
	for _, previewSizeName := range PreviewSizeNames() {
		size, _ := ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKey(file, size)
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

func patchAction(ctx context.Context, action, src, dst string, d *data, fileCache FileCache) error {
	switch action {
	case "copy":
		return fileutils.Copy(files.DefaultFs, src, dst)
	case "rename":
		src = path.Clean("/" + src)
		dst = path.Clean("/" + dst)

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       src,
			Modify:     true,
			Expand:     false,
			ReadHeader: false,
		})
		if err != nil {
			return err
		}

		// delete thumbnails
		err = delThumbs(ctx, fileCache, file)
		if err != nil {
			return err
		}

		return fileutils.MoveFile(files.DefaultFs, src, dst)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
}
