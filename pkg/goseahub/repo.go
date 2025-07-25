package goseahub

import (
	"files/pkg/common"
	"files/pkg/goseahub/goseaserv"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
	"net/http"
)

type RepoInfoResponse struct {
	RepoID       string `json:"repo_id"`
	RepoName     string `json:"repo_name"`
	OwnerEmail   string `json:"owner_email"`
	OwnerName    string `json:"owner_name"`
	OwnerContact string `json:"owner_contact_email"`
	Size         int64  `json:"size"`
	Encrypted    bool   `json:"encrypted"`
	FileCount    int    `json:"file_count"`
	Permission   string `json:"permission"`
	NoQuota      bool   `json:"no_quota"`
	IsAdmin      bool   `json:"is_admin"`
	IsVirtual    bool   `json:"is_virtual"`
	SharedOut    bool   `json:"has_been_shared_out"`
	NeedDecrypt  bool   `json:"lib_need_decrypt"`
	LastModified string `json:"last_modified"`
	Status       string `json:"status"`
}

func RepoGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	vars := mux.Vars(r)
	repoId := vars["repo_id"]

	bflName := r.Header.Get("X-Bfl-User")
	username := bflName + "@auth.local"
	oldUsername := goseaserv.GetOldUsername(bflName + "@seafile.com") // temp compatible

	klog.Infof("~~~Debug log: %s, %s, %s", repoId, oldUsername, username)

	return 0, nil

	//repo, err := goseaserv.GlobalSeafileAPI.GetRepo(repoID)
	//if err != nil {
	//	http.Error(w, "Library not found", http.StatusNotFound)
	//	return
	//}
	//
	//// 4. 权限检查
	//permission, err := seafileapi.CheckFolderPermission(r.Context(), repoID, "/")
	//if err != nil || permission == "" {
	//	http.Error(w, "Permission denied", http.StatusForbidden)
	//	return
	//}
	//
	//// 5. 处理加密状态
	//libNeedDecrypt := false
	//if repo.Encrypted && !seafileapi.IsPasswordSet(repoID, username) {
	//	libNeedDecrypt = true
	//}
	//
	//// 6. 获取所有者信息
	//repoOwner, err := getRepoOwner(r.Context(), repoID)
	//if err != nil {
	//	http.Error(w, "Failed to get repo owner", http.StatusInternalServerError)
	//	return
	//}
	//
	//// 7. 检查共享状态
	//hasSharedOut, err := repoHasBeenSharedOut(r.Context(), repoID)
	//if err != nil {
	//	utils.LogError(err)
	//	hasSharedOut = false
	//}
	//
	//// 8. 组装响应数据
	//response := RepoInfoResponse{
	//	RepoID:       repo.ID,
	//	RepoName:     repo.Name,
	//	OwnerEmail:   repoOwner,
	//	OwnerName:    utils.EmailToNickname(utils.EmailToContactEmail(repoOwner)),
	//	OwnerContact: utils.EmailToContactEmail(repoOwner),
	//	Size:         repo.Size,
	//	Encrypted:    repo.Encrypted,
	//	FileCount:    repo.FileCount,
	//	Permission:   permission,
	//	NoQuota:      seafileapi.CheckQuota(repoID) < 0,
	//	IsAdmin:      isRepoAdmin(username, repoID),
	//	IsVirtual:    repo.IsVirtual,
	//	SharedOut:    hasSharedOut,
	//	NeedDecrypt:  libNeedDecrypt,
	//	LastModified: utils.TimestampToISO(repo.LastModify),
	//	Status:       utils.NormalizeRepoStatusCode(repo.Status),
	//}
	//
	//// 9. 返回JSON响应
	//w.Header().Set("Content-Type", "application/json")
	//json.NewEncoder(w).Encode(response)
}

//func getRepoOwner(ctx context.Context, repoID string) (string, error) {
//	// 实现获取仓库所有者的逻辑
//	return "owner@example.com", nil
//}
//
//func repoHasBeenSharedOut(ctx context.Context, repoID string) (bool, error) {
//	// 实现检查仓库是否被共享的逻辑
//	return true, nil
//}
//
//func isRepoAdmin(username, repoID string) bool {
//	// 实现管理员检查逻辑
//	return false
//}
