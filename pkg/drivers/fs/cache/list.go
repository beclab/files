package cache

import (
	"errors"
	"files/pkg/appdata"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"net/http"
	"path/filepath"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

func (s *CacheStorage) List(fileParam *models.FileParam) (int, error) {

	var r = s.Base.Handler.Request
	var w = s.Base.Handler.ResponseWriter
	var fsType = fileParam.FileType

	if fileParam.Extend == "" {
		return s.nodes()
	}

	var cacheRootPrefix = s.Base.GetRoot(fsType)
	if cacheRootPrefix == "" {
		return 400, errors.New("type root path invalid")
	}

	var pvc, err = s.Base.GetPvc(fsType)
	if err != nil {
		return 400, errors.New("pvc not found")
	}

	var file *files.FileInfo
	var basePath = filepath.Join(cacheRootPrefix, pvc)
	klog.Infof("POSIX CACHE list, owner: %s basepath: %s, param: %s", s.Base.Handler.Owner, basePath, fileParam.Json())
	file, err = files.NewFileInfo(files.FileOptions{
		Fs:           afero.NewBasePathFs(afero.NewOsFs(), basePath),
		Path:         fileParam.Path,
		Modify:       true,
		Expand:       true,
		ReadHeader:   s.Base.Handler.Data.Server.TypeDetectionByHeader,
		Content:      true,
		AppendPrefix: filepath.Join("/Cache"),
	})
	// file.Path = filepath.Join("/AppData", pvc, fileParam.Path) // for test
	file.Path = filepath.Join("/Cache", fileParam.Path)
	if err != nil {
		if common.ErrToStatus(err) == http.StatusNotFound {
			return common.RenderJSON(w, r, file)
		}
		return common.ErrToStatus(err), err
	}

	if file.IsDir {
		file.Listing.Sorting = files.DefaultSorting
		file.Listing.ApplySort()
	}
	return common.RenderJSON(w, r, file)
}

func (s *CacheStorage) nodes() (int, error) {
	var r = s.Base.Handler.Request
	var w = s.Base.Handler.ResponseWriter
	nodes, err := appdata.AppData.GetNodes()
	if err != nil {
		return 400, errors.New("nodes not found")
	}
	var data = map[string]interface{}{
		"code": 200,
		"data": nodes.Items,
	}
	return common.RenderJSON(w, r, data)
}
