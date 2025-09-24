package seahub

import (
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/hertz/biz/model/api/share"
	"fmt"
	"github.com/bytedance/gopkg/util/logger"
	"k8s.io/klog/v2"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	INT_PERMISSION_NONE = iota
	INT_PERMISSION_VIEW
	INT_PERMISSION_UPLOAD
	INT_PERMISSION_EDIT
	INT_PERMISSION_ADMIN
)

var SharePermissionMap = map[int]string{
	INT_PERMISSION_NONE:   "",
	INT_PERMISSION_VIEW:   "r",
	INT_PERMISSION_UPLOAD: "w", // not used for sync
	INT_PERMISSION_EDIT:   "rw",
	INT_PERMISSION_ADMIN:  "admin",
}

func listUserSharedItems(sharePathId, repoId, path string) ([]map[string]interface{}, error) {
	var shareItems []map[string]string
	var err error

	repoOwner, err := seaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
	if err != nil {
		return nil, err
	}

	if path == "/" {
		shareItems, err = seaserv.GlobalSeafileAPI.ListRepoSharedTo(repoOwner, repoId)
	} else {
		shareItems, err = seaserv.GlobalSeafileAPI.GetSharedUsersForSubdir(repoId, path, repoOwner)
	}

	if err != nil {
		return nil, err
	}

	queryParams := &database.QueryParams{}
	queryParams.AND = []database.Filter{}
	database.BuildStringQueryParam(sharePathId, "share_members.path_id", "=", &queryParams.AND, true)
	database.BuildIntQueryParam(INT_PERMISSION_ADMIN, "share_members.permission", "=", &queryParams.AND, true)

	adminUsers, _, err := database.QueryShareMember(queryParams, 0, 0, "share_members.id", "ASC", nil)
	if err != nil {
		klog.Errorf("QueryShareMember error: %v", err)
		return nil, err
	}

	adminSet := make(map[string]bool)
	for _, user := range adminUsers {
		adminSet[user.ShareMember] = true
	}

	var ret []map[string]interface{}
	for _, item := range shareItems {
		ret = append(ret, map[string]interface{}{
			"share_type": "user",
			"user_info": map[string]string{
				"name":          item["user"],
				"nickname":      strings.TrimSuffix(item["user"], "@auth.local"),
				"contact_email": item["user"],
			},
			"permission": item["perm"],
			"is_admin":   adminSet[item["user"]],
		})
	}

	return ret, nil
}

func handleSharedToArgs(shareType string) (bool, bool) {
	sharedToUser := false
	sharedToGroup := false

	if shareType != "" {
		for _, e := range strings.Split(shareType, ",") {
			e = strings.TrimSpace(e)
			if e == "user" {
				sharedToUser = true
			} else if e == "group" {
				sharedToGroup = true
			}
		}
	} else {
		sharedToUser = true
		sharedToGroup = true
	}

	return sharedToUser, sharedToGroup
}

func HandleGetDirSharedItems(sharePath *share.SharePath, shareType string) ([]byte, error) {
	repoId := sharePath.Extend
	path := sharePath.Path

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("library %s not found", repoId)
	}

	sharedToUser, _ := handleSharedToArgs(shareType)

	if path == "" {
		path = "/"
	}
	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, path)
	if dirId == "" || err != nil {
		return nil, fmt.Errorf("folder %s not found", path)
	}

	var ret []map[string]interface{}
	if sharedToUser {
		ret, err = listUserSharedItems(sharePath.ID, repoId, path)
		if err != nil {
			return nil, err
		}
	}
	return common.ToBytes(ret), nil
}

func isRepoAdmin(sharePathId, username, repoId string) bool {
	// from share vision
	queryParams := &database.QueryParams{}
	queryParams.AND = []database.Filter{}
	database.BuildStringQueryParam(sharePathId, "share_members.path_id", "=", &queryParams.AND, true)
	database.BuildStringQueryParam(username, "share_members.username", "=", &queryParams.AND, true)
	database.BuildIntQueryParam(INT_PERMISSION_ADMIN, "share_members.permission", "=", &queryParams.AND, true)

	_, total, err := database.QueryShareMember(queryParams, 0, 0, "share_members.id", "ASC", nil)
	if err != nil {
		klog.Errorf("QueryShareMember error: %v", err)
		return false
	}
	if total > 0 {
		return true
	}

	// Get repo owner (either regular or org)
	var repoOwner string
	repoOwner, err = seaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
	if err != nil {
		logger.Errorf("repo %s owner is None", repoId)
		return false
	}

	// Check if user is repo owner
	if username == repoOwner {
		return true
	}

	return false
}

var emailRegex = regexp.MustCompile(`(?i)` +
	// local：dot-atom format
	`(^[-!#$%&*+/=?^_` + "`" + `{}|~0-9A-Z]+(\.[-!#$%&*+/=?^_` + "`" + `{}|~0-9A-Z]+)*` +
	// local：quoted-string format
	`|^"([\x01-\x08\x0B\x0C\x0E-\x1F!#-\[\]-\x7F]|\\[\x01-\x0B\x0C\x0E-\x7F])*"` +
	// domain name
	`)@((?:[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?\.)+(?:[A-Z]{2,6}\.?|[A-Z0-9-]{2,}\.?)$)` +
	// IPv4 address format
	`|\[(25[0-5]|2[0-4]\d|[0-1]?\d?\d)(\.(25[0-5]|2[0-4]\d|[0-1]?\d?\d)){3}\]`)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func isValidUsername(username string) bool {
	return isValidEmail(username)
}

func UpdateUserDirPermission(repoID, path, owner, shareTo, permission string) {
	if permission == PERMISSION_ADMIN {
		permission = PERMISSION_READ_WRITE
	}

	if path == "/" {
		seaserv.GlobalSeafileAPI.SetSharePermission(repoID, owner, shareTo, permission)
	} else {
		seaserv.GlobalSeafileAPI.UpdateShareSubdirPermForUser(repoID, path, owner, shareTo, permission)
	}
}

// HandlePostDirSharedItems update permission
func HandlePostDirSharedItems(sharePath *share.SharePath, shareMember *share.ShareMember, shareType string) ([]byte, error) {
	repoId := sharePath.Extend
	username := sharePath.Owner + "@auth.local"

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("library %s not found", repoId)
	}

	path := sharePath.Path
	if path == "" {
		path = "/"
	}
	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, path)
	if dirId == "" || err != nil {
		return nil, fmt.Errorf("folder %s not found", path)
	}

	permission := SharePermissionMap[int(shareMember.Permission)]

	repoOwner, err := seaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
	if err != nil {
		return nil, err
	}
	if repoOwner != username && !isRepoAdmin(sharePath.ID, username, repoId) {
		return nil, fmt.Errorf("permission denied")
	}

	sharedToUser, _ := handleSharedToArgs(shareType)
	if sharedToUser {
		allUsers, err := seaserv.ListAllUsers()
		if err != nil {
			klog.Errorf("Error listing users: %v", err)
			return nil, err
		}

		sharedTo := shareMember.ShareMember + "@auth.local"
		if sharedTo == "" || !isValidUsername(sharedTo) {
			return nil, fmt.Errorf("email %s invalid", sharedTo)
		}

		var existedUser map[string]interface{}
		var ok bool
		var existedUsername string
		if existedUser, ok = allUsers[sharedTo]; !ok {
			return nil, fmt.Errorf("user not found")
		}
		if existedUsername, ok = existedUser["username"].(string); !ok || existedUsername == "" {
			return nil, fmt.Errorf("user not found")
		}

		UpdateUserDirPermission(repoId, path, repoOwner, sharedTo, permission)
	}

	return common.ToBytes(map[string]interface{}{"success": true}), nil
}

func HasSharedToUser(sharePathId, repoID, path, username string) bool {
	items, err := listUserSharedItems(sharePathId, repoID, path)
	if err != nil {
		klog.Errorf("listUserSharedItems error: %v", err)
		return false
	}

	for _, item := range items {
		if username == item["user_info"].(map[string]string)["name"] { // username is with "@auth.local"
			return true
		}
	}
	return false
}

func ShareDirToUser(repo map[string]string, path, owner, shareFrom, shareTo, permission string) error {
	//var extraPermission string

	if permission == PERMISSION_ADMIN {
		//extraPermission = permission	// admin is controlled in share_members, no need to additional save here
		permission = PERMISSION_READ_WRITE
	}

	if path == "/" {
		_, err := seaserv.GlobalSeafileAPI.ShareRepo(repo["repo_id"], owner, shareTo, permission)
		if err != nil {
			return err
		}
	} else {
		_, err := seaserv.GlobalSeafileAPI.ShareSubdirToUser(repo["repo_id"], path, owner, shareTo, permission, "")
		if err != nil {
			return err
		}
	}

	return nil
}

func HandlePutDirSharedItems(sharePath *share.SharePath, shareMembers []*share.ShareMember, shareType string) ([]byte, error) {
	repoId := sharePath.Extend
	username := sharePath.Owner + "@auth.local"

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("library %s not found", repoId)
	}

	path := sharePath.Path
	if path == "" {
		path = "/"
	}
	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, path)
	if dirId == "" || err != nil {
		return nil, fmt.Errorf("folder %s not found", path)
	}

	encrypted, err := strconv.ParseBool(repo["encrypted"])
	if err != nil {
		klog.Errorf("Error parsing repo encrypted: %v", err)
		encrypted = false
	}
	if encrypted && path != "/" {
		return nil, fmt.Errorf("folder invalid")
	}

	if shareType != "user" && shareType != "group" {
		return nil, fmt.Errorf("share_type invalid")
	}

	repoOwner, err := seaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
	if err != nil {
		return nil, err
	}
	if repoOwner != username && !isRepoAdmin(sharePath.ID, username, repoId) {
		return nil, fmt.Errorf("permission denied")
	}

	result := struct {
		Failed  []map[string]interface{} `json:"failed"`
		Success []map[string]interface{} `json:"success"`
	}{
		Failed:  make([]map[string]interface{}, 0),
		Success: make([]map[string]interface{}, 0),
	}

	if shareType == "user" {
		allUsers, err := seaserv.ListAllUsers()
		if err != nil {
			klog.Errorf("Error listing users: %v", err)
			return nil, err
		}

		for _, shareMember := range shareMembers {
			permission := int(shareMember.Permission)
			if permission != INT_PERMISSION_VIEW && permission != INT_PERMISSION_EDIT && permission != INT_PERMISSION_ADMIN {
				klog.Errorf("Invalid permission: %d", permission)
				return nil, fmt.Errorf("permission denied")
			}

			toUser := shareMember.ShareMember + "@auth.local"

			if !isValidUsername(toUser) {
				klog.Errorf("Invalid username: %s", toUser)
				result.Failed = append(result.Failed, map[string]interface{}{
					"email":     toUser,
					"error_msg": "username invalid.",
				})
				continue
			}

			var existedUser map[string]interface{}
			var ok bool
			var existedUsername string
			if existedUser, ok = allUsers[toUser]; !ok {
				klog.Errorf("user not found")
				return nil, fmt.Errorf("user not found")
			}
			if existedUsername, ok = existedUser["username"].(string); !ok || existedUsername == "" {
				klog.Errorf("user %s not found", toUser)
				result.Failed = append(result.Failed, map[string]interface{}{
					"email":     toUser,
					"error_msg": fmt.Sprintf("User %s not found.", toUser),
				})
				continue
			}

			if HasSharedToUser(sharePath.ID, repoId, path, toUser) {
				klog.Errorf("Share already exists")
				result.Failed = append(result.Failed, map[string]interface{}{
					"email":     toUser,
					"error_msg": fmt.Sprintf("This item has been shared to %s.", toUser),
				})
				continue
			}

			func() {
				repoOwner, err = seaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
				if err != nil {
					klog.Error(err)
					result.Failed = append(result.Failed, map[string]interface{}{
						"email":     toUser,
						"error_msg": "Internal Server Error",
					})
					return
				}
				if toUser == repoOwner {
					klog.Errorf("Library can not be shared to owner")
					result.Failed = append(result.Failed, map[string]interface{}{
						"email":     toUser,
						"error_msg": "Library can not be shared to owner",
					})
					return
				}

				err = ShareDirToUser(repo, path, repoOwner, username, toUser, SharePermissionMap[permission])

				if err != nil {
					klog.Error(err)
					result.Failed = append(result.Failed, map[string]interface{}{
						"email":     toUser,
						"error_msg": "Internal Server Error",
					})
					return
				}

				permDisplay := PERMISSION_READ_WRITE
				if SharePermissionMap[permission] == PERMISSION_ADMIN {
					permDisplay = PERMISSION_ADMIN
				}

				result.Success = append(result.Success, map[string]interface{}{
					"share_type": "user",
					"user_info": map[string]interface{}{
						"name":          toUser,
						"nickname":      strings.TrimSuffix(toUser, "@auth.local"),
						"contact_email": toUser,
					},
					"permission": permDisplay,
					"is_admin":   SharePermissionMap[permission] == PERMISSION_ADMIN,
				})
			}()
		}
	}

	return common.ToBytes(result), nil
}

func CheckUserShareOutPermission(sharePathId, repoID, path, shareTo string) string {
	if path == "/" {
		path = ""
	}

	repo, err := seaserv.GlobalSeafileAPI.GetSharedRepoByPath(repoID, path, shareTo, false)
	if repo == nil || err != nil {
		klog.Errorf("CheckUserShareOutPermission: Error getting repo info: %v", err)
		return ""
	}

	permission := repo["permission"]

	if path == "" {
		//extraPermission :=
		queryParams := &database.QueryParams{}
		queryParams.AND = []database.Filter{}
		database.BuildStringQueryParam(sharePathId, "share_members.path_id", "=", &queryParams.AND, true)
		database.BuildIntQueryParam(INT_PERMISSION_ADMIN, "share_members.permission", "=", &queryParams.AND, true)
		database.BuildStringQueryParam(shareTo, "share_members.share_member", "=", &queryParams.AND, true)

		adminUsers, _, err := database.QueryShareMember(queryParams, 0, 0, "share_members.id", "ASC", nil)
		if err != nil {
			klog.Errorf("QueryShareMember error: %v", err)
			return ""
		}
		if len(adminUsers) > 0 {
			permission = PERMISSION_ADMIN
		}
	}

	return permission
}

func HandleDeleteDirSharedItems(sharePath *share.SharePath, shareMember *share.ShareMember, shareType string) ([]byte, error) {
	repoId := sharePath.Extend
	username := sharePath.Owner + "@auth.local"

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("library %s not found", repoId)
	}

	path := sharePath.Path
	if path == "" {
		path = "/"
	}
	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, path)
	if dirId == "" || err != nil {
		return nil, fmt.Errorf("folder %s not found", path)
	}

	repoOwner, err := seaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
	if err != nil {
		return nil, err
	}
	if repoOwner != username && !isRepoAdmin(sharePath.ID, username, repoId) {
		return nil, fmt.Errorf("permission denied")
	}

	sharedToUser, _ := handleSharedToArgs(shareType)
	if sharedToUser {
		sharedTo := shareMember.ShareMember + "@auth.local"
		if sharedTo == "" || !isValidUsername(sharedTo) {
			return nil, fmt.Errorf("email %s invalid", sharedTo)
		}

		permissionStr := CheckUserShareOutPermission(sharePath.ID, repoId, path, sharedTo) // no need to real use because share not managed by seahub any more
		klog.Infof("permission str: %v", permissionStr)

		if path == "/" {
			if _, err = seaserv.GlobalSeafileAPI.RemoveShare(repoId, repoOwner, sharedTo); err != nil {
				return nil, err
			}
		} else {
			if _, err = seaserv.GlobalSeafileAPI.UnshareSubdirForUser(repoId, path, repoOwner, sharedTo); err != nil {
				return nil, err
			}
		}
	}

	return common.ToBytes(map[string]interface{}{"success": true}), nil
}

func GetSharedOutRepos(username string) ([]map[string]string, error) {
	sharedOutRepos := make([]map[string]string, 0)

	repos, err := seaserv.GlobalSeafileAPI.GetShareOutRepoList(username, -1, -1)
	if err != nil {
		klog.Errorf("GetSharedOutRepos Error: %v", err)
		return nil, err
	}
	sharedOutRepos = append(sharedOutRepos, repos...)

	pubRepos, err := seaserv.GlobalSeafileAPI.ListInnerPubReposByOwner(username)
	if err != nil {
		klog.Errorf("GetInnerPubRepos Error: %v", err)
		return nil, err
	}
	sharedOutRepos = append(sharedOutRepos, pubRepos...)

	sort.Slice(sharedOutRepos, func(i, j int) bool {
		return sharedOutRepos[i]["repo_name"] < sharedOutRepos[j]["repo_name"]
	})

	memberQueryParams := &database.QueryParams{}
	memberQueryParams.AND = []database.Filter{}
	database.BuildIntQueryParam(INT_PERMISSION_ADMIN, "share_members.permission", "=", &memberQueryParams.AND, true)
	database.BuildStringQueryParam("sync", "share_paths.file_type", "=", &memberQueryParams.AND, true)
	memberJoinConditions := []*database.JoinCondition{}
	memberJoinConditions = append(memberJoinConditions, &database.JoinCondition{
		Table:     "share_members",
		Field:     "path_id",
		JoinTable: "share_paths",
		JoinField: "id",
	})
	adminMembers, _, err := database.QueryShareMember(memberQueryParams, 0, 0, "share_members.id", "ASC", memberJoinConditions)
	if err != nil {
		klog.Errorf("QueryShareMember error: %v", err)
		return nil, err
	}

	pathQueryParams := &database.QueryParams{}
	pathQueryParams.AND = []database.Filter{}
	database.BuildIntQueryParam(INT_PERMISSION_ADMIN, "share_members.permission", "=", &memberQueryParams.AND, true)
	database.BuildStringQueryParam("sync", "share_paths.file_type", "=", &memberQueryParams.AND, true)
	pathJoinConditions := []*database.JoinCondition{}
	pathJoinConditions = append(pathJoinConditions, &database.JoinCondition{
		Table:     "share_paths",
		Field:     "id",
		JoinTable: "share_members",
		JoinField: "path_id",
	})
	adminPaths, _, err := database.QuerySharePath(pathQueryParams, 0, 0, "share_members.id", "ASC", pathJoinConditions)
	if err != nil {
		klog.Errorf("QuerySharePath error: %v", err)
		return nil, err
	}

	type AdminRelation struct {
		RepoId   string
		Username string
	}
	adminMap := make(map[AdminRelation]struct{})
	for _, adminMember := range adminMembers {
		for _, adminPath := range adminPaths {
			if adminMember.PathID == adminPath.ID {
				key := AdminRelation{
					RepoId:   adminPath.Extend,
					Username: adminMember.ShareMember + "@auth.local",
				}
				adminMap[key] = struct{}{}
			}
		}
	}

	returnedResult := make([]map[string]string, 0)
	for _, repo := range sharedOutRepos {
		isVirtual, _ := strconv.ParseBool(repo["is_virtual"])
		if isVirtual {
			continue
		}

		result := make(map[string]string)
		result["repo_id"] = strings.Trim(repo["repo_id"], " ")
		result["repo_name"] = repo["repo_name"]
		result["encrypted"] = repo["encrypted"]
		result["share_type"] = repo["share_type"]
		result["share_permission"] = strings.Trim(repo["permission"], " ")
		result["modifier_email"] = repo["last_modifier"]
		result["modifier_name"] = seaserv.Email2Nickname(seaserv.Email2ContactEmail(repo["last_modifier"]))
		result["modifier_contact_email"] = seaserv.Email2ContactEmail(repo["last_modifier"])

		if repo["share_type"] == "personal" {
			userEmail := repo["user"]
			result["user_name"] = seaserv.Email2Nickname(seaserv.Email2ContactEmail(userEmail))
			result["user_email"] = userEmail
			result["contact_email"] = seaserv.Email2ContactEmail(userEmail)
		}

		returnedResult = append(returnedResult, result)
	}

	for _, result := range returnedResult {
		if shareType, ok := result["share_type"]; ok {
			if shareType == "personal" {
				if userEmail, ok := result["user_email"]; ok {
					queryKey := AdminRelation{RepoId: result["repo_id"], Username: userEmail}
					if _, exists := adminMap[queryKey]; exists {
						result["is_admin"] = "true"
					} else {
						result["is_admin"] = "false"
					}
				}
			}
		}
	}

	return returnedResult, nil
}

func GetSharedOutFolders(username string) ([]map[string]string, error) {
	sharedRepos := make([]map[string]string, 0)

	var err error

	repos, err := seaserv.GlobalSeafileAPI.GetShareOutRepoList(username, -1, -1)
	if err != nil {
		klog.Errorf("GetShareOutFolders error: %v", err)
		return nil, err
	}
	sharedRepos = append(sharedRepos, repos...)

	sort.Slice(sharedRepos, func(i, j int) bool {
		return sharedRepos[i]["repo_name"] < sharedRepos[j]["repo_name"]
	})

	returnedResult := make([]map[string]string, 0)

	for _, repo := range sharedRepos {
		isVirtual, _ := strconv.ParseBool(repo["is_virtual"])
		if !isVirtual {
			continue
		}

		result := map[string]string{
			"origin_repo_id":   strings.Trim(repo["origin_repo_id"], " "),
			"origin_repo_name": repo["origin_repo_name"],
			"repo_id":          strings.Trim(repo["repo_id"], " "),
			"repo_name":        repo["repo_name"],
			"path":             repo["origin_path"],
			"folder_name":      repo["name"],
			"share_type":       repo["share_type"],
			"share_permission": strings.Trim(repo["permission"], " "),
		}

		if repo["share_type"] == "personal" {
			contactEmail := seaserv.Email2ContactEmail(repo["user"])
			result["user_name"] = seaserv.Email2Nickname(contactEmail)
			result["user_email"] = repo["user"]
			result["contact_email"] = contactEmail
		}

		returnedResult = append(returnedResult, result)
	}

	return returnedResult, nil
}
