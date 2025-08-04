package operations

import (
	"context"
	"encoding/json"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/utils"
	commonutils "files/pkg/utils"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

type Interface interface {
	List(fs string) (*OperationsList, error)
}

type operations struct {
	config config.Interface
}

func NewOperations() *operations {
	return &operations{}
}

func (o *operations) List(fs string) (*OperationsList, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, ListPath)
	var param = OperationsListReq{
		Fs:     fs,
		Remote: "",
		Opt: struct {
			Metadata bool "json:\"metadata\""
		}{
			Metadata: true,
		},
	}

	resp, err := utils.Request(context.Background(), url, http.MethodPost, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations list error: %v, fs: %s", err, fs)
		return nil, err
	}

	var data *OperationsList
	if err := json.Unmarshal(resp, &data); err != nil {
		klog.Errorf("[rclone] operations list unmarshal error: %v, fs: %s", err, fs)
		return nil, err
	}

	return data, nil
}
