package posix

import (
	"files/pkg/common"
	"files/pkg/fileutils"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/preview"
)

type CacheStorage struct {
	Posix *PosixStorage
}

func (s *CacheStorage) List(fileParam *models.FileParam) (int, error) {
	var w = s.Posix.Handler.ResponseWriter
	var r = s.Posix.Handler.Request
	if fileParam.Extend == "" {
		var nodes = global.GlobalNode.GetNodes()
		var data = map[string]interface{}{
			"code": 200,
			"data": nodes,
		}
		return common.RenderJSON(w, r, data)
	}

	return s.Posix.List(fileParam)
}

func (s *CacheStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	return s.Posix.CreateFolder(fileParam)
}

func (s *CacheStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.Posix.Rename(fileParam)
}

func (s *CacheStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	return s.Posix.Preview(fileParam, imgSvc, fileCache)
}
