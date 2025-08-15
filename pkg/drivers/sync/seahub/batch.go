package seahub

import (
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/models"
	"fmt"
	"k8s.io/klog/v2"
	"path/filepath"
	"strings"
)

func HandleDelete(fileParam *models.FileParam) error {
	parentDir, filename := filepath.Split(strings.TrimSuffix(fileParam.Path, "/"))
	if filename == "" {
		return errors.New("filename is empty")
	}
	newFileParam := &models.FileParam{
		Owner:    fileParam.Owner,
		FileType: fileParam.FileType,
		Extend:   fileParam.Extend,
		Path:     parentDir,
	}
	dirents := []string{filename}
	_, err := HandleBatchDelete(newFileParam, dirents)
	if err != nil {
		return err
	}
	return nil
}

func HandleBatchDelete(fileParam *models.FileParam, dirents []string) ([]byte, error) {
	repoId := fileParam.Extend
	parentDir := fileParam.Path

	if repoId == "" {
		return nil, errors.New("repoId is empty")
	}
	if parentDir == "" {
		return nil, errors.New("parentDir is empty")
	}
	if len(dirents) == 0 {
		return nil, errors.New("dirents is empty")
	}

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, errors.New("repo is nil")
	}

	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, parentDir)
	if dirId == "" || err != nil {
		klog.Errorf("fail to check dir exists %s, err=%s", parentDir, err)
		return nil, errors.New("folder not found")
	}

	username := fileParam.Owner + "@auth.local"

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil || permission != "rw" {
		return nil, errors.New("permission denied")
	}

	folderPerms, err := GetSubFolderPermissionByDir(username, repoId, parentDir)
	for _, dirent := range dirents {
		if perm, exists := folderPerms[dirent]; exists && perm != "rw" {
			return nil, errors.New(fmt.Sprintf("Can't delete folder %s, please check its permission", dirent))
		}
	}

	if resultCode, err := seaserv.GlobalSeafileAPI.DelFile(repoId, parentDir, string(common.ToBytes(dirents)), username); resultCode != 0 || err != nil {
		klog.Errorf("Failed to delete: result_code: %d, err: %v", resultCode, err)
		return nil, errors.New("failed to delete")
	}

	response := map[string]interface{}{
		"success":   true,
		"commit_id": repo["head_commit_id"],
	}
	return common.ToBytes(response), nil
}

func GetSubFolderPermissionByDir(username, repoID, parentDir string) (map[string]string, error) {
	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoID, parentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get dir id: %v", err)
	}
	if dirId == "" {
		return nil, fmt.Errorf("failed to get dir id")
	}

	dirents, err := seaserv.GlobalSeafileAPI.ListDirWithPerm(repoID, parentDir, dirId, username, -1, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %v", err)
	}

	folderPermissionDict := make(map[string]string)
	for _, dirent := range dirents {
		isDir, err := IsDirectory(dirent["mode"])
		if err != nil {
			return nil, fmt.Errorf("failed to check dir permission: %v", err)
		}
		if isDir {
			folderPermissionDict[dirent["obj_name"]] = dirent["permission"]
		}
	}

	return folderPermissionDict, nil
}

type CopyMoveReq struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

func HandleBatchCopy(owner, srcRepoId, srcParentDir string, srcDirents []string, dstRepoId, dstParentDir string) ([]byte, error) {
	srcRepo, err := seaserv.GlobalSeafileAPI.GetRepo(srcRepoId)
	if err != nil || srcRepo == nil {
		klog.Error(fmt.Sprintf("Library %s not found, err: %v", srcRepoId, err))
		return nil, errors.New("library not found")
	}

	srcDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(srcRepoId, srcParentDir)
	if err != nil || srcDirId == "" {
		klog.Error(fmt.Sprintf("Folder %s not found, err: %v", srcParentDir, err))
		return nil, errors.New("folder not found")
	}

	dstRepo, err := seaserv.GlobalSeafileAPI.GetRepo(dstRepoId)
	if err != nil || dstRepo == nil {
		klog.Error(fmt.Sprintf("Library %s not found, err: %v", dstRepoId, err))
		return nil, errors.New("library not found")
	}

	dstDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(dstRepoId, dstParentDir)
	if err != nil || dstDirId == "" {
		klog.Error(fmt.Sprintf("Folder %s not found, err: %v", dstParentDir, err))
		return nil, errors.New("folder not found")
	}

	username := owner + "@auth.local"

	srcPerm, err := CheckFolderPermission(username, srcRepoId, srcParentDir)
	if err != nil || !strings.Contains(srcPerm, "r") {
		klog.Error("Permission denied.")
		return nil, errors.New("permission denied")
	}

	dstPerm, err := CheckFolderPermission(username, dstRepoId, dstParentDir)
	if err != nil || dstPerm != "rw" {
		klog.Error("Permission denied.")
		return nil, errors.New("permission denied")
	}

	_, err = seaserv.GlobalSeafileAPI.CopyFile(
		srcRepoId, srcParentDir, string(common.ToBytes(srcDirents)),
		dstRepoId, dstParentDir, string(common.ToBytes(srcDirents)),
		username, 0, 1,
	)

	if err != nil {
		klog.Errorf("Copy error: %v", err)
		return nil, err
	}

	response := map[string]interface{}{
		"success": true,
	}
	return common.ToBytes(response), nil
}

func HandleBatchMove(owner, srcRepoId, srcParentDir string, srcDirents []string, dstRepoId, dstParentDir string) ([]byte, error) {
	srcRepo, err := seaserv.GlobalSeafileAPI.GetRepo(srcRepoId)
	if err != nil || srcRepo == nil {
		klog.Error(fmt.Sprintf("Library %s not found, err: %v", srcRepoId, err))
		return nil, errors.New("library not found")
	}

	srcDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(srcRepoId, srcParentDir)
	if err != nil || srcDirId == "" {
		klog.Error(fmt.Sprintf("Folder %s not found, err: %v", srcParentDir, err))
		return nil, errors.New("folder not found")
	}

	dstRepo, err := seaserv.GlobalSeafileAPI.GetRepo(dstRepoId)
	if err != nil || dstRepo == nil {
		klog.Error(fmt.Sprintf("Library %s not found, err: %v", dstRepoId, err))
		return nil, errors.New("library not found")
	}

	dstDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(dstRepoId, dstParentDir)
	if err != nil || dstDirId == "" {
		klog.Error(fmt.Sprintf("Folder %s not found, err: %v", dstParentDir, err))
		return nil, errors.New("folder not found")
	}

	username := owner + "@auth.local"

	srcPerm, err := CheckFolderPermission(username, srcRepoId, srcParentDir)
	if err != nil || srcPerm != "rw" {
		klog.Error("Permission denied.")
		return nil, errors.New("permission denied")
	}

	dstPerm, err := CheckFolderPermission(username, dstRepoId, dstParentDir)
	if err != nil || dstPerm != "rw" {
		klog.Error("Permission denied.")
		return nil, errors.New("permission denied")
	}

	folderPerms, err := GetSubFolderPermissionByDir(username, srcRepoId, srcParentDir)
	for _, dirent := range srcDirents {
		if perm, exists := folderPerms[dirent]; exists {
			if perm != "rw" {
				klog.Errorf("Can't move folder %s, please check its permission.", dirent)
				return nil, fmt.Errorf("cant move folder %s", dirent)
			}
		}
	}

	_, err = seaserv.GlobalSeafileAPI.MoveFile(
		srcRepoId, srcParentDir, string(common.ToBytes(srcDirents)),
		dstRepoId, dstParentDir, string(common.ToBytes(srcDirents)),
		false, username, 0, 1,
	)
	if err != nil {
		klog.Errorf("Copy error: %v", err)
		return nil, err
	}

	response := map[string]interface{}{
		"success": true,
	}
	return common.ToBytes(response), nil
}
