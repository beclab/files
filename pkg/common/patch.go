package common

import (
	"context"
	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/preview"
	"fmt"
	"github.com/spf13/afero"
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

func PatchAction(ctx context.Context, action, src, dst string, fileCache fileutils.FileCache) error {
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
		err = preview.DelThumbs(ctx, fileCache, file)
		if err != nil {
			return err
		}

		return fileutils.MoveFile(files.DefaultFs, src, dst)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
}
