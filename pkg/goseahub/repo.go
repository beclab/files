package goseahub

import (
	"files/pkg/common"
	"files/pkg/goseahub/goseaserv"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
	"net/http"
	"strconv"
)

var (
	PERMISSION_PREVIEW       = "preview"    // preview only on the web, can not be downloaded
	PERMISSION_PREVIEW_EDIT  = "cloud-edit" // preview only with edit on the web
	PERMISSION_READ          = "r"
	PERMISSION_READ_WRITE    = "rw"
	PERMISSION_ADMIN         = "admin"
	CUSTOM_PERMISSION_PREFIX = "custom"
)

func CheckFolderPermission(username, repoId, path string) (string, error) {
	repoStatus, err := goseaserv.GlobalSeafileAPI.GetRepoStatus(repoId)
	if err != nil {
		return "", err
	}
	if repoStatus == 1 {
		return PERMISSION_READ, nil
	}
	result, err := goseaserv.GlobalSeafileAPI.CheckPermissionByPath(repoId, path, username)
	if err != nil {
		return "", err
	}
	klog.Infof("%s!!", result)
	return result, nil
}

func repoHasBeenSharedOut(repoId string) (bool, error) {
	shared, err := goseaserv.GlobalSeafileAPI.RepoHasBeenShared(repoId, true)
	if err != nil {
		return false, err
	}
	inner, err := goseaserv.GlobalSeafileAPI.IsInnerPubRepo(repoId)
	if err != nil {
		return false, err
	}
	if shared || inner {
		return true, nil
	}
	return false, nil
}

func RepoGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	vars := mux.Vars(r)
	repoId := vars["repo_id"]

	bflName := r.Header.Get("X-Bfl-User")
	username := bflName + "@auth.local"
	oldUsername := goseaserv.GetOldUsername(bflName + "@seafile.com") // temp compatible

	repo, err := goseaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return http.StatusNotFound, err
	}

	permission, err := CheckFolderPermission(username, repoId, "/")
	if err != nil || permission == "" {
		permission, err = CheckFolderPermission(oldUsername, repoId, "/") // temp compatible
		if err != nil || permission == "" {
			return http.StatusForbidden, err
		}
		// return http.StatusForbidden, err
	}

	libNeedDecrypt := false
	encrypted, err := strconv.ParseBool(repo["encrypted"])
	if err != nil {
		return http.StatusBadRequest, err
	}
	passwordSet, err := goseaserv.GlobalSeafileAPI.IsPasswordSet(repoId, username)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	oldPasswordSet, err := goseaserv.GlobalSeafileAPI.IsPasswordSet(repoId, oldUsername) // temp compatible
	if err != nil {
		return http.StatusInternalServerError, err
	}
	passwordSet = passwordSet || oldPasswordSet

	if encrypted && passwordSet {
		libNeedDecrypt = true
	}

	repoOwner, err := goseaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	hasSharedOut, err := repoHasBeenSharedOut(repoId)
	if err != nil {
		klog.Error(err)
		hasSharedOut = false
	}

	quota, err := goseaserv.GlobalSeafileAPI.CheckQuota(repoId, 0)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	isVirtual, err := strconv.ParseBool(repo["is_virtual"])
	if err != nil {
		klog.Errorf("Error parsing is_virtual flag: %v", err)
		isVirtual = false
	}

	response := map[string]interface{}{
		"repo_id":       repo["id"],
		"repo_name":     repo["name"],
		"owner_email":   repoOwner,
		"owner_name":    goseaserv.Email2Nickname(goseaserv.Email2ContactEmail(repoOwner)),
		"owner_contact": goseaserv.Email2ContactEmail(repoOwner),
		"size:":         repo["size"],
		"encrypted":     encrypted,
		"file_count":    repo["file_count"],
		"permission":    permission,
		"no_quota":      quota < 0,
		"is_admin":      bflName == goseaserv.Email2Nickname(goseaserv.Email2ContactEmail(repoOwner)),
		"is_virtual":    isVirtual,
		"shared_out":    hasSharedOut,
		"need_decrypt":  libNeedDecrypt,
		"last_modified": TimestampToISO(repo["last_modify"]),
		"status":        NormalizeRepoStatusCode(repo["status"]),
	}

	return common.RenderJSON(w, r, response)
}
