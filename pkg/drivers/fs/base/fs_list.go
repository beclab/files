package base

import (
	"files/pkg/common"
	"files/pkg/files"
	"net/http"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

var (
	MountedData   []files.DiskInfo = nil
	mu            sync.Mutex
	MountedTicker = time.NewTicker(5 * time.Minute)
)

func (s *FSStorage) List() (int, error) {
	var r = s.Base.Request
	var w = s.Base.ResponseWriter

	klog.Infoln("X-Bfl-User: ", s.Base.Owner)
	var file *files.FileInfo

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       r.URL.Path,
		Modify:     true,
		Expand:     true,
		ReadHeader: s.Base.Data.Server.TypeDetectionByHeader,
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
