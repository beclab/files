package sync

import (
	"files/pkg/common"
	"files/pkg/models"
	"net/http"
	"strings"

	"k8s.io/klog/v2"
)

func (s *SyncStorage) List(fileParam *models.FileParam) (int, error) {
	klog.Infof("POSIX SYNC list, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	var w = s.Handler.ResponseWriter
	var r = s.Handler.Request
	klog.Infof("Request headers: %+v", r.Header)

	var err error
	var urlPath = r.URL.Path

	src := strings.TrimPrefix(urlPath, "/sync")
	src, err = common.UnescapeURLIfEscaped(src)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if fileParam.Extend == "" {
		return s.repos()
	}

	if !strings.HasPrefix(r.URL.Path, "/sync") || strings.HasSuffix(src, "/") {
		src = strings.Trim(src, "/") + "/"

		getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + fileParam.Extend + "/dir/?p=" + common.EscapeURLWithSpace(fileParam.Path) + "&with_thumbnail=true"
		klog.Infoln(getUrl)

		res, err := s.Service.Get(getUrl, http.MethodGet, nil)
		if err != nil {
			return common.ErrToStatus(err), err
		}
		return common.RenderJSON(w, r, res)
	}

	repoID, prefix, filename := ParseSyncPath(src)
	getUrl := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file" + common.EscapeURLWithSpace(prefix) + common.EscapeURLWithSpace(filename) + "?dict=1"
	klog.Infoln(getUrl)

	res, err := s.Service.Get(getUrl, http.MethodGet, nil)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	return common.RenderJSON(w, r, res)
}

func (s *SyncStorage) repos() (int, error) {
	var w = s.Handler.ResponseWriter
	var r = s.Handler.Request

	getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/?type=mine"
	klog.Infoln(getUrl)

	res, err := s.Service.Get(getUrl, http.MethodGet, nil)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	return common.RenderJSON(w, r, res)
}
