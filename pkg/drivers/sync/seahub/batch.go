package seahub

import (
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/models"
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

// ErrEntryNotFound marks a dirent missing from its parent listing.
// HandleDelete swallows it for idempotent internal cleanup; SyncStorage.Delete
// surfaces it as "file not found" / "folder not found".
type ErrEntryNotFound struct {
	IsDir bool
}

func (e *ErrEntryNotFound) Error() string {
	if e.IsDir {
		return "folder not found"
	}
	return "file not found"
}

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
		var nfe *ErrEntryNotFound
		if errors.As(err, &nfe) {
			klog.Infof("HandleDelete: target already absent, treating as success: %s", fileParam.Path)
			return nil
		}
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
		klog.Error(err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("repo %s not found", repoId)
		return nil, errors.New("repo not found")
	}

	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, parentDir, true)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if dirId == "" {
		klog.Errorf("fail to check dir exists %s, err=%s", parentDir, err)
		return nil, errors.New("folder not found")
	}

	username := fileParam.Owner + "@auth.local"

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil {
		return nil, err
	}
	if permission != "rw" {
		return nil, errors.New("permission denied")
	}

	// One listing serves both existence (del_file is silently idempotent
	// for missing entries) and sub-folder perm checks.
	entries, err := seaserv.GlobalSeafileAPI.ListDirWithPerm(repoId, parentDir, dirId, username, -1, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %v", err)
	}
	existing := make(map[string]bool, len(entries))
	folderPerms := make(map[string]string, len(entries))
	for _, e := range entries {
		existing[e["obj_name"]] = true
		isDir, dErr := IsDirectory(e["mode"])
		if dErr != nil {
			return nil, fmt.Errorf("failed to check dir permission: %v", dErr)
		}
		if isDir {
			folderPerms[e["obj_name"]] = e["permission"]
		}
	}

	for _, dirent := range dirents {
		name := strings.TrimRight(dirent, "/")
		if !existing[name] {
			return nil, &ErrEntryNotFound{IsDir: strings.HasSuffix(dirent, "/")}
		}
	}

	for _, dirent := range dirents {
		name := strings.TrimRight(dirent, "/")
		if perm, ok := folderPerms[name]; ok && perm != "rw" {
			return nil, fmt.Errorf("Can't delete folder %s, please check its permission", name)
		}
	}

	cleanDirents := make([]string, len(dirents))
	for i, d := range dirents {
		cleanDirents[i] = strings.TrimRight(d, "/")
	}
	resultCode, err := seaserv.GlobalSeafileAPI.DelFile(repoId, parentDir, string(common.ToBytes(cleanDirents)), username)
	if err != nil {
		klog.Errorf("Failed to delete: %v", err)
		return nil, err
	}
	if resultCode != 0 {
		klog.Errorf("Failed to delete: result_code: %d", resultCode)
		return nil, fmt.Errorf("failed to delete: result_code: %d", resultCode)
	}

	response := map[string]interface{}{
		"success":   true,
		"commit_id": repo["head_commit_id"],
	}
	return common.ToBytes(response), nil
}

func GetSubFolderPermissionByDir(username, repoID, parentDir string) (map[string]string, error) {
	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoID, parentDir, true)
	if err != nil {
		return nil, err
	}
	if dirId == "" {
		return nil, errors.New("folder not found")
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

// resolveDstDirents returns the slice of names to use for the destination
// side of a batch copy/move. When dstDirents is nil/empty (the historical
// "paste with original name" case) it falls back to srcDirents, preserving
// the prior behavior. When provided, it must be 1:1 with srcDirents.
func resolveDstDirents(srcDirents, dstDirents []string) ([]string, error) {
	if len(dstDirents) == 0 {
		return srcDirents, nil
	}
	if len(dstDirents) != len(srcDirents) {
		return nil, fmt.Errorf("dstDirents length (%d) does not match srcDirents length (%d)", len(dstDirents), len(srcDirents))
	}
	return dstDirents, nil
}

// HandleBatchCopy copies srcDirents from srcParentDir into dstParentDir.
//
// When dstDirents is non-empty it must be the same length as srcDirents and
// supplies the per-item target name (enabling rename-on-copy). When nil/empty
// the destination name equals the source name (existing behavior).
func HandleBatchCopy(owner, srcRepoId, srcParentDir string, srcDirents []string, dstRepoId, dstParentDir string, dstDirents []string) ([]byte, error) {
	srcRepo, err := seaserv.GlobalSeafileAPI.GetRepo(srcRepoId)
	if err != nil {
		return nil, err
	}
	if srcRepo == nil {
		klog.Error(fmt.Sprintf("Library %s not found, err: %v", srcRepoId, err))
		return nil, errors.New("library not found")
	}

	srcDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(srcRepoId, srcParentDir, true)
	if err != nil {
		return nil, err
	}
	if srcDirId == "" {
		klog.Error(fmt.Sprintf("Folder %s not found, err: %v", srcParentDir, err))
		return nil, errors.New("folder not found")
	}

	dstRepo, err := seaserv.GlobalSeafileAPI.GetRepo(dstRepoId)
	if err != nil {
		return nil, err
	}
	if dstRepo == nil {
		klog.Error(fmt.Sprintf("Library %s not found, err: %v", dstRepoId, err))
		return nil, errors.New("library not found")
	}

	dstDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(dstRepoId, dstParentDir, true)
	if err != nil {
		return nil, err
	}
	if dstDirId == "" {
		klog.Error(fmt.Sprintf("Folder %s not found, err: %v", dstParentDir, err))
		return nil, errors.New("folder not found")
	}

	username := owner + "@auth.local"

	srcPerm, err := CheckFolderPermission(username, srcRepoId, srcParentDir)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(srcPerm, "r") {
		klog.Error("Permission denied.")
		return nil, errors.New("permission denied")
	}

	// Require "rw" on dst, mirroring HandleBatchMove. Previously a
	// successful lookup of "r" / "" / "cloud-edit" fell through to
	// CopyFile and relied on seafile to reject it.
	dstPerm, err := CheckFolderPermission(username, dstRepoId, dstParentDir)
	if err != nil {
		return nil, err
	}
	if dstPerm != "rw" {
		klog.Error("Permission denied.")
		return nil, errors.New("permission denied")
	}

	finalDstDirents, err := resolveDstDirents(srcDirents, dstDirents)
	if err != nil {
		return nil, err
	}

	_, err = seaserv.GlobalSeafileAPI.CopyFile(
		srcRepoId, srcParentDir, string(common.ToBytes(srcDirents)),
		dstRepoId, dstParentDir, string(common.ToBytes(finalDstDirents)),
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

// HandleBatchMove moves srcDirents from srcParentDir into dstParentDir.
//
// When dstDirents is non-empty it must be the same length as srcDirents and
// supplies the per-item target name (enabling rename-on-move). When nil/empty
// the destination name equals the source name (existing behavior).
func HandleBatchMove(owner, srcRepoId, srcParentDir string, srcDirents []string, dstRepoId, dstParentDir string, dstDirents []string) ([]byte, error) {
	srcRepo, err := seaserv.GlobalSeafileAPI.GetRepo(srcRepoId)
	if err != nil {
		return nil, err
	}
	if srcRepo == nil {
		klog.Error(fmt.Sprintf("Library %s not found, err: %v", srcRepoId, err))
		return nil, errors.New("library not found")
	}

	srcDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(srcRepoId, srcParentDir, true)
	if err != nil {
		return nil, err
	}
	if srcDirId == "" {
		klog.Error(fmt.Sprintf("Folder %s not found, err: %v", srcParentDir, err))
		return nil, errors.New("folder not found")
	}

	dstRepo, err := seaserv.GlobalSeafileAPI.GetRepo(dstRepoId)
	if err != nil {
		return nil, err
	}
	if dstRepo == nil {
		klog.Error(fmt.Sprintf("Library %s not found, err: %v", dstRepoId, err))
		return nil, errors.New("library not found")
	}

	dstDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(dstRepoId, dstParentDir, true)
	if err != nil {
		return nil, err
	}
	if dstDirId == "" {
		klog.Error(fmt.Sprintf("Folder %s not found, err: %v", dstParentDir, err))
		return nil, errors.New("folder not found")
	}

	username := owner + "@auth.local"

	srcPerm, err := CheckFolderPermission(username, srcRepoId, srcParentDir)
	if err != nil {
		return nil, err
	}
	if srcPerm != "rw" {
		klog.Error("Permission denied.")
		return nil, errors.New("permission denied")
	}

	dstPerm, err := CheckFolderPermission(username, dstRepoId, dstParentDir)
	if err != nil {
		return nil, err
	}
	if dstPerm != "rw" {
		klog.Error("Permission denied.")
		return nil, errors.New("permission denied")
	}

	folderPerms, err := GetSubFolderPermissionByDir(username, srcRepoId, srcParentDir)
	if err != nil {
		klog.Errorf("get sub folder permission failed: %v", err)
		return nil, err
	}
	for _, dirent := range srcDirents {
		if perm, exists := folderPerms[dirent]; exists {
			if perm != "rw" {
				klog.Errorf("Can't move folder %s, please check its permission.", dirent)
				return nil, fmt.Errorf("cant move folder %s", dirent)
			}
		}
	}

	finalDstDirents, err := resolveDstDirents(srcDirents, dstDirents)
	if err != nil {
		return nil, err
	}

	_, err = seaserv.GlobalSeafileAPI.MoveFile(
		srcRepoId, srcParentDir, string(common.ToBytes(srcDirents)),
		dstRepoId, dstParentDir, string(common.ToBytes(finalDstDirents)),
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
