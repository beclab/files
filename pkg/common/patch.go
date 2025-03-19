package common

import (
	"context"
	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/preview"
	"fmt"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func CheckParent(src, dst string) error {
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

func AddVersionSuffix(source string, fs afero.Fs, isDir bool) string {
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

func MoveDir(ctx context.Context, fs afero.Fs, source, dest string, fileCache fileutils.FileCache) error {
	// Get properties of source.
	srcinfo, err := fs.Stat(source)
	if err != nil {
		return err
	}

	// Create the destination directory.
	if err = fileutils.MkdirAllWithChown(fs, dest, srcinfo.Mode()); err != nil {
		klog.Errorln(err)
		return err
	}

	dir, _ := fs.Open(source)
	obs, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var errs []error

	for _, obj := range obs {
		fsource := filepath.Join(source, obj.Name())
		fdest := filepath.Join(dest, obj.Name())

		if obj.IsDir() {
			// Create sub-directories, recursively.
			err = MoveDir(ctx, fs, fsource, fdest, fileCache)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = MoveFile(ctx, fs, fsource, fdest, fileCache)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	var errString string
	for _, err := range errs {
		errString += err.Error() + "\n"
	}

	if errString != "" {
		return fmt.Errorf(errString)
	}

	return nil
}

func MoveFile(ctx context.Context, fs afero.Fs, src, dst string, fileCache fileutils.FileCache) error {
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
	err = preview.DelThumbs(ctx, fileCache, file)
	if err != nil {
		return err
	}

	return fileutils.MoveFile(fs, src, dst)
}

func Move(ctx context.Context, fs afero.Fs, src, dst string, fileCache fileutils.FileCache) error {
	if src = path.Clean("/" + src); src == "" {
		return os.ErrNotExist
	}

	if dst = path.Clean("/" + dst); dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the virtual root directory.
		return os.ErrInvalid
	}

	if dst == src {
		return os.ErrInvalid
	}

	info, err := fs.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return MoveDir(ctx, fs, src, dst, fileCache)
	}

	return MoveFile(ctx, fs, src, dst, fileCache)
}

func PatchAction(ctx context.Context, action, src, dst string, fileCache fileutils.FileCache) error {
	switch action {
	case "copy":
		return fileutils.Copy(files.DefaultFs, src, dst)
	case "rename":
		return Move(ctx, files.DefaultFs, src, dst, fileCache)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
}
