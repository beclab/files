package sync

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
	"net/http"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

func (s *SyncStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	klog.Infof("SYNC preview, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	var w = s.Handler.ResponseWriter

	var seahubUrl string
	var previewSize string
	var isImage bool = true
	if strings.HasSuffix(fileParam.Path, ".txt") {
		isImage = false
	}

	if isImage {
		var size = s.Handler.Request.URL.Query().Get("size")
		if size != "big" {
			size = "thumb"
		}
		if size == "big" {
			previewSize = "/1080"
		} else {
			previewSize = "/128"
		}
		seahubUrl = "http://127.0.0.1:80/seahub/thumbnail/" + fileParam.Extend + previewSize + fileParam.Path
	} else {
		seahubUrl = "http://127.0.0.1:80/seahub/lib/" + fileParam.Extend + "/file" + fileParam.Path + "?dict=1"
	}

	klog.Infof("SYNC preview, owner: %s, url: %s", s.Handler.Owner, seahubUrl)

	res, err := s.Service.Get(seahubUrl, http.MethodGet, nil)
	if err != nil {
		return 400, err
	}

	if isImage {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(res)))
	}
	return w.Write(res)
}
