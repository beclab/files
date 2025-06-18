package sync

import (
	"files/pkg/common"
	"files/pkg/models"
	"net/http"
	"strings"

	"k8s.io/klog/v2"
)

func (s *SyncStorage) List(fileParam *models.FileParam) (int, error) {
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
	klog.Infof("r.URL.Path: %s, src Path: %s", urlPath, src)

	if src == "/" {
		// this is for "/sync/" which is listing all repos of mine
		getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/?type=mine"
		klog.Infoln(getUrl)

		res, err := s.Service.Get(getUrl, http.MethodGet, nil)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		return common.RenderJSON(w, r, res)
	}

	if !strings.HasPrefix(r.URL.Path, "/sync") || strings.HasSuffix(src, "/") {
		src = strings.Trim(src, "/") + "/"
		repoID, prefix, _ := ParseSyncPath(src)

		getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace(prefix) + "&with_thumbnail=true"
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
