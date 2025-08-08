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

	klog.Infof("~~~Debug log: SeafservThreadedRpc: %+v", SeafservThreadedRpc)
	klog.Infof("~~~Debug log: SeafservThreadedRpc.NamedPipeClient: %+v", SeafservThreadedRpc.NamedPipeClient)
}

func initRpcConfig() {
	seafileRpcPath := v.GetString("seafile.rpc.path")
	klog.Infof("[SEAFILE RPC Config] SEAFILE RPC Path: %s", seafileRpcPath)

	initRpcClient(seafileRpcPath)
}

func initRpcClient(pipePath string) {
	if pipePath == "" {
		klog.Infof("~~~Debug log: Using custom RPC pipe path: %s", pipePath)
	}

	socketPath := filepath.Join(pipePath, "seafile.sock")
	klog.Infof("~~~Debug log: Creating RPC client with socket path: %s", socketPath)
	SeafservThreadedRpc = searpc.NewSeafServerClient(socketPath)
	klog.Infof("~~~Debug log: Successfully created SeafservThreadedRpc client")

	CcnetThreadedRpc = SeafservThreadedRpc
	klog.Infof("~~~Debug log: RPC client initialization completed")
}
