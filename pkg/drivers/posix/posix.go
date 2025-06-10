package posix

import (
	"files/pkg/common"
	"files/pkg/constant"
	"files/pkg/drivers/base"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/preview"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

type PosixStorage struct {
	Handler *base.HandlerParam
}

func (s *PosixStorage) List(fileParam *models.FileParam) (int, error) {
	var r = s.Handler.Request
	var w = s.Handler.ResponseWriter
	var owner = s.Handler.Owner

	klog.Infof("POSIX list, owner: %s, param: %s", owner, fileParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return 400, err
	}

	klog.Infof("POSIX list, owner: %s, resourceuri: %s", s.Handler.Owner, resourceUri)
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         afero.NewBasePathFs(afero.NewOsFs(), resourceUri),
		FsType:     fileParam.FileType,
		FsExtend:   fileParam.Extend,
		Path:       fileParam.Path,
		Modify:     true,
		Expand:     true,
		ReadHeader: s.Handler.Data.Server.TypeDetectionByHeader,
		Content:    false,
	})

	if err != nil {
		if common.ErrToStatus(err) == http.StatusNotFound {
			return common.RenderJSON(w, r, file)
		}
		return common.ErrToStatus(err), err
	}

	if fileParam.FileType == constant.Cache { // cache
		file.Path = filepath.Join("/Cache", fileParam.Extend)
	}

	if (fileParam.FileType == constant.External || fileParam.FileType == constant.Usb || fileParam.FileType == constant.Hdd || fileParam.FileType == constant.Internal || fileParam.FileType == constant.Smb) && fileParam.Extend != "" { // external
		// s.combineMounted(file)
		for _, f := range file.Items {
			f.ExternalType = global.GlobalMounted.CheckExternalType(f.Name)
		}
		file.Path = filepath.Join("/External", fileParam.Extend)
	}

	if file.IsDir {
		file.Listing.Sorting = files.DefaultSorting
		file.Listing.ApplySort()
	}

	return common.RenderJSON(w, r, file)
}

func (s *PosixStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	var owner = s.Handler.Owner
	var mode = 0775

	klog.Infof("POSIX create, owner: %s, param: %s", owner, fileParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return 400, err
	}

	fileMode := os.FileMode(mode)

	klog.Infof("POSIX create, owner: %s, basepath: %s", owner, resourceUri)
	if strings.HasSuffix(fileParam.Path, "/") {
		// files.DefaultFs
		if err := fileutils.MkdirAllWithChown(afero.NewBasePathFs(afero.NewOsFs(), resourceUri), fileParam.Path, fileMode); err != nil {
			klog.Errorln(err)
			return common.ErrToStatus(err), err
		}
		return http.StatusOK, nil
	}
	return http.StatusBadRequest, fmt.Errorf("%s is not a valid directory path", fileParam.Path)
}

func (s *PosixStorage) Rename(fileParam *models.FileParam) (int, error) {
	klog.Infof("POSIX rename, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	var r = s.Handler.Request

	var action = r.URL.Query().Get("action")
	var dst = r.URL.Query().Get("destination")

	if action != "rename" {
		return 400, fmt.Errorf("invalid action: %s", action)
	}

	if dst == "" {
		return 400, fmt.Errorf("invalid destination: %s", dst)
	}

	return 0, nil
}

func (s *PosixStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	var w = s.Handler.ResponseWriter
	var r = s.Handler.Request
	var server = s.Handler.Data.Server
	var owner = s.Handler.Owner

	klog.Infof("POSIX preview, owner: %s, param: %s", owner, fileParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return 400, err
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         afero.NewBasePathFs(afero.NewOsFs(), resourceUri),
		Path:       fileParam.Path,
		Modify:     true,
		Expand:     true,
		ReadHeader: s.Handler.Data.Server.TypeDetectionByHeader,
	})
	if err != nil {
		return common.ErrToStatus(err), err
	}

	preview.SetContentDisposition(w, r, file)

	switch file.Type {
	case "image":
		return preview.HandleImagePreview(w, r, imgSvc, fileCache, file, server)
	case "text":
		return preview.HandlerTextPreview(w, r, fileCache, file, server)
	default:
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", file.Type)
	}
}
