package base

import (
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

var (
	MountedData   []files.DiskInfo = nil
	mu            sync.Mutex
	MountedTicker = time.NewTicker(5 * time.Minute)
)

func (s *FSStorage) List(fileParam *models.FileParam) (int, error) {
	var r = s.Handler.Request
	var w = s.Handler.ResponseWriter
	var fsType = fileParam.FileType

	var rootPrefix = s.GetRoot(fsType)
	if rootPrefix == "" {
		return 400, errors.New("type root path invalid")
	}

	var pvc, err = s.GetPvc(fsType)
	if err != nil {
		return 400, errors.New("pvc not found")
	}

	var file *files.FileInfo
	var basePath = filepath.Join(rootPrefix, pvc, fileParam.Extend)
	klog.Infof("POSIX BASE list, owner: %s, basepath: %s, param: %s", s.Handler.Owner, basePath, fileParam.Json())
	file, err = files.NewFileInfo(files.FileOptions{
		Fs:         afero.NewBasePathFs(afero.NewOsFs(), basePath),
		Path:       fileParam.Path,
		Modify:     true,
		Expand:     true,
		ReadHeader: s.Handler.Data.Server.TypeDetectionByHeader,
		Content:    true,
	})

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
