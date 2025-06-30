package posix

import (
	"encoding/json"
	"files/pkg/drivers/base"
	"files/pkg/files"
	"files/pkg/models"
	"files/pkg/preview"
	"fmt"
	"io"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

type PosixStorage struct {
	Handler *base.HandlerParam
}

func (s *PosixStorage) List(fileParam *models.FileParam) ([]byte, error) {
	var owner = s.Handler.Owner

	klog.Infof("POSIX list, owner: %s, param: %s", owner, fileParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	klog.Infof("POSIX list, owner: %s, resourceuri: %s", s.Handler.Owner, resourceUri)
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         afero.NewBasePathFs(afero.NewOsFs(), resourceUri),
		Path:       fileParam.Path,
		Modify:     true,
		Expand:     true,
		ReadHeader: s.Handler.Data.Server.TypeDetectionByHeader,
		Content:    false,
	})

	if err != nil {
		return nil, err
	}

	if file == nil {
		return nil, fmt.Errorf("file %s not exists", fileParam.Path)
	}

	if file.IsDir {
		file.Listing.Sorting = files.DefaultSorting
		file.Listing.ApplySort()
	}

	res, err := json.Marshal(file)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *PosixStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	var owner = s.Handler.Owner

	klog.Infof("POSIX preview, owner: %s, param: %s, query: %s", owner, fileParam.Json(), queryParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         afero.NewBasePathFs(afero.NewOsFs(), resourceUri),
		Path:       fileParam.Path,
		Modify:     true,
		Expand:     true,
		ReadHeader: false,
	})
	if err != nil {
		return nil, err
	}

	switch file.Type {
	case "image":
		return preview.HandleImagePreview(file, queryParam)
	default:
		return nil, fmt.Errorf("can't create preview for %s type", file.Type)
	}
}

func (s *PosixStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	var owner = s.Handler.Owner

	klog.Infof("POSIX raw, owner: %s, param: %s", owner, fileParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         afero.NewBasePathFs(afero.NewOsFs(), resourceUri),
		Path:       fileParam.Path,
		Modify:     true,
		Expand:     false,
		ReadHeader: false,
	})
	if err != nil {
		return nil, err
	}

	if file.IsDir {
		return nil, fmt.Errorf("not supported currently")
	}

	return getRawFile(file)
}
