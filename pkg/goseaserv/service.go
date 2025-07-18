package goseaserv

import (
	"files/pkg/goseafile"
	"files/pkg/gosearpc"
	"fmt"
	"github.com/go-ini/ini"
	"k8s.io/klog/v2"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type SearpcError = gosearpc.SearpcError
type SeafileRpcClient = goseafile.SeafServerThreadedRpcClient

// 环境变量配置
const (
	DebugEnv = "SEAFILE_DEBUG"
)

var (
	// 配置路径
	CCNET_CONF_PATH          string
	SEAFILE_CONF_DIR         string
	SEAFILE_CENTRAL_CONF_DIR string
	SEAFILE_RPC_PIPE_PATH    string

	// 服务配置
	LDAP_HOST             string
	FILE_SERVER_PORT      string
	MAX_UPLOAD_FILE_SIZE  int64
	MAX_DOWNLOAD_DIR_SIZE int64 = 100 * (1 << 20) // 默认100MB
	USE_GO_FILESERVER     bool
	CALC_SHARE_USAGE      bool

	// RPC客户端
	SeafservThreadedRpc *SeafileRpcClient
	CcnetThreadedRpc    *SeafileRpcClient
)

// 初始化配置
func init() {
	// 加载环境变量
	klog.Infof("~~~Debug log: loading Env Config")
	loadEnvConfig()

	// 初始化RPC客户端
	klog.Infof("~~~Debug log: initializing rpc client")
	initRpcClient()

	// 加载配置文件
	klog.Infof("~~~Debug log: loading Server Config")
	loadServerConfig()

	klog.Infof("~~~Debug log: initializing APIs")
	GlobalSeafileAPI = NewSeafileAPI(SeafservThreadedRpc)
	GlobalCcnetAPI = NewCcnetAPI(SeafservThreadedRpc)

	klog.Infof("~~~Debug log: SeafservThreadedRpc: %+v", SeafservThreadedRpc)
	klog.Infof("~~~Debug log: SeafservThreadedRpc.NamedPipeClient: %+v", SeafservThreadedRpc.NamedPipeClient)
}

func loadEnvConfig() {
	klog.Infof("~~~Debug log: Starting environment configuration loading")
	debug := os.Getenv(DebugEnv) != ""
	klog.Infof("~~~Debug log: Debug mode enabled: %v", debug)

	loadPath := func(key string, check bool) (string, error) {
		klog.Infof("~~~Debug log: Loading environment variable %s (required: %v)", key, check)
		value := os.Getenv(key)
		if value == "" {
			if check {
				klog.Errorf("~~~Debug log: Missing required environment variable %s", key)
				return "", fmt.Errorf("environment variable %s is undefined", key)
			} else {
				klog.Infof("~~~Debug log: Missing optional environment variable %s", key)
				return "", nil
			}
		}
		if debug && value != "" {
			klog.Infof("~~~Debug log: Environment variable %s resolved to: %s", key, value)
		}
		cleaned := filepath.Clean(os.ExpandEnv(value))
		klog.Infof("~~~Debug log: Cleaned path for %s: %s", key, cleaned)
		return cleaned, nil
	}

	var err error
	klog.Infof("~~~Debug log: Loading CCNET_CONF_DIR...")
	CCNET_CONF_PATH, err = loadPath("CCNET_CONF_DIR", true)
	if err != nil {
		klog.Fatalf("~~~Debug log: Failed to load CCNET_CONF_DIR: %v", err)
	}
	klog.Infof("~~~Debug log: Successfully loaded CCNET_CONF_DIR: %s", CCNET_CONF_PATH)

	klog.Infof("~~~Debug log: Loading SEAFILE_CONF_DIR...")
	SEAFILE_CONF_DIR, err = loadPath("SEAFILE_CONF_DIR", true)
	if err != nil {
		klog.Fatalf("~~~Debug log: Failed to load SEAFILE_CONF_DIR: %v", err)
	}
	klog.Infof("~~~Debug log: Successfully loaded SEAFILE_CONF_DIR: %s", SEAFILE_CONF_DIR)

	klog.Infof("~~~Debug log: Loading optional SEAFILE_CENTRAL_CONF_DIR...")
	if centralConf, err := loadPath("SEAFILE_CENTRAL_CONF_DIR", false); err == nil {
		SEAFILE_CENTRAL_CONF_DIR = centralConf
		klog.Infof("~~~Debug log: Loaded optional SEAFILE_CENTRAL_CONF_DIR: %s", centralConf)
	}

	klog.Infof("~~~Debug log: Loading optional SEAFILE_RPC_PIPE_PATH...")
	if rpcPath, err := loadPath("SEAFILE_RPC_PIPE_PATH", false); err == nil {
		SEAFILE_RPC_PIPE_PATH = rpcPath
		klog.Infof("~~~Debug log: Loaded optional SEAFILE_RPC_PIPE_PATH: %s", rpcPath)
	}
}

func initRpcClient() {
	klog.Infof("~~~Debug log: Initializing RPC client...")
	pipePath := SEAFILE_RPC_PIPE_PATH
	if pipePath == "" {
		klog.Infof("~~~Debug log: Using default RPC pipe path from SEAFILE_CONF_DIR: %s", SEAFILE_CONF_DIR)
		pipePath = SEAFILE_CONF_DIR
	} else {
		klog.Infof("~~~Debug log: Using custom RPC pipe path: %s", pipePath)
	}

	socketPath := filepath.Join(pipePath, "seafile.sock")
	klog.Infof("~~~Debug log: Creating RPC client with socket path: %s", socketPath)
	SeafservThreadedRpc = goseafile.NewSeafServerClient(socketPath)
	klog.Infof("~~~Debug log: Successfully created SeafservThreadedRpc client")

	klog.Infof("~~~Debug log: Assigning CcnetThreadedRpc to SeafservThreadedRpc instance")
	CcnetThreadedRpc = SeafservThreadedRpc
	klog.Infof("~~~Debug log: RPC client initialization completed")
}

func loadServerConfig() {
	klog.Infof("~~~Debug log: Starting server configuration loading")

	// 加载ccnet配置
	ccnetPath := getConfigPath("ccnet.conf")
	klog.Infof("~~~Debug log: Loading ccnet configuration from: %s", ccnetPath)
	loadCcnetConfig(ccnetPath)
	klog.Infof("~~~Debug log: Successfully loaded ccnet configuration")

	// 加载seafile配置
	seafilePath := getConfigPath("seafile.conf")
	klog.Infof("~~~Debug log: Loading seafile configuration from: %s", seafilePath)
	loadSeafileConfig(seafilePath)
	klog.Infof("~~~Debug log: Successfully loaded seafile configuration")

	klog.Infof("~~~Debug log: Server configuration loading completed")
}

func getConfigPath(filename string) string {
	if SEAFILE_CENTRAL_CONF_DIR != "" {
		return filepath.Join(SEAFILE_CENTRAL_CONF_DIR, filename)
	}
	return filepath.Join(CCNET_CONF_PATH, filename)
}

//func getSeafileConfigPath(filename string) string {
//	if SEAFILE_CONF_DIR != "" {
//		return filepath.Join(SEAFILE_CONF_DIR, filename)
//	}
//	return filepath.Join(SEAFILE_CENTRAL_CONF_DIR, filename)
//}

func loadCcnetConfig(path string) {
	cfg, err := ini.Load(path)
	if err != nil {
		log.Fatal("Failed to read ccnet config: ", err)
	}

	if section := cfg.Section("LDAP"); section != nil {
		LDAP_HOST = section.Key("HOST").String()
	}
}

func loadSeafileConfig(path string) {
	cfg, err := ini.Load(path)
	if err != nil {
		log.Fatal("Failed to read seafile config: ", err)
	}

	// 文件服务器配置
	FILE_SERVER_PORT = getFileserverOption(cfg, "port", "8082")

	// 解析上传限制
	if maxUpload, err := strconv.Atoi(getFileserverOption(cfg, "max_upload_size", "0")); err == nil && maxUpload > 0 {
		MAX_UPLOAD_FILE_SIZE = int64(maxUpload) * (1 << 20)
	}

	// 解析下载目录限制
	if maxDirSize, err := strconv.Atoi(getFileserverOption(cfg, "max_download_dir_size", "0")); err == nil && maxDirSize > 0 {
		MAX_DOWNLOAD_DIR_SIZE = int64(maxDirSize) * (1 << 20)
	}

	// 使用Go文件服务器标志
	if section := cfg.Section("fileserver"); section != nil {
		if useGo, err := section.Key("use_go_fileserver").Bool(); err == nil {
			USE_GO_FILESERVER = useGo
		}
	}

	// 共享存储计算标志
	if section := cfg.Section("quota"); section != nil {
		if calcShare, err := section.Key("calc_share_usage").Bool(); err == nil {
			CALC_SHARE_USAGE = calcShare
		}
	}
}

func getFileserverOption(cfg *ini.File, key, defaultValue string) string {
	for _, section := range []string{"fileserver", "httpserver"} {
		if s := cfg.Section(section); s != nil {
			if value := s.Key(key).String(); value != "" {
				return value
			}
		}
	}
	return defaultValue
}

// 基础API函数
func GetEmailusers(source string, start, limit int, isActive *bool) ([]map[string]string, error) {
	var status string
	if isActive != nil {
		if *isActive {
			status = "active"
		} else {
			status = "inactive"
		}
	}
	ret, err := SeafservThreadedRpc.GetEmailusers(source, start, limit, status)
	return ret.([]map[string]string), err
}

//func main() {
//	// 示例用法
//	users, err := GetEmailusers("test", 0, 10, nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println("Got users:", users)
//}
