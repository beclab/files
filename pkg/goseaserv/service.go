package goseaserv

import (
	"files/pkg/goseafile"
	"files/pkg/gosearpc"
	"fmt"
	"github.com/go-ini/ini"
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
	loadEnvConfig()

	// 初始化RPC客户端
	initRpcClient()

	// 加载配置文件
	loadServerConfig()
}

func loadEnvConfig() {
	debug := os.Getenv(DebugEnv) != ""

	loadPath := func(key string, check bool) (string, error) {
		value := os.Getenv(key)
		if value == "" && check {
			return "", fmt.Errorf("environment variable %s is undefined", key)
		}
		if debug && value != "" {
			log.Printf("Loading %s from %s", key, value)
		}
		return filepath.Clean(os.ExpandEnv(value)), nil
	}

	var err error
	CCNET_CONF_PATH, err = loadPath("CCNET_CONF_DIR", true)
	if err != nil {
		log.Fatal(err)
	}

	SEAFILE_CONF_DIR, err = loadPath("SEAFILE_CONF_DIR", true)
	if err != nil {
		log.Fatal(err)
	}

	SEAFILE_CENTRAL_CONF_DIR, _ = loadPath("SEAFILE_CENTRAL_CONF_DIR", false)
	SEAFILE_RPC_PIPE_PATH, _ = loadPath("SEAFILE_RPC_PIPE_PATH", false)
}

func initRpcClient() {
	pipePath := SEAFILE_RPC_PIPE_PATH
	if pipePath == "" {
		pipePath = SEAFILE_CONF_DIR
	}
	SeafservThreadedRpc = goseafile.NewSeafServerClient(filepath.Join(pipePath, "seafile.sock"))
	CcnetThreadedRpc = SeafservThreadedRpc
}

func loadServerConfig() {
	// 加载ccnet配置
	ccnetPath := getConfigPath("ccnet.conf")
	loadCcnetConfig(ccnetPath)

	// 加载seafile配置
	seafilePath := getSeafileConfigPath("seafile.conf")
	loadSeafileConfig(seafilePath)
}

func getConfigPath(filename string) string {
	if SEAFILE_CENTRAL_CONF_DIR != "" {
		return filepath.Join(SEAFILE_CENTRAL_CONF_DIR, filename)
	}
	return filepath.Join(CCNET_CONF_PATH, filename)
}

func getSeafileConfigPath(filename string) string {
	if SEAFILE_CONF_DIR != "" {
		return filepath.Join(SEAFILE_CONF_DIR, filename)
	}
	return filepath.Join(SEAFILE_CENTRAL_CONF_DIR, filename)
}

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
