package seahub

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"fmt"
	"k8s.io/klog/v2"
	"math"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

var (
	defaultRepoID string
	once          sync.Once
)

func getSystemDefaultRepoId() string {
	once.Do(func() {
		repoID, err := seaserv.GlobalSeafileAPI.GetSystemDefaultRepoId()
		if err != nil {
			klog.Infof("Error getting default repo ID: %v", err)
			return
		}
		defaultRepoID = repoID
	})
	return defaultRepoID
}

func HandleReposGet(header *http.Header, types []string) ([]byte, error) {
	MigrateSeahubUserToRedis(*header)
	filterBy := map[string]bool{
		"mine":   false,
		"shared": false,
	}

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

	bflName := header.Get("X-Bfl-User")
	username := bflName + "@auth.local"
	oldUsername := seaserv.GetOldUsername(bflName + "@seafile.com") // temp compatible

	usernameCache := make(map[string]string)
	nicknameCache := make(map[string]string)

	repoInfoList := make([]map[string]interface{}, 0)

	if filterBy["mine"] {
		ownedRepos, err := seaserv.GlobalSeafileAPI.GetOwnedRepoList(username, false, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processRepos(ownedRepos, username, &repoInfoList, usernameCache, nicknameCache)
		}

		oldOwnedRepos, err := seaserv.GlobalSeafileAPI.GetOwnedRepoList(oldUsername, false, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processRepos(oldOwnedRepos, oldUsername, &repoInfoList, usernameCache, nicknameCache)
		}
	}

	if filterBy["shared"] {
		sharedRepos, err := seaserv.GlobalSeafileAPI.GetShareInRepoList(username, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processSharedRepos(sharedRepos, username, &repoInfoList, usernameCache, nicknameCache)
		}

		oldSharedRepos, err := seaserv.GlobalSeafileAPI.GetShareInRepoList(oldUsername, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processSharedRepos(oldSharedRepos, oldUsername, &repoInfoList, usernameCache, nicknameCache)
		}
	}

	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	eventMsg := fmt.Sprintf("user-login\t%s\t%s\t%d", username, timestamp, -1)
	if resultCode, err := seaserv.GlobalSeafileAPI.PublishEvent("seahub.stats", eventMsg); err != nil || resultCode != 0 {
		klog.Errorf("Publish event failed, code: %d, err: %v", resultCode, err)
	}

	wrappedData := map[string]interface{}{
		"repos": repoInfoList,
	}

	jsonBytes, err := json.Marshal(wrappedData)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

func ReposGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	MigrateSeahubUserToRedis(r.Header)
	filterBy := map[string]bool{
		"mine":   false,
		"shared": false,
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
	oldUsername := seaserv.GetOldUsername(bflName + "@seafile.com") // temp compatible

	usernameCache := make(map[string]string)
	nicknameCache := make(map[string]string)

	repoInfoList := make([]map[string]interface{}, 0)

	if filterBy["mine"] {
		ownedRepos, err := seaserv.GlobalSeafileAPI.GetOwnedRepoList(username, false, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processRepos(ownedRepos, username, &repoInfoList, usernameCache, nicknameCache)
		}

		oldOwnedRepos, err := seaserv.GlobalSeafileAPI.GetOwnedRepoList(oldUsername, false, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processRepos(oldOwnedRepos, oldUsername, &repoInfoList, usernameCache, nicknameCache)
		}
	}

	if filterBy["shared"] {
		sharedRepos, err := seaserv.GlobalSeafileAPI.GetShareInRepoList(username, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processSharedRepos(sharedRepos, username, &repoInfoList, usernameCache, nicknameCache)
		}

		oldSharedRepos, err := seaserv.GlobalSeafileAPI.GetShareInRepoList(oldUsername, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processSharedRepos(oldSharedRepos, oldUsername, &repoInfoList, usernameCache, nicknameCache)
		}
	}

	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	eventMsg := fmt.Sprintf("user-login\t%s\t%s\t%d", username, timestamp, -1)
	if resultCode, err := seaserv.GlobalSeafileAPI.PublishEvent("seahub.stats", eventMsg); err != nil || resultCode != 0 {
		klog.Errorf("Publish event failed, code: %d, err: %v", resultCode, err)
	}

	return common.RenderJSON(w, r, repoInfoList)
}

func processRepos(repos []map[string]string, username string,
	repoInfoList *[]map[string]interface{}, usernameCache, nicknameCache map[string]string) {

	modifiers := make(map[string]bool)
	for _, repo := range repos {
		modifiers[repo["last_modifier"]] = true
	}
	for e := range modifiers {
		if _, exists := usernameCache[e]; !exists {
			usernameCache[e] = seaserv.Email2ContactEmail(e)
		}
		if _, exists := nicknameCache[e]; !exists {
			nicknameCache[e] = seaserv.Email2Nickname(seaserv.Email2ContactEmail(e))
		}
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i]["last_modify"] > repos[j]["last_modify"]
	})

	for _, repo := range repos {
		klog.Infof("~~~Debug log: repo = %v", repo)
		isVirtual, err := strconv.ParseBool(repo["is_virtual"])
		if err != nil {
			klog.Errorf("Error parsing is_virtual flag: %v", err)
			isVirtual = false
		}
		if isVirtual {
			continue
		}

		repoInfo := map[string]interface{}{
			"type":                   "mine",
			"repo_id":                repo["repo_id"],
			"repo_name":              repo["repo_name"],
			"owner_email":            username,
			"owner_name":             seaserv.Email2Nickname(seaserv.Email2ContactEmail(username)),
			"owner_contact_email":    seaserv.Email2ContactEmail(username),
			"last_modified":          TimestampToISO(repo["last_modify"]),
			"modifier_email":         repo["last_modifier"],
			"modifier_name":          nicknameCache["last_modifier"],
			"modifier_contact_email": usernameCache["last_modifier"],
			"size":                   repo["size"],
			"encrypted":              repo["encrypted"],
			"permission":             "rw",
			"status":                 NormalizeRepoStatusCode(repo["status"]),
			"salt":                   getSalt(repo),
			"is_virtual":             isVirtual,
		}

		*repoInfoList = append(*repoInfoList, repoInfo)
	}
}

func processSharedRepos(sharedRepos []map[string]string, username string,
	repoInfoList *[]map[string]interface{}, usernameCache, nicknameCache map[string]string) {

	owners := make(map[string]bool)
	modifiers := make(map[string]bool)
	for _, repo := range sharedRepos {
		owners[repo["user"]] = true
		modifiers[repo["last_modifier"]] = true
	}

	for user := range owners {
		if _, exists := usernameCache[user]; !exists {
			usernameCache[user] = seaserv.Email2ContactEmail(user)
		}
		if _, exists := nicknameCache[user]; !exists {
			nicknameCache[user] = seaserv.Email2Nickname(usernameCache[user])
		}
	}
	for user := range modifiers {
		if _, exists := usernameCache[user]; !exists {
			usernameCache[user] = seaserv.Email2ContactEmail(user)
		}
		if _, exists := nicknameCache[user]; !exists {
			nicknameCache[user] = seaserv.Email2Nickname(usernameCache[user])
		}
	}

	sort.Slice(sharedRepos, func(i, j int) bool {
		return sharedRepos[i]["last_modify"] > sharedRepos[j]["last_modify"]
	})

	for _, repo := range sharedRepos {
		ownerUsername := repo["user"]
		ownerName := nicknameCache[ownerUsername]
		ownerContact := usernameCache[ownerUsername]

		repoInfo := map[string]interface{}{
			"type":                   "shared",
			"repo_id":                repo["repo_id"],
			"repo_name":              repo["repo_name"],
			"last_modified":          TimestampToISO(repo["last_modify"]),
			"modifier_email":         repo["last_modifier"],
			"modifier_name":          nicknameCache[repo["last_modifier"]],
			"modifier_contact_email": usernameCache[repo["last_modifier"]],
			"owner_email":            ownerUsername,
			"owner_name":             ownerName,
			"owner_contact_email":    ownerContact,
			"size":                   repo["size"],
			"encrypted":              repo["encrypted"],
			"permission":             repo["permission"],
			"status":                 NormalizeRepoStatusCode(repo["status"]),
			"salt":                   getSalt(repo),
		}

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

func TimestampToISO(timestamp interface{}) string {
	var tsSec, tsNsec int64

	switch v := timestamp.(type) {
	case int:
		tsSec = int64(v)
	case int64:
		tsSec = v
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			klog.Errorf("Invalid float timestamp: %v", v)
			return fmt.Sprintf("%v", timestamp)
		}
		tsSec = int64(v)
		tsNsec = int64((v - float64(tsSec)) * 1e9)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			tsSec = i
		} else {
			klog.Errorf("String timestamp parse failed: %v", err)
			return v
		}
	default:
		klog.Errorf("Unsupported timestamp type: %T", timestamp)
		return fmt.Sprintf("%v", timestamp)
	}

	t := time.Unix(tsSec, tsNsec).UTC()

	return t.Format("2006-01-02T15:04:05.000Z07:00")
}

var REPO_STATUS_NORMAL = "normal"
var REPO_STATUS_READ_ONLY = "read-only"

func NormalizeRepoStatusCode(status string) string {
	if status == "0" {
		return REPO_STATUS_NORMAL
	} else if status == "1" {
		return REPO_STATUS_READ_ONLY
	}
	return ""
}
