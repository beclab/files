package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"files/pkg/constant"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/utils"
	"mime/multipart"
	"net/http"

	"k8s.io/klog/v2"
)

type commonFunc func(contextQueryArgs *models.QueryParam) ([]byte, error)

func commonHandle(fn commonFunc) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var path = r.URL.Path
		var owner = r.Header.Get(constant.REQUEST_HEADER_OWNER)
		if owner == "" {
			http.Error(w, "user not found", http.StatusBadRequest)
			return
		}

		var contextQueryArgs = models.CreateQueryParam(owner, r, false, false)

		klog.Infof("Incoming Path: %s, user: %s, method: %s", path, owner, r.Method)

		res, err := fn(contextQueryArgs)
		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			})
			return
		}

		w.Write(res)
		return
	})

	return handler
}

// health
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}

/**
 * get repos
 */
func reposGetHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	var owner = contextQueryArgs.Owner

	var header = &http.Header{
		constant.REQUEST_HEADER_OWNER: []string{owner},
	}
	var url = "http://127.0.0.1:80/seahub/api/v2.1/repos/?type=mine"
	var repos, err = utils.RequestWithContext(url, http.MethodGet, header, nil)
	if err != nil {
		klog.Errorf("get repos error: %v", err)
		return nil, err
	}
	klog.Infof("get repos: %s", string(repos))
	return repos, nil
}

/**
 * create new repo
 */

func createRepoHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	var owner = contextQueryArgs.Owner
	var repoName = contextQueryArgs.RepoName
	var url = "http://127.0.0.1:80/seahub/api2/repos/?from=web"

	if repoName == "" {
		return nil, errors.New("repo name is empty")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", repoName)
	_ = writer.WriteField("passwd", "")

	var header = &http.Header{
		constant.REQUEST_HEADER_OWNER: []string{owner},
		"Content-Type":                []string{writer.FormDataContentType()},
	}

	var res, err = utils.RequestWithContext(url, http.MethodPost, header, body.Bytes())
	if err != nil {
		klog.Errorf("create repo error: %v, name: %s", err, repoName)
		return nil, err
	}

	klog.Infof("repo create new one success, name: %s, result: %s", repoName, string(res))

	return nil, nil
}

/**
 * get nodes
 */
func nodesGetHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	var nodes = global.GlobalNode.GetNodes()

	var data = make(map[string]interface{})
	data["nodes"] = nodes
	data["currentNode"] = constant.NodeName

	var result = make(map[string]interface{})
	result["code"] = http.StatusOK
	result["data"] = data

	res, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return res, nil
}
