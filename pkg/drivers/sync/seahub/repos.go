package seahub

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/searpc"
	"files/pkg/drivers/sync/seahub/seaserv"
	"fmt"
	"html/template"
	"k8s.io/klog/v2"
	"math"
	"sort"
	"strconv"
	"strings"
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

func HandleReposGet(owner string, types []string) ([]byte, error) {
	filterBy := map[string]bool{
		"mine":         false,
		"shared":       false,
		"shared_by_me": false,
		"shared_to_me": false,
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

	username := owner + "@auth.local"

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
	}

	if filterBy["shared_to_me"] {
		sharedInRepos, err := seaserv.GlobalSeafileAPI.GetShareInRepoList(username, -1, -1)
		if err != nil {
			klog.Errorln(err)
		} else {
			processSharedInRepos(sharedInRepos, username, &repoInfoList, usernameCache, nicknameCache)
		}
	}

	if filterBy["shared"] || filterBy["shared_by_me"] {
		sharedOutRepos, err := GetSharedOutRepos(username)
		if err != nil {
			klog.Errorln(err)
		}
		sharedOutFolders, err := GetSharedOutFolders(username)
		if err != nil {
			klog.Errorln(err)
		} else {
			sharedOutRepos = append(sharedOutRepos, sharedOutFolders...)
		}
		if err = json.Unmarshal(common.ToBytes(sharedOutRepos), &repoInfoList); err != nil {
			klog.Errorf("Failed to unmarshal response body: %v", err)
			return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
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

	return common.ToBytes(wrappedData), nil
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

func processSharedInRepos(sharedRepos []map[string]string, username string,
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
			"repo_id":                strings.Trim(repo["repo_id"], " "),
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
			"permission":             strings.Trim(repo["permission"], " "),
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

func HandleRepoDelete(owner, repoId string) ([]byte, error) {
	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Infof("API error when getting repo: %v", err)
		return nil, err
	}

	if repo == nil {
		if resultCode, err := seaserv.GlobalSeafileAPI.RemoveRepo(repoId); err != nil || resultCode != 0 {
			klog.Errorf("Failed to remove repo: result_code: %d, err: %v", resultCode, err)
			return nil, errors.New("failed to remove repo")
		}
		return common.ToBytes(map[string]interface{}{"success": true}), nil
	}

	username := owner + "@auth.local"

	repoOwner, err := seaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
	if err != nil {
		klog.Infof("Error getting repo owner: %v", err)
		return nil, err
	}

	if username != repoOwner { // temp compatible
		return nil, errors.New("Permission denied")
	}

	repoStatusInt, err := strconv.ParseInt(repo["status"], 10, 32)
	if err != nil {
		klog.Errorf("Error parsing repo status: %v", err)
		return nil, err
	}
	if repoStatusInt != 0 {
		return nil, errors.New("Permission denied")
	}

	if _, err = seaserv.GlobalSeafileAPI.RemoveRepo(repoId); err != nil {
		klog.Errorf("Error removing repo: %v", err)
		return nil, err
	}
	return common.ToBytes(map[string]interface{}{"success": true}), nil
}

var units = []string{"B", "KB", "MB", "GB", "TB", "PB"}

func FileSizeFormat(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	absBytes := bytes
	negative := false
	if bytes < 0 {
		absBytes = -bytes
		negative = true
	}

	size := float64(absBytes)
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	var result string
	if intSize := int64(size); size == float64(intSize) {
		result = fmt.Sprintf("%d %s", intSize, units[unitIndex])
	} else {
		result = fmt.Sprintf("%.1f %s", size, units[unitIndex])
	}

	if negative {
		result = "-" + result
	}

	return result
}

const TIME_ZONE = "Asia/Shanghai"

func translateSeahubTime(value interface{}, autoescape bool) template.HTML {
	var val time.Time
	switch v := value.(type) {
	case int:
		val = time.Unix(int64(v), 0)
	case int64:
		val = time.Unix(v, 0)
	case time.Time:
		val = v
	default:
		return template.HTML(fmt.Sprint(value))
	}

	loc, _ := time.LoadLocation(TIME_ZONE)
	if val.Location() != loc {
		val = val.In(loc)
	}

	isoTime := val.Format(time.RFC3339)
	titleTime := val.Format(time.RFC1123Z)

	translatedTime := humanReadableTime(val)

	if autoescape {
		translatedTime = template.HTMLEscapeString(translatedTime)
	}

	return template.HTML(fmt.Sprintf(
		`<time datetime="%s" is="relative-time" title="%s">%s</time>`,
		isoTime, titleTime, translatedTime,
	))
}

func humanReadableTime(t time.Time) string {
	now := time.Now().In(t.Location())
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	default:
		return t.Format("2006-01-02 15:04")
	}
}

func GetRepoLastModify(repo map[string]string) string {
	if repo["head_cmmt_id"] != "" {
		version, err := strconv.Atoi(repo["version"])
		if err != nil {
			klog.Errorf("Error parsing repo version: %v", err)
			return "0"
		}
		if commit, err := seaserv.GlobalSeafileAPI.GetCommit(repo["id"], version, repo["head_cmmt_id"]); err == nil && commit != nil {
			return commit["ctime"]
		}
	}

	klog.Infof("head_cmmt_id is missing for repo %s", repo["id"])
	if commits, err := seaserv.GlobalSeafileAPI.GetCommitList(repo["id"], 0, 1); err == nil && len(commits) > 0 {
		return commits[0]["ctime"]
	}

	return "0"
}

func CalculateReposLastModify(repos []map[string]string) {
	for _, repo := range repos {
		repo["latest_modify"] = GetRepoLastModify(repo)
	}
}

func repoDownloadInfo(repoId, username string, genSyncToken bool) (map[string]interface{}, error) {
	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Errorf("Error getting repo: %v", err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("Error getting repo: %v", repoId)
		return nil, errors.New("Repo not found")
	}

	var token string
	if genSyncToken {
		token, err = seaserv.GlobalSeafileAPI.GenerateRepoToken(repoId, username)
		if err != nil {
			klog.Errorf("Error generating token: %v", err)
			return nil, err
		}
	}

	repoSizeInt, err := strconv.ParseInt(repo["size"], 10, 32)
	if err != nil {
		klog.Errorf("Error parsing repo size: %v", err)
		repoSizeInt = 0
	}

	encrypted, err := strconv.ParseBool(repo["encrypted"])
	if err != nil {
		klog.Errorf("Error parsing repo encrypted: %v", err)
		encrypted = false
	}

	encVersion, err := strconv.Atoi(repo["enc_version"])
	if err != nil {
		klog.Errorf("Error parsing repo env_version: %v", err)
		encVersion = -1
	}

	permission, err := seaserv.GlobalSeafileAPI.CheckPermissionByPath(repoId, "/", username)
	if err != nil {
		klog.Errorf("Error checking permission: %v", err)
		return nil, err
	}

	CalculateReposLastModify([]map[string]string{repo})

	info := map[string]interface{}{
		"relay_id":            "44e8f253849ad910dc142247227c8ece8ec0f971",
		"relay_addr":          "127.0.0.1",
		"relay_port":          "80",
		"email":               username,
		"token":               token,
		"repo_id":             repoId,
		"repo_name":           repo["name"],
		"repo_desc":           repo["desc"],
		"repo_size":           repoSizeInt,
		"repo_size_formatted": FileSizeFormat(repoSizeInt),
		"mtime":               TimestampToISO(repo["latest_modify"]),
		"mtime_relative":      translateSeahubTime(repo["latest_modify"], false),
		"encrypted":           map[bool]int{true: 1, false: 0}[encrypted],
		"enc_version":         encVersion,
		"salt":                "",
		"magic":               "",
		"random_key":          "",
		"repo_version":        repo["version"],
		"head_commit_id":      repo["head_cmmt_id"],
		"permission":          permission,
	}

	if encrypted {
		info["magic"] = repo["magic"]
		info["random_key"] = repo["random_key"]
		if encVersion >= 3 {
			info["salt"] = repo["salt"]
		}
	}

	return info, nil
}

func HandleRepoPost(owner, repoName, passwd string) ([]byte, error) {
	if repoName == "" {
		return nil, errors.New("repository name is required")
	}
	if !isValidDirentName(repoName) {
		return nil, errors.New("invalid repository name")
	}

	username := owner + "@auth.local"

	var repoId string
	var err error
	repoId, err = createRepo(repoName, "", username, nil)

	if err != nil {
		klog.Errorf("Create repo error: %v", err)
		return nil, err
	}

	resp, err := repoDownloadInfo(repoId, username, true)

	return common.ToBytes(resp), nil
}

func createRepo(name, desc, username string, passwd *string) (string, error) {
	if passwd != nil && *passwd != "" {
		return "", &searpc.SearpcError{Msg: "NOT allow to create encrypted library."}
	}

	return seaserv.GlobalSeafileAPI.CreateRepo(name, desc, username, passwd, 2)
}

func isValidDirentName(name string) bool {
	// `repo_id` parameter is not used in seafile api
	ret, err := seaserv.GlobalSeafileAPI.IsValidFilename("fake_repo_id", name)
	if err != nil {
		klog.Errorf("Error validating dirent name: %v", err)
		return false
	}
	if ret == 0 {
		return false
	}
	return true
}

func HandleRepoPatch(owner, repoId, repoName, repoDesc, op string) ([]byte, error) {
	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Errorf("Error getting repo: %v", err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("Error getting repo: %v", repoId)
		return nil, errors.New("Repo not found")
	}

	// we only use op == "rename" now
	if op == "" {
		op = "rename"
	}

	switch op {
	case "checkpassword":
		return nil, errors.New("not supported now")

	case "setpassword":
		return nil, errors.New("not supported now")

	case "rename":
		if !isValidDirentName(repoName) {
			return nil, errors.New("invalid repo name")
		}

		username := owner + "@auth.local"

		repoOwner, err := seaserv.GlobalSeafileAPI.GetRepoOwner(repo["id"])
		if err != nil {
			klog.Errorf("Error getting repo owner: %v", err)
			return nil, err
		}

		if username != repoOwner {
			return nil, errors.New("You do not have permission to rename this library.")
		}

		repoStatusInt, err := strconv.Atoi(repo["status"])
		if err != nil {
			klog.Errorf("Error parsing repo status: %v", err)
			return nil, err
		}
		if repoStatusInt != 0 {
			return nil, errors.New("Permission denied.")
		}

		if resultCode, err := seaserv.GlobalSeafileAPI.EditRepo(repoId, repoName, repoDesc, username); err != nil || resultCode != 0 {
			klog.Errorf("Failed to edit repo: result_code: %d, err: %v", resultCode, err)
			return nil, errors.New("Failed to edit repo")
		}
		return []byte("success"), nil

	default:
		return nil, errors.New("unsupported operation")
	}
}
