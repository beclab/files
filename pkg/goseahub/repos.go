package goseahub

//
//import (
//	"encoding/json"
//	"fmt"
//	"net/http"
//	"time"
//)
//
//func ReposHandler(w http.ResponseWriter, r *http.Request) {
//	// 初始化过滤参数
//	filterBy := map[string]bool{
//		"mine":   false,
//		"shared": false,
//		"group":  false,
//		"public": false,
//	}
//
//	// 解析查询参数
//	query := r.URL.Query()
//	types := query["type"]
//	if len(types) == 0 {
//		for k := range filterBy {
//			filterBy[k] = true
//		}
//	} else {
//		for _, t := range types {
//			if _, exists := filterBy[t]; exists {
//				filterBy[t] = true
//			}
//		}
//	}
//
//	// 获取当前用户
//	ctx := r.Context()
//	user := ctx.Value("user").(*models.User)
//	email := user.Email
//
//	// 初始化缓存字典
//	contactCache := make(map[string]string)
//	nicknameCache := make(map[string]string)
//
//	// 获取组织上下文
//	orgID := int64(-1)
//	if org, ok := ctx.Value("org").(*models.Organization); ok {
//		orgID = org.OrgID
//	}
//
//	// 获取星标仓库
//	starredRepos, err := models.GetStarredRepos(email)
//	if err != nil {
//		utils.LogError(err)
//	}
//	starredRepoIDs := make(map[string]bool)
//	for _, repo := range starredRepos {
//		starredRepoIDs[repo.RepoID] = true
//	}
//
//	repoInfoList := make([]map[string]interface{}, 0)
//
//	// 处理我的仓库
//	if filterBy["mine"] {
//		var ownedRepos []*models.Repo
//		if orgID != -1 {
//			ownedRepos, err = services.GetOrgOwnedRepos(orgID, email)
//		} else {
//			ownedRepos, err = services.GetOwnedRepos(email)
//		}
//		if err != nil {
//			utils.LogError(err)
//		} else {
//			processRepos(ownedRepos, "mine", email, starredRepoIDs, &repoInfoList, contactCache, nicknameCache)
//		}
//	}
//
//	// 处理共享仓库
//	if filterBy["shared"] {
//		var sharedRepos []*models.SharedRepo
//		if orgID != -1 {
//			sharedRepos, err = services.GetOrgSharedRepos(orgID, email)
//		} else {
//			sharedRepos, err = services.GetSharedRepos(email)
//		}
//		if err != nil {
//			utils.LogError(err)
//		} else {
//			processSharedRepos(sharedRepos, email, starredRepoIDs, &repoInfoList, contactCache, nicknameCache)
//		}
//	}
//
//	// 处理群组仓库
//	if filterBy["group"] {
//		var groupRepos []*models.GroupRepo
//		if orgID != -1 {
//			groupRepos, err = services.GetOrgGroupRepos(email, orgID)
//		} else {
//			groupRepos, err = services.GetGroupRepos(email)
//		}
//		if err != nil {
//			utils.LogError(err)
//		} else {
//			processGroupRepos(groupRepos, email, starredRepoIDs, &repoInfoList, contactCache, nicknameCache)
//		}
//	}
//
//	// 处理公共仓库
//	if filterBy["public"] && user.Permissions.CanViewOrg {
//		publicRepos, err := services.ListInnerPubRepos(r)
//		if err != nil {
//			utils.LogError(err)
//		} else {
//			processPublicRepos(publicRepos, email, starredRepoIDs, &repoInfoList, contactCache, nicknameCache)
//		}
//	}
//
//	// 发布登录事件
//	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
//	eventMsg := fmt.Sprintf("user-login\t%s\t%s\t%d", email, timestamp, orgID)
//	if err := services.PublishEvent("seahub.stats", eventMsg); err != nil {
//		utils.LogError(err)
//	}
//
//	// 返回JSON响应
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(map[string]interface{}{
//		"repos": repoInfoList,
//	})
//}
//
//// 辅助函数保持与原始代码相似的处理逻辑
//func processRepos(repos []*models.Repo, repoType, email string, starredRepoIDs map[string]bool,
//	repoInfoList *[]map[string]interface{}, contactCache, nicknameCache map[string]string) {
//
//	// 获取监控仓库
//	monitoredRepoIDs := make(map[string]bool)
//	monitoredRepos, err := models.GetMonitoredRepos(email, repos)
//	if err != nil {
//		utils.LogError(err)
//	} else {
//		for _, repo := range monitoredRepos {
//			monitoredRepoIDs[repo.RepoID] = true
//		}
//	}
//
//	// 缓存用户信息
//	modifiers := make(map[string]bool)
//	for _, repo := range repos {
//		modifiers[repo.LastModifier] = true
//	}
//	for e := range modifiers {
//		if _, exists := contactCache[e]; !exists {
//			contactCache[e] = utils.Email2ContactEmail(e)
//		}
//		if _, exists := nicknameCache[e]; !exists {
//			nicknameCache[e] = utils.Email2Nickname(utils.Email2ContactEmail(e))
//		}
//	}
//
//	// 构建仓库信息
//	for _, repo := range repos {
//		if repo.IsVirtual {
//			continue
//		}
//
//		repoInfo := map[string]interface{}{
//			"type":                   repoType,
//			"repo_id":                repo.ID,
//			"repo_name":              repo.Name,
//			"owner_email":            email,
//			"owner_name":             utils.Email2Nickname(utils.Email2ContactEmail(email)),
//			"owner_contact_email":    utils.Email2ContactEmail(email),
//			"last_modified":          utils.TimestampToISO(repo.LastModify),
//			"modifier_email":         repo.LastModifier,
//			"modifier_name":          nicknameCache[repo.LastModifier],
//			"modifier_contact_email": contactCache[repo.LastModifier],
//			"size":                   repo.Size,
//			"encrypted":              repo.Encrypted,
//			"permission":             "rw",
//			"starred":                starredRepoIDs[repo.ID],
//			"monitored":              monitoredRepoIDs[repo.ID],
//			"status":                 utils.NormalizeRepoStatus(repo.Status),
//			"salt":                   getSalt(repo),
//		}
//
//		if utils.IsProVersion() && utils.EnableStorageClasses() {
//			repoInfo["storage_name"] = repo.StorageName
//			repoInfo["storage_id"] = repo.StorageID
//		}
//
//		*repoInfoList = append(*repoInfoList, repoInfo)
//	}
//}
//
//// 其他辅助函数保持与Python代码相似的逻辑
//func getSalt(repo *models.Repo) string {
//	if repo.EncVersion >= 3 {
//		return repo.Salt
//	}
//	return ""
//}
//
//// ... [其他辅助函数实现] ...
