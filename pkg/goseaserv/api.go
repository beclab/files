package goseaserv

import (
	"errors"
	"files/pkg/gosearpc"
	"fmt"
	"k8s.io/klog/v2"
)

const (
	REPO_STATUS_NORMAL    = 0
	REPO_STATUS_READ_ONLY = 1
)

type SeafileAPI struct {
	rpcClient *SeafileRpcClient
}

type Repo struct {
	ID        string `json:"repo_id"`
	Name      string `json:"repo_name"`
	Desc      string `json:"repo_desc"`
	Encrypted bool   `json:"encrypted"`
}

type Dirent struct {
	Type         string `json:"type"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	LastModified int64  `json:"last_modified"`
}

func NewSeafileAPI(rpcClient *SeafileRpcClient) *SeafileAPI {
	klog.Infof("~~~Debug log: Initializing GlobalSeafileAPI...")
	if rpcClient == nil {
		klog.Errorf("rpc client cannot be nil")
		return nil
	}
	return &SeafileAPI{
		rpcClient: rpcClient,
	}
}

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

func (s *SeafileAPI) GetOwnedRepoList(username string, retCorrupted bool, start int, limit int) ([]map[string]string, error) {
	var retCorruptedFlag int
	if retCorrupted {
		retCorruptedFlag = 1
	} else {
		retCorruptedFlag = 0
	}

	ret, err := s.rpcClient.GetOwnedRepoList(username, retCorruptedFlag, start, limit)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - username: %s, error: %v", username, err)
		return nil, err
	}
	klog.Infof("~~~Debug log: RPC call succeeded - username: %s, response type: %T", username, ret)

	objList, ok := ret.([]*gosearpc.SearpcObj)
	if !ok {
		klog.Errorf("~~~Debug log: Type assertion failed - expected: []*gosearpc.SearpcObj, actual: %T", ret)
		return nil, fmt.Errorf("type assertion failed - expected: []*gosearpc.SearpcObj, actual:%T", ret)
	}
	klog.Infof("~~~Debug log: Successfully converted to []*gosearpc.SearpcObj - length: %d", len(objList))

	repos, err := gosearpc.ObjListMapString(objList)
	if err != nil {
		klog.Errorf("~~~Debug log: Parse object list failed - error: %v", err)
	}

	klog.Infof("~~~Debug log: Conversion completed - total users: %d", len(repos))
	return repos, err
}

var GlobalSeafileAPI *SeafileAPI // = NewSeafileAPI(SeafservThreadedRpc)

type CcnetAPI struct {
	rpcClient *SeafileRpcClient
}

func NewCcnetAPI(rpcClient *SeafileRpcClient) *CcnetAPI {
	klog.Infof("~~~Debug log: Initializing GlobalCcnetAPI...")
	if rpcClient == nil {
		klog.Errorf("rpc client cannot be nil")
		return nil
	}

	return &CcnetAPI{
		rpcClient: rpcClient,
	}
}

func (s *CcnetAPI) GetEmailusers(source string, start, limit int, isActive *bool) ([]map[string]string, error) {
	klog.Infof("~~~Debug log: GetEmailusers called - source: %s, start: %d, limit: %d, isActive: %v",
		source, start, limit, isActive)

	var status string
	if isActive != nil {
		if *isActive {
			status = "active"
		} else {
			status = "inactive"
		}
		klog.Infof("~~~Debug log: Status filter set to '%s'", status)
	} else {
		klog.Info("~~~Debug log: No status filter applied (isActive=nil)")
	}

	klog.Infof("~~~Debug log: Calling RPC with params - source: %s, start: %d, limit: %d, status: %s",
		source, start, limit, status)

	ret, err := s.rpcClient.GetEmailusers(source, start, limit, status)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - source: %s, error: %v", source, err)
		return nil, err
	}
	klog.Infof("~~~Debug log: RPC call succeeded - source: %s, response type: %T", source, ret)

	objList, ok := ret.([]*gosearpc.SearpcObj)
	if !ok {
		klog.Errorf("~~~Debug log: Type assertion failed - expected: []*gosearpc.SearpcObj, actual: %T", ret)
		return nil, fmt.Errorf("type assertion failed - expected: []*gosearpc.SearpcObj, actual:%T", ret)
	}
	klog.Infof("~~~Debug log: Successfully converted to []*gosearpc.SearpcObj - length: %d", len(objList))

	users, err := gosearpc.ObjListMapString(objList)
	if err != nil {
		klog.Errorf("~~~Debug log: Parse object list failed - error: %v", err)
	}

	klog.Infof("~~~Debug log: Conversion completed - total users: %d", len(users))
	return users, err
}

func (s *CcnetAPI) CountEmailusers(source string) (int, error) {
	if s.rpcClient == nil {
		klog.Errorf("rpc client cannot be nil")
		s.rpcClient = SeafservThreadedRpc
	}
	if s.rpcClient == nil {
		klog.Errorf("rpc client cannot be nil")
		return 0, fmt.Errorf("rpc client is nil")
	}
	ret, err := s.rpcClient.CountEmailusers(source)
	if err != nil {
		return 0, fmt.Errorf("count email users failed: %v", err)
	}
	return ret.(int), err
}

func (s *CcnetAPI) CountInactiveEmailusers(source string) (int, error) {
	if s.rpcClient == nil {
		klog.Errorf("rpc client cannot be nil")
		s.rpcClient = SeafservThreadedRpc
	}
	if s.rpcClient == nil {
		klog.Errorf("rpc client cannot be nil")
		return 0, fmt.Errorf("rpc client is nil")
	}
	ret, err := s.rpcClient.CountInactiveEmailusers(source)
	if err != nil {
		return 0, fmt.Errorf("count inactive email users failed: %v", err)
	}
	return ret.(int), err
}

var GlobalCcnetAPI *CcnetAPI //= NewCcnetAPI(SeafservThreadedRpc)
