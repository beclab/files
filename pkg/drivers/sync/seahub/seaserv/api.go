package seaserv

import (
	"files/pkg/drivers/sync/seahub/searpc"
	"fmt"
	"k8s.io/klog/v2"
)

const (
	REPO_STATUS_NORMAL    = 0
	REPO_STATUS_READ_ONLY = 1
)

func ReturnBool(ret interface{}) (bool, error) {
	// Return non-zero if True, otherwise 0.
	retInt, err := ReturnInt(ret)
	if err != nil {
		return false, err
	}
	return retInt != 0, nil
}

func ReturnInt(ret interface{}) (int, error) {
	retInt, ok := ret.(int)
	if !ok {
		klog.Errorf("~~~Debug log: Type assertion failed - expected: int, actual: %T", ret)
		return -1, fmt.Errorf("type assertion failed - expected: int, actual:%T", ret)
	}
	klog.Infof("~~~Debug log: Successfully converted to int - %d", retInt)
	return retInt, nil
}

func ReturnInt64(ret interface{}) (int64, error) {
	retInt, ok := ret.(int)
	if !ok {
		klog.Errorf("~~~Debug log: Type assertion failed - expected: int, actual: %T", ret)
		return -1, fmt.Errorf("type assertion failed - expected: int, actual:%T", ret)
	}
	klog.Infof("~~~Debug log: Successfully converted to int - %d", retInt)
	return int64(retInt), nil
}

func ReturnString(ret interface{}) (string, error) {
	retString, ok := ret.(string)
	if !ok {
		klog.Errorf("~~~Debug log: Type assertion failed - expected: string, actual: %T", ret)
		return "", fmt.Errorf("type assertion failed - expected: string, actual:%T", ret)
	}
	klog.Infof("~~~Debug log: Successfully converted to string - %s", retString)
	return retString, nil
}

func ReturnObject(ret interface{}) (map[string]string, error) {
	obj, ok := ret.(*searpc.SearpcObj)
	if !ok {
		klog.Errorf("~~~Debug log: Type assertion failed - expected: *searpc.SearpcObj, actual: %T", ret)
		return nil, fmt.Errorf("type assertion failed - expected: *searpc.SearpcObj, actual:%T", ret)
	}

	if obj == nil {
		klog.Infof("~~~Debug log: Successfully converted to nil - %v", ret)
		return nil, nil
	} else {
		klog.Infof("~~~Debug log: Successfully converted to *searpc.SearpcObj - %v", obj)
	}
	retObject, err := obj.MapString()
	if err != nil {
		klog.Errorf("~~~Debug log: Parse object failed - error: %v", err)
		return nil, err
	}

	klog.Infof("~~~Debug log: Conversion completed - retObject: %v", retObject)
	return retObject, nil
}

func ReturnObjList(ret interface{}) ([]map[string]string, error) {
	objList, ok := ret.([]*searpc.SearpcObj)
	if !ok {
		klog.Errorf("~~~Debug log: Type assertion failed - expected: []*searpc.SearpcObj, actual: %T", ret)
		return nil, fmt.Errorf("type assertion failed - expected: []*searpc.SearpcObj, actual:%T", ret)
	}
	if objList == nil {
		klog.Infof("~~~Debug log: Successfully converted to nil - %v", objList)
		return nil, nil
	} else {
		klog.Infof("~~~Debug log: Successfully converted to []*searpc.SearpcObj - length: %d", len(objList))
	}

	retObjList, err := searpc.ObjListMapString(objList)
	if err != nil {
		klog.Errorf("~~~Debug log: Parse object list failed - error: %v", err)
		return nil, err
	}

	klog.Infof("~~~Debug log: Conversion completed - total objs: %d", len(retObjList))
	return retObjList, nil
}

type SeafileAPI struct {
	rpcClient *SeafileRpcClient
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

func (s *SeafileAPI) GetFileServerAccessToken(repoId, objId, op, username string, useOnetime bool) (string, error) {
	/*
		op: the operation, can be 'view', 'download', 'download-dir', 'downloadblks',
		'upload', 'update', 'upload-blks-api', 'upload-blks-aj',
		'update-blks-api', 'update-blks-aj'

		Return: the access token in string
	*/
	onetime := 0
	if useOnetime {
		onetime = 1
	}
	ret, err := s.rpcClient.SeafileWebGetAccessToken(repoId, objId, op, username, onetime)
	if err != nil {
		return "", err
	}
	return ReturnString(ret)
}

func (s *SeafileAPI) CreateRepo(name, desc, username string, password *string, encVersion int) (string, error) {
	ret, err := s.rpcClient.SeafileCreateRepo(name, desc, username, password, encVersion)
	if err != nil {
		return "", fmt.Errorf("create repo failed: %v", err)
	}
	return ReturnString(ret)
}

func (s *SeafileAPI) GetRepo(repoId string) (map[string]string, error) {
	ret, err := s.rpcClient.SeafileGetRepo(repoId)
	if err != nil {
		return nil, fmt.Errorf("get repo failed: %v", err)
	}
	return ReturnObject(ret)
}

func (s *SeafileAPI) RemoveRepo(repoId string) (int, error) {
	ret, err := s.rpcClient.SeafileDestroyRepo(repoId)
	if err != nil {
		return -1, fmt.Errorf("remove repo failed: %v", err)
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) EditRepo(repoId, name, description, username string) (int, error) {
	ret, err := s.rpcClient.SeafileEditRepo(repoId, name, description, username)
	if err != nil {
		return -1, fmt.Errorf("edit repo failed: %v", err)
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) IsRepoOwner(username, repoId string) (bool, error) {
	ret, err := s.rpcClient.SeafileIsRepoOwner(username, repoId)
	if err != nil {
		return false, fmt.Errorf("check repo owner failed: %v", err)
	}
	retInt, err := ReturnInt(ret)
	if err != nil {
		return false, fmt.Errorf("check repo owner failed: %v", err)
	}
	if retInt == 1 {
		return true, nil
	}
	return false, nil
}

func (s *SeafileAPI) GetRepoOwner(repoId string) (string, error) {
	ret, err := s.rpcClient.SeafileGetRepoOwner(repoId)
	if err != nil {
		return "", fmt.Errorf("get repo owner failed: %v", err)
	}
	return ReturnString(ret)
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
	return ReturnObjList(ret)
}

func (s *SeafileAPI) GenerateRepoToken(repoId, username string) (string, error) {
	ret, err := s.rpcClient.SeafileGanarateRepoToken(repoId, username)
	if err != nil {
		return "", fmt.Errorf("generate repo token failed: %v", err)
	}
	return ReturnString(ret)
}

func (s *SeafileAPI) DeleteRepoTokensByEmail(email string) (int, error) {
	ret, err := s.rpcClient.DeleteRepoTokensByEmail(email)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - email: %s, error: %v", email, err)
		return -1, err
	}
	klog.Infof("~~~Debug log: RPC call succeeded - email: %s, response type: %T", email, ret)
	return ReturnInt(ret)
}

func (s *SeafileAPI) GetSystemDefaultRepoId() (string, error) {
	ret, err := s.rpcClient.GetSystemDefaultRepoId()
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return "", err
	}
	klog.Infof("~~~Debug log: RPC call succeeded - response type: %T", ret)
	return ReturnString(ret)
}

func (s *SeafileAPI) GetDirIdByPath(repoId, path string) (string, error) {
	ret, err := s.rpcClient.SeafileGetDirIdByPath(repoId, path)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return "", err
	}
	return ReturnString(ret)
}

func (s *SeafileAPI) GetCommitList(repoId string, offset, limit int) ([]map[string]string, error) {
	ret, err := s.rpcClient.SeafileGetCommitList(repoId, offset, limit)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObjList(ret)
}

func (s *SeafileAPI) GetCommit(repoId string, repoVersion int, cmtId string) (map[string]string, error) {
	ret, err := s.rpcClient.SeafileGetCommit(repoId, repoVersion, cmtId)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObject(ret)
}

func (s *SeafileAPI) ListDirWithPerm(repoId, dirPath, dirId, user string, offset, limit int) ([]map[string]string, error) {
	ret, err := s.rpcClient.ListDirWithPerm(repoId, dirPath, dirId, user, offset, limit)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObjList(ret)
}

func (s *SeafileAPI) ListDirByDirId(repoId, dirId string, offset, limit int) ([]map[string]string, error) {
	ret, err := s.rpcClient.SeafileListDir(repoId, dirId, offset, limit)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObjList(ret)
}

func (s *SeafileAPI) ListDirByPath(repoId, path string, offset, limit int) ([]map[string]string, error) {
	dirIdInterface, err := s.rpcClient.SeafileGetDirIdByPath(repoId, path)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	dirId, err := ReturnString(dirIdInterface)
	if err != nil {
		return nil, err
	}
	if dirId == "" {
		return nil, nil
	}
	ret, err := s.rpcClient.SeafileListDir(repoId, dirId, offset, limit)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObjList(ret)
}

func (s *SeafileAPI) ListDirByCommitAndPath(repoId, commitId, path string, offset, limit int) ([]map[string]string, error) {
	dirIdInterface, err := s.rpcClient.SeafileGetDirIdByCommitAndPath(repoId, commitId, path)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	dirId, err := ReturnString(dirIdInterface)
	if err != nil {
		return nil, err
	}
	if dirId == "" {
		return nil, nil
	}
	ret, err := s.rpcClient.SeafileListDir(repoId, dirId, offset, limit)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObjList(ret)
}

func (s *SeafileAPI) GetFileSize(storeId string, version int, fileId string) (int64, error) {
	ret, err := s.rpcClient.SeafileGetFileSize(storeId, version, fileId)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return 0, err
	}
	return ReturnInt64(ret)
}

func (s *SeafileAPI) GetFileIdByPath(repoId, path string) (string, error) {
	ret, err := s.rpcClient.SeafileGetFileIdByPath(repoId, path)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return "", err
	}
	return ReturnString(ret)
}

func (s *SeafileAPI) GetDirIdByCommitAndPath(repoId, commitId, path string) (string, error) {
	ret, err := s.rpcClient.SeafileGetDirIdByCommitAndPath(repoId, commitId, path)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return "", err
	}
	return ReturnString(ret)
}

func (s *SeafileAPI) GetDirentByPath(repoId, path string) (map[string]string, error) {
	ret, err := s.rpcClient.SeafileGetDirentByPath(repoId, path)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObject(ret)
}

func (s *SeafileAPI) DelFile(repoId, parentDir, filename, username string) (int, error) {
	ret, err := s.rpcClient.SeafileDelFile(repoId, parentDir, filename, username)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) CopyFile(srcRepo, srcDir, srcFilename, dstRepo, dstDir, dstFilename, username string, needProgress, synchronous int) (map[string]string, error) {
	ret, err := s.rpcClient.SeafileCopyFile(srcRepo, srcDir, srcFilename, dstRepo, dstDir, dstFilename, username, needProgress, synchronous)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObject(ret)
}

func (s *SeafileAPI) RenameFile(repoId, parentDir, oldname, newname, username string) (int, error) {
	ret, err := s.rpcClient.SeafileRenameFile(repoId, parentDir, oldname, newname, username)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) PostDir(repoId, parentDir, dirname, username string) (int, error) {
	ret, err := s.rpcClient.SeafilePostDir(repoId, parentDir, dirname, username)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) IsValidFilename(repoId, filename string) (int, error) {
	// Return: 0 on invalid; 1 on valid.
	ret, err := s.rpcClient.SeafileIsValidFilename(repoId, filename)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return 0, err
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) GetUploadTmpFileOffset(repoId, filePath string) (int, error) {
	ret, err := s.rpcClient.SeafileGetUploadTmpFileOffset(repoId, filePath)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) RemoveShare(repoId, fromUsername, toUsername string) (int, error) {
	ret, err := s.rpcClient.SeafileRemoveShare(repoId, fromUsername, toUsername)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) GetShareInRepoList(username string, start, limit int) ([]map[string]string, error) {
	ret, err := s.rpcClient.SeafileListShareRepos(username, "to_email", start, limit)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return nil, err
	}
	return ReturnObjList(ret)
}

func (s *SeafileAPI) IsInnerPubRepo(repoId string) (bool, error) {
	ret, err := s.rpcClient.IsInnerPubRepo(repoId)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return false, err
	}
	return ReturnBool(ret)
}

func (s *SeafileAPI) CheckQuota(repoId string, delta int64) (int, error) {
	ret, err := s.rpcClient.CheckQuota(repoId, delta)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) CheckPermissionByPath(repoId, path, user string) (string, error) {
	ret, err := s.rpcClient.CheckPermissionByPath(repoId, path, user)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return "", err
	}
	return ReturnString(ret)
}

func (s *SeafileAPI) RepoHasBeenShared(repoId string, includingGroups bool) (bool, error) {
	var iGint int = 0
	if includingGroups {
		iGint = 1
	}
	ret, err := s.rpcClient.RepoHasBeenShared(repoId, iGint)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return false, err
	}
	return ReturnBool(ret)
}

func (s *SeafileAPI) GetRepoStatus(repoId string) (int, error) {
	ret, err := s.rpcClient.GetRepoStatus(repoId)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

func (s *SeafileAPI) IsPasswordSet(repoId, username string) (bool, error) {
	ret, err := s.rpcClient.SeafileIsPasswdSet(repoId, username)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return false, err
	}
	return ReturnBool(ret)
}

func (s *SeafileAPI) PublishEvent(channel, content string) (int, error) {
	ret, err := s.rpcClient.PublishEvent(channel, content)
	if err != nil {
		klog.Errorf("~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

var GlobalSeafileAPI *SeafileAPI

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

func (s *CcnetAPI) AddEmailuser(email string, passwd string, isStaff int, isActive int) (int, error) {
	ret, err := s.rpcClient.AddEmailuser(email, passwd, isStaff, isActive)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	klog.Infof("~~~Debug log: RPC call succeeded - response type: %T", ret)
	return ReturnInt(ret)
}

func (s *CcnetAPI) RemoveEmailuser(source, email string) (int, error) {
	ret, err := s.rpcClient.RemoveEmailuser(source, email)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

func (s *CcnetAPI) GetEmailuser(email string) (map[string]string, error) {
	klog.Infof("~~~Debug log: GetEmailuser called - email: %s", email)

	ret, err := s.rpcClient.GetEmailuser(email)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - email: %s, error: %v", email, err)
		return nil, err
	}
	klog.Infof("~~~Debug log: RPC call succeeded - email: %s, response type: %T", email, ret)
	return ReturnObject(ret)
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
	return ReturnObjList(ret)
}

func (s *CcnetAPI) CountEmailusers(source string) (int, error) {
	ret, err := s.rpcClient.CountEmailusers(source)
	if err != nil {
		return 0, fmt.Errorf("count email users failed: %v", err)
	}
	return ReturnInt(ret)
}

func (s *CcnetAPI) CountInactiveEmailusers(source string) (int, error) {
	ret, err := s.rpcClient.CountInactiveEmailusers(source)
	if err != nil {
		return 0, fmt.Errorf("count inactive email users failed: %v", err)
	}
	return ReturnInt(ret)
}

func (s *CcnetAPI) UpdateEmailuser(source string, userId int, password string, isStaff int, isActive int) (int, error) {
	ret, err := s.rpcClient.UpdateEmailuser(source, userId, password, isStaff, isActive)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	klog.Infof("~~~Debug log: RPC call succeeded - response type: %T", ret)
	return ReturnInt(ret)
}

func (s *CcnetAPI) RemoveGroupUser(username string) (int, error) {
	ret, err := s.rpcClient.RemoveGroupUser(username)
	if err != nil {
		klog.Errorf("~~~Debug log: RPC call failed - error: %v", err)
		return -1, err
	}
	return ReturnInt(ret)
}

var GlobalCcnetAPI *CcnetAPI
