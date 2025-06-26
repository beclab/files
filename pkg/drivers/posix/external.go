package posix

import (
	"files/pkg/common"
	"files/pkg/fileutils"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/preview"
	"net/http"
)

type ExternalStorage struct {
	Posix *PosixStorage
}

// CreateFolder implements base.Execute.
func (s *ExternalStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	panic("unimplemented")
}

// Preview implements base.Execute.
func (s *ExternalStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	panic("unimplemented")
}

// Rename implements base.Execute.
func (s *ExternalStorage) Rename(fileParam *models.FileParam) (int, error) {
	panic("unimplemented")
}

func (s *ExternalStorage) List(fileParam *models.FileParam) (int, error) {
	var w = s.Posix.Handler.ResponseWriter
	var r = s.Posix.Handler.Request

	if fileParam.Extend == "" {
		var nodes = global.GlobalNode.GetNodes()
		var data = map[string]interface{}{
			"code": http.StatusOK,
			"data": nodes,
		}

		return common.RenderJSON(w, r, data)
	}

	return s.Posix.List(fileParam)
}
