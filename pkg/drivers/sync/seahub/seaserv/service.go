package seaserv

import (
	"files/pkg/drivers/sync/seahub/searpc"
	v "github.com/spf13/viper"
	"k8s.io/klog/v2"
	"path/filepath"
)

type SearpcError = searpc.SearpcError
type SeafileRpcClient = searpc.SeafServerThreadedRpcClient

var SeafservThreadedRpc *SeafileRpcClient
var CcnetThreadedRpc *SeafileRpcClient

func InitSeaRPC() {
	initRpcConfig()
	GlobalSeafileAPI = NewSeafileAPI(SeafservThreadedRpc)
	GlobalCcnetAPI = NewCcnetAPI(SeafservThreadedRpc)
}

func initRpcConfig() {
	seafileRpcPath := v.GetString("seafile.rpc.path")
	klog.Infof("[SEAFILE RPC Config] SEAFILE RPC Path: %s", seafileRpcPath)

	initRpcClient(seafileRpcPath)
}

func initRpcClient(pipePath string) {
	if pipePath == "" {
		klog.Infof("Using custom RPC pipe path: %s", pipePath)
	}

	socketPath := filepath.Join(pipePath, "seafile.sock")
	klog.Infof("Creating RPC client with socket path: %s", socketPath)
	SeafservThreadedRpc = searpc.NewSeafServerClient(socketPath)

	CcnetThreadedRpc = SeafservThreadedRpc
	klog.Infof("RPC client initialization completed")
}
