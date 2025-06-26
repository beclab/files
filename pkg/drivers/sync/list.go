package sync

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/models"
	"net/http"

	"k8s.io/klog/v2"
)

func (s *SyncStorage) List(fileParam *models.FileParam) (int, error) {
	var w = s.Handler.ResponseWriter
	var r = s.Handler.Request
	var owner = s.Handler.Owner

	klog.Infof("SYNC list, owner: %s, param: %s", owner, fileParam.Json())

	if fileParam.Extend == "" {
		return s.getRepos()
	}

	getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + fileParam.Extend + "/dir/?p=" + common.EscapeURLWithSpace(fileParam.Path) + "&with_thumbnail=true"

	klog.Infof("SYNC list, owner: %s, url: %s", s.Handler.Owner, getUrl)

	res, err := s.Service.Get(getUrl, http.MethodGet, nil)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	var data map[string]interface{}
	if err = json.Unmarshal(res, &data); err != nil {
		return 503, err
	}

	return common.RenderJSON(w, r, data)
}

func (s *SyncStorage) getRepos() (int, error) {
	var w = s.Handler.ResponseWriter
	var r = s.Handler.Request

	getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/?type=mine"
	klog.Infoln(getUrl)

	res, err := s.Service.Get(getUrl, http.MethodGet, nil)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	var data map[string]interface{}
	if err = json.Unmarshal(res, &data); err != nil {
		return 503, err
	}

	return common.RenderJSON(w, r, data)
}
