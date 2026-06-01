package seahub

import (
	"errors"
	"strings"

	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/models"

	"k8s.io/klog/v2"
)

// ErrSyncPermissionDenied is the sentinel returned by EnsureSyncPermission
// when the resolved Level does not allow the requested action.
var ErrSyncPermissionDenied = errors.New("permission denied")

// Seam over the Seafile RPC so the permission helpers can be unit-tested
// without a live rpc client. Production wiring points at GlobalSeafileAPI;
// tests override these and restore them in t.Cleanup.
var (
	getRepoStatus = func(repoId string) (int, error) {
		return seaserv.GlobalSeafileAPI.GetRepoStatus(repoId)
	}
	checkPermissionByPath = func(repoId, path, user string) (string, error) {
		return seaserv.GlobalSeafileAPI.CheckPermissionByPath(repoId, path, user)
	}
)

// SyncParentDir returns the directory that contains path, so per-folder
// Seafile permission queries hit the same folder across the codebase. A
// file path resolves to its containing directory; a directory (with or
// without a trailing slash) resolves to itself's parent's terminating
// slash form. Root and bare names resolve to "/".
func SyncParentDir(path string) string {
	parent := strings.TrimSuffix(path, "/")
	if parent == "" {
		return "/"
	}
	if pos := strings.LastIndex(parent, "/"); pos >= 0 {
		return parent[:pos+1]
	}
	return "/"
}

// CheckFolderPermission returns the raw Seafile permission string for the
// user on repoId/path. A read-only repo status short-circuits to read.
func CheckFolderPermission(username, repoId, path string) (string, error) {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	klog.Infof("[sync] username: %s, repoId: %s, path: %s", username, repoId, path)
	repoStatus, err := getRepoStatus(repoId)
	if err != nil {
		return "", err
	}
	if repoStatus == seaserv.REPO_STATUS_READ_ONLY {
		return PERMISSION_READ, nil
	}
	result, err := checkPermissionByPath(repoId, path, username)
	if err != nil {
		return "", err
	}
	result = strings.Trim(result, " ")
	klog.Infof("[sync] %s!!", result)
	return result, nil
}

// ResolveSyncLevel resolves the unified Level a user has on repoId/path,
// turning the raw Seafile permission string into the shared model.
func ResolveSyncLevel(username, repoId, path string) (models.Level, error) {
	perm, err := CheckFolderPermission(username, repoId, path)
	if err != nil {
		return models.LevelNone, err
	}
	return models.LevelFromSyncPermission(strings.TrimSpace(perm)), nil
}

// EnsureSyncPermission returns nil when the user may perform action on
// repoId/path, ErrSyncPermissionDenied when the resolved Level forbids it,
// or the underlying error when resolution fails.
func EnsureSyncPermission(username, repoId, path string, action models.Action) error {
	lvl, err := ResolveSyncLevel(username, repoId, path)
	if err != nil {
		return err
	}
	if !lvl.Allow(action) {
		klog.Warningf("[sync] permission denied: user=%s repo=%s path=%s action=%v level=%v", username, repoId, path, action, lvl)
		return ErrSyncPermissionDenied
	}
	return nil
}
