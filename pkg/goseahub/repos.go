package goseahub

import (
	"files/pkg/common"
	"files/pkg/goseahub/goseaserv"
	"k8s.io/klog/v2"
	"net/http"
	"strconv"
	"sync"
)

var (
	defaultRepoID string
	once          sync.Once
)

func getSystemDefaultRepoId() string {
	once.Do(func() {
		repoID, err := goseaserv.GlobalSeafileAPI.GetSystemDefaultRepoId()
		if err != nil {
			klog.Infof("Error getting default repo ID: %v", err)
			return
		}
		defaultRepoID = repoID
	})
	return defaultRepoID
}

func ReposGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	filterBy := map[string]bool{
		"mine":   false,
		"shared": false,
		"group":  false,
		"public": false,
	}

	query := r.URL.Query()
	types := query["type"]
	if len(types) == 0 {
		for k := range filterBy {
			filterBy[k] = true
		}
	} else {
		for _, t := range types {
			if _, exists := filterBy[t]; exists {
				filterBy[t] = true
			}
		}
	}

	bflName := r.Header.Get("X-Bfl-User")
	username := bflName + "@auth.local"
	oldUsername := goseaserv.Email2ContactEmail(bflName + "@seafile.com") // temp comptible

	usernameCache := make(map[string]string)
	nicknameCache := make(map[string]string)

	repoInfoList := make([]map[string]interface{}, 0)

	if filterBy["mine"] {
		ownedRepos, err := goseaserv.GlobalSeafileAPI.GetOwnedRepoList(username, false, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processRepos(ownedRepos, "mine", username, nil, &repoInfoList, usernameCache, nicknameCache)
		}
		
		oldOwnedRepos, err := goseaserv.GlobalSeafileAPI.GetOwnedRepoList(oldUsername, false, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processRepos(oldOwnedRepos, "mine", oldUsername, nil, &repoInfoList, usernameCache, nicknameCache)
		}
	}

	//if filterBy["shared"] {
	//	var sharedRepos []*models.SharedRepo
	//	if orgID != -1 {
	//		sharedRepos, err = services.GetOrgSharedRepos(orgID, email)
	//	} else {
	//		sharedRepos, err = services.GetSharedRepos(email)
	//	}
	//	if err != nil {
	//		utils.LogError(err)
	//	} else {
	//		processSharedRepos(sharedRepos, email, starredRepoIDs, &repoInfoList, contactCache, nicknameCache)
	//	}
	//}

	//if filterBy["group"] {
	//	var groupRepos []*models.GroupRepo
	//	if orgID != -1 {
	//		groupRepos, err = services.GetOrgGroupRepos(email, orgID)
	//	} else {
	//		groupRepos, err = services.GetGroupRepos(email)
	//	}
	//	if err != nil {
	//		utils.LogError(err)
	//	} else {
	//		processGroupRepos(groupRepos, email, starredRepoIDs, &repoInfoList, contactCache, nicknameCache)
	//	}
	//}

	//if filterBy["public"] && user.Permissions.CanViewOrg {
	//	publicRepos, err := services.ListInnerPubRepos(r)
	//	if err != nil {
	//		utils.LogError(err)
	//	} else {
	//		processPublicRepos(publicRepos, email, starredRepoIDs, &repoInfoList, contactCache, nicknameCache)
	//	}
	//}

	//timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	//eventMsg := fmt.Sprintf("user-login\t%s\t%s\t%d", email, timestamp, orgID)
	//if err := services.PublishEvent("seahub.stats", eventMsg); err != nil {
	//	utils.LogError(err)
	//}

	//w.Header().Set("Content-Type", "application/json")
	//json.NewEncoder(w).Encode(map[string]interface{}{
	//	"repos": repoInfoList,
	//})
	return common.RenderJSON(w, r, repoInfoList)
}

func processRepos(repos []map[string]string, repoType, email string, starredRepoIDs map[string]bool,
	repoInfoList *[]map[string]interface{}, contactCache, nicknameCache map[string]string) {

	//monitoredRepoIDs := make(map[string]bool)
	//monitoredRepos, err := models.GetMonitoredRepos(email, repos)
	//if err != nil {
	//	utils.LogError(err)
	//} else {
	//	for _, repo := range monitoredRepos {
	//		monitoredRepoIDs[repo.RepoID] = true
	//	}
	//}
	//
	//modifiers := make(map[string]bool)
	//for _, repo := range repos {
	//	modifiers[repo.LastModifier] = true
	//}
	//for e := range modifiers {
	//	if _, exists := contactCache[e]; !exists {
	//		contactCache[e] = utils.Email2ContactEmail(e)
	//	}
	//	if _, exists := nicknameCache[e]; !exists {
	//		nicknameCache[e] = utils.Email2Nickname(utils.Email2ContactEmail(e))
	//	}
	//}

	for _, repo := range repos {
		klog.Infof("~~~Debug log: repo = %v", repo)
		//if repo.IsVirtual {
		//	continue
		//}

		repoInfo := map[string]interface{}{
			"type":                repoType,
			"repo_id":             repo["repo_id"],
			"repo_name":           repo["repo_name"],
			"owner_email":         email,
			"owner_name":          goseaserv.Email2Nickname(goseaserv.Email2ContactEmail(email)),
			"owner_contact_email": goseaserv.Email2ContactEmail(email),
			//"last_modified":          utils.TimestampToISO(repo.LastModify),
			//"modifier_email":         repo.LastModifier,
			//"modifier_name":          nicknameCache[repo.LastModifier],
			//"modifier_contact_email": contactCache[repo.LastModifier],
			"size":       repo["size"],
			"encrypted":  repo["encrypted"],
			"permission": "rw",
			//"starred":                starredRepoIDs[repo["ID"]],
			//"monitored":              monitoredRepoIDs[repo.ID],
			//"status":                 utils.NormalizeRepoStatus(repo.Status),
			"status": repo["status"],
			"salt":   getSalt(repo),
		}

		//if utils.IsProVersion() && utils.EnableStorageClasses() {
		//	repoInfo["storage_name"] = repo.StorageName
		//	repoInfo["storage_id"] = repo.StorageID
		//}

		*repoInfoList = append(*repoInfoList, repoInfo)
	}
}

func getSalt(repo map[string]string) string {
	envVersion, err := strconv.Atoi(repo["enc_version"])
	if err != nil {
		klog.Errorln(err)
		return ""
	}
	if envVersion >= 3 {
		return repo["salt"]
	}
	return ""
}
