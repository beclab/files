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
