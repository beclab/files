package externalinternal

import (
	"files/pkg/common"
	"files/pkg/files"
	"net/http"

	"k8s.io/klog/v2"
)

func (s *InternalStorage) List() (int, error) {
	var r = s.Base.Base.Request
	var w = s.Base.Base.ResponseWriter

	klog.Infoln("X-Bfl-User: ", s.Base.Base.Owner)

	var file, err = files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       r.URL.Path,
		Modify:     true,
		Expand:     true,
		ReadHeader: s.Base.Base.Data.Server.TypeDetectionByHeader,
		Content:    true,
	})
	if err != nil {
		if common.ErrToStatus(err) == http.StatusNotFound {
			return common.RenderJSON(w, r, file)
		}
		return common.ErrToStatus(err), err
	}

	if file.IsDir {
		files.GetExternalExtraInfos(file, nil, 1)
		file.Listing.Sorting = files.DefaultSorting
		file.Listing.ApplySort()
	}
	return common.RenderJSON(w, r, file)
}
