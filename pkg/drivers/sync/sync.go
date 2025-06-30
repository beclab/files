package sync

import (
	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"io"
	"net/http"

	"k8s.io/klog/v2"
)

type SyncStorage struct {
	Handler *base.HandlerParam
	Service *Service
}

func (s *SyncStorage) List(fileParam *models.FileParam) ([]byte, error) {
	var owner = s.Handler.Owner

	klog.Infof("SYNC list, owner: %s, param: %s", owner, fileParam.Json())

	getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + fileParam.Extend + "/dir/?p=" + common.EscapeURLWithSpace(fileParam.Path) + "&with_thumbnail=true"

	klog.Infof("SYNC list, owner: %s, url: %s", s.Handler.Owner, getUrl)

	res, err := s.Service.Get(getUrl, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *SyncStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	klog.Infof("SYNC preview, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())
	var seahubUrl string
	var previewSize string

	var size = queryParam.Size
	if size != "big" {
		size = "thumb"
	}
	if size == "big" {
		previewSize = "/1080"
	} else {
		previewSize = "/128"
	}
	seahubUrl = "http://127.0.0.1:80/seahub/thumbnail/" + fileParam.Extend + previewSize + fileParam.Path

	klog.Infof("SYNC preview, owner: %s, url: %s", s.Handler.Owner, seahubUrl)

	res, err := s.Service.Get(seahubUrl, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *SyncStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	// var owner = s.Handler.Owner

	// klog.Infof("SYNC raw, owner: %s, param: %s", owner, fileParam.Json())

	// dlUrl := "http://127.0.0.1:80/seahub/lib/" + fileParam.Extend + "/file" + common.EscapeAndJoin(fileParam.Path, "/") + "/" + "?dl=1"
	// klog.Infof("redirect url: %s", dlUrl)

	// request, err := http.NewRequest("GET", dlUrl, nil)
	// if err != nil {
	// 	return nil, err
	// }

	// client := &http.Client{}
	// response, err := client.Do(request)
	// if err != nil {
	// 	return nil, err
	// }

	// return response.Body, nil
	return nil, nil
}
