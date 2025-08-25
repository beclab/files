package handle_func

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

/**
 * get nodes
 */
func NodesGetHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	var nodes = global.GlobalNode.GetNodes()

	var data = make(map[string]interface{})
	data["nodes"] = nodes
	data["currentNode"] = common.NodeName

	var result = make(map[string]interface{})
	result["code"] = consts.StatusOK
	result["data"] = data

	res, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return res, nil
}
