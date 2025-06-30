package http

import (
	"encoding/json"
	"files/pkg/constant"
	"files/pkg/global"
	"files/pkg/utils"
	"net/http"

	"k8s.io/klog/v2"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}

func reposGetHandler(owner string) ([]byte, error) {
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

func nodesGetHandler(owner string) ([]byte, error) {
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
