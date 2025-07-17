package goseaserv

import (
	"errors"
	"fmt"
)

// 常量定义
const (
	REPO_STATUS_NORMAL    = 0
	REPO_STATUS_READ_ONLY = 1
)

// 核心结构体
type SeafileAPI struct {
	rpcClient *SeafileRpcClient
}

// 仓库对象
type Repo struct {
	ID        string `json:"repo_id"`
	Name      string `json:"repo_name"`
	Desc      string `json:"repo_desc"`
	Encrypted bool   `json:"encrypted"`
}

// 目录项对象
type Dirent struct {
	Type         string `json:"type"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	LastModified int64  `json:"last_modified"`
}

// 构造函数
func NewSeafileAPI(rpcClient *SeafileRpcClient) *SeafileAPI {
	return &SeafileAPI{
		rpcClient: rpcClient,
	}
}

// 仓库操作示例
func (s *SeafileAPI) CreateRepo(name, desc, username string, password string, encVersion int) (*Repo, error) {
	result, err := s.rpcClient.SeafileCreateRepo(name, desc, username, password, encVersion)
	if err != nil {
		return nil, fmt.Errorf("create repo failed: %v", err)
	}

	repoData, ok := result.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid repo data format")
	}

	return &Repo{
		ID:   repoData["repo_id"].(string),
		Name: repoData["repo_name"].(string),
	}, nil
}

// 全局实例
var GlobalSeafileAPI = NewSeafileAPI(SeafservThreadedRpc)

type CcnetAPI struct {
	rpcClient *SeafileRpcClient
}

func NewCcnetAPI(rpcClient *SeafileRpcClient) *CcnetAPI {
	return &CcnetAPI{
		rpcClient: rpcClient,
	}
}

func (s *CcnetAPI) GetEmailusers(source string, start, limit int, isActive *bool) ([]map[string]string, error) {
	var status string
	if isActive != nil {
		if *isActive {
			status = "active"
		} else {
			status = "inactive"
		}
	}
	ret, err := s.rpcClient.GetEmailusers(source, start, limit, status)
	return ret.([]map[string]string), err
}

func (s *CcnetAPI) CountEmailusers(source string) (int, error) {
	ret, err := s.rpcClient.CountEmailusers(source)
	if err != nil {
		return 0, fmt.Errorf("count email users failed: %v", err)
	}
	return ret.(int), err
}

func (s *CcnetAPI) CountInactiveEmailusers(source string) (int, error) {
	ret, err := s.rpcClient.CountInactiveEmailusers(source)
	if err != nil {
		return 0, fmt.Errorf("count inactive email users failed: %v", err)
	}
	return ret.(int), err
}

var GlobalCcnetAPI = NewCcnetAPI(SeafservThreadedRpc)
