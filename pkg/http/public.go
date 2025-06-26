package http

import (
	"files/pkg/common"
	"files/pkg/constant"
	"files/pkg/global"
	"net/http"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}

func nodesGetHandler(w http.ResponseWriter, r *http.Request) {
	var nodes = global.GlobalNode.GetNodes()

	var data = make(map[string]interface{})
	data["nodes"] = nodes
	data["currentNode"] = constant.NodeName

	var result = make(map[string]interface{})
	result["code"] = http.StatusOK
	result["data"] = data
	common.RenderJSON(w, r, result)
}
