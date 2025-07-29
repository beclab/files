package seahub

import (
	"encoding/json"
	"errors"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/models"
	"fmt"
	"k8s.io/klog/v2"
	"net/http"
)

func HandleBatchDelete(header http.Header, fileParam *models.FileParam, dirents []string) ([]byte, error) {
	MigrateSeahubUserToRedis(header)

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

	bflName := header.Get("X-Bfl-User")
	username := bflName + "@auth.local"
	oldUsername := seaserv.GetOldUsername(bflName + "@seafile.com") // temp compatible
	useUsername := username

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil || permission != "rw" {
		permission, err = CheckFolderPermission(oldUsername, repoId, parentDir) // temp compatible
		if err != nil || permission != "rw" {
			return nil, errors.New("permission denied")
		} else {
			useUsername = oldUsername
		}
	}

	folderPerms, err := GetSubFolderPermissionByDir(useUsername, repoId, parentDir)
	for _, dirent := range dirents {
		if perm, exists := folderPerms[dirent]; exists && perm != "rw" {
			return nil, errors.New(fmt.Sprintf("Can't delete folder %s, please check its permission", dirent))
		}
	}

	direntsJSON, _ := json.Marshal(dirents)
	if resultCode, err := seaserv.GlobalSeafileAPI.DelFile(repoId, parentDir, string(direntsJSON), useUsername); resultCode != 0 || err != nil {
		klog.Errorf("Failed to delete: result_code: %d, err: %v", resultCode, err)
		return nil, errors.New("failed to delete")
	}

	response := map[string]interface{}{
		"success":   true,
		"commit_id": repo["head_commit_id"],
	}
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
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
