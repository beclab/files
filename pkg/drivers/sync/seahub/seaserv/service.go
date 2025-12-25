package seaserv

import (
	"files/pkg/drivers/sync/seahub/searpc"
	"path/filepath"
	"sync"
	"time"

	v "github.com/spf13/viper"
	"k8s.io/klog/v2"
)

type SearpcError = searpc.SearpcError
type SeafileRpcClient = searpc.SeafServerThreadedRpcClient

var SeafservThreadedRpc *SeafileRpcClient
var CcnetThreadedRpc *SeafileRpcClient

func InitSeaRPC() {
	initRpcConfig()
	GlobalSeafileAPI = NewSeafileAPI(SeafservThreadedRpc)
	GlobalCcnetAPI = NewCcnetAPI(SeafservThreadedRpc)

	go startPeriodicCheckSeaRPC()
}

func startPeriodicCheckSeaRPC() {
	var initLock sync.Mutex

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		res, err := GlobalCcnetAPI.CountEmailusers("DB")
		if err != nil {
			klog.Errorf("check seafile rpc error: %v", err)
		}

		if res < 0 || err != nil {
			initLock.Lock()
			initRpcConfig()
			GlobalSeafileAPI = NewSeafileAPI(SeafservThreadedRpc)
			GlobalCcnetAPI = NewCcnetAPI(SeafservThreadedRpc)
			initLock.Unlock()
		}
	}
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
