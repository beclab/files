package seaserv

import (
	"errors"
	"files/pkg/redisutils"
	"fmt"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
)

var UNUSABLE_PASSWORD = "!" // This will never be a valid hash

func SaveUser(username, password string, isStaff, isActive bool) int {
	var resultCode int

	emailuser, err := GlobalCcnetAPI.GetEmailuser(username)
	if err != nil {
		return -1
	}

	if emailuser != nil && (strings.EqualFold(emailuser["source"], "db") || strings.EqualFold(emailuser["source"], "ldapimport")) {
		if password == "" {
			password = UNUSABLE_PASSWORD
		}

		actualSource := "LDAP"
		if strings.EqualFold(emailuser["source"], "DB") {
			actualSource = "DB"
		}

		if !isActive {
			if _, err := GlobalSeafileAPI.DeleteRepoTokensByEmail(username); err != nil {
				klog.Infof("Error clearing token for user %s: %v", username, err)
			}
		}

		userId, err := strconv.Atoi(emailuser["id"])
		if err != nil {
			klog.Errorf("Error converting email user id %s: %v", emailuser["id"], err)
			return -1
		}
		resultCode, err = GlobalCcnetAPI.UpdateEmailuser(
			actualSource,
			userId,
			password,
			boolToInt(isStaff),
			boolToInt(isActive),
		)
		if err != nil {
			klog.Errorf("Error updating user %s: %v", username, err)
			return -1
		}
	} else {
		resultCode, err = GlobalCcnetAPI.AddEmailuser(
			username,
			password,
			boolToInt(isStaff),
			boolToInt(isActive),
		)
		if err != nil {
			klog.Errorf("Error adding user %s: %v", username, err)
			return -1
		}
	}

	return resultCode // -1 stands for failed; 0 stands for success
}

func DeleteUser(username string) error {
	var resultCode int

	emailuser, err := GlobalCcnetAPI.GetEmailuser(username)
	if err != nil {
		return err
	}

	actualSource := "LDAP"
	if strings.EqualFold(emailuser["source"], "DB") {
		actualSource = "DB"
	}

	ownedRepos, err := GlobalSeafileAPI.GetOwnedRepoList(username, false, -1, -1)

	for _, repo := range ownedRepos {
		resultCode, err = GlobalSeafileAPI.RemoveRepo(repo["id"])
		if err != nil || resultCode != 0 {
			return errors.New(fmt.Sprintf("Error removing repo owned by user %s", username))
		}
	}

	repos, err := GlobalSeafileAPI.GetShareInRepoList(username, -1, -1)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		resultCode, err = GlobalSeafileAPI.RemoveShare(repo["repo_id"], repo["user"], username)
		if err != nil || resultCode != 0 {
			return errors.New(fmt.Sprintf("Error removing share in repo for user %s", username))
		}
	}

	resultCode, err = GlobalSeafileAPI.DeleteRepoTokensByEmail(username)
	if err != nil || resultCode != 0 {
		return errors.New(fmt.Sprintf("Error clearing token for user %s", username))
	}

	resultCode, err = GlobalCcnetAPI.RemoveGroupUser(username)
	if err != nil || resultCode != 0 {
		return errors.New(fmt.Sprintf("Error clearing group user for user %s", username))
	}

	resultCode, err = GlobalCcnetAPI.RemoveEmailuser(actualSource, username)
	if err != nil || resultCode != 0 {
		return errors.New(fmt.Sprintf("Error clearing email user for user %s", username))
	}

	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// temp compatible
func GetOldUsername(key string) string {
	usernameMap, err := redisutils.RedisClient.HGetAll("old_seahub_email_map").Result()
	if err != nil {
		klog.Error(err)
		return key
	}
	if len(usernameMap) > 0 {
		if val, exists := usernameMap[key]; exists {
			return val
		}
	}
	return key
}

func Email2ContactEmail(value string) string {
	emailMap, err := redisutils.RedisClient.HGetAll("old_seahub_email_map").Result()
	if err != nil {
		klog.Error(err)
	} else {
		for k, v := range emailMap {
			if v == value {
				return k
			}
		}
	}
	return value
}

func Email2Nickname(value string) string {
	var nickname string
	parts := strings.Split(value, "@")
	nickname = parts[0]
	return nickname
}

func ListAllUsers() (map[string]map[string]interface{}, error) {
	emailUserCount, err := GlobalCcnetAPI.CountEmailusers("DB")
	if err != nil {
		klog.Errorf("count email users failed: %v", err.Error())
		return nil, err
	}
	inactiveEmailUserCount, err := GlobalCcnetAPI.CountInactiveEmailusers("DB")
	if err != nil {
		klog.Errorf("count inactive email users failed: %v", err.Error())
		return nil, err
	}
	totalCount := emailUserCount + inactiveEmailUserCount

	users, err := GlobalCcnetAPI.GetEmailusers("DB", 0, totalCount, nil)

	for key, value := range users {
		klog.Infof("~~~Debug log: Key: %v, Value: %v (Type: %T)\n", key, value, value)
	}

	data := make(map[string]map[string]interface{})

	for _, user := range users {
		info := make(map[string]interface{})
		username := user["email"]
		info["username"] = username
		info["email"] = username
		info["name"] = Email2Nickname(Email2ContactEmail(username))
		info["contact_email"] = Email2ContactEmail(username)

		info["is_staff"] = user["is_staff"]
		info["is_active"] = user["is_active"]

		data[info["username"].(string)] = info
	}

	// just for test
	total := make(map[string]interface{})
	total["total"] = totalCount
	data["total"] = total

	klog.Infoln("User data:", dataToString(data))

	return data, nil
}

func dataToString(data map[string]map[string]interface{}) string {
	str := "{"
	for k, v := range data {
		str += "\"" + k + "\": {"
		for key, val := range v {
			str += "\"" + key + "\": "
			switch val.(type) {
			case string:
				str += "\"" + val.(string) + "\","
			case bool:
				str += strconv.FormatBool(val.(bool)) + ","
			default:
				str += "\"\","
			}
		}
		str = str[:len(str)-1] + "},"
	}
	if len(data) > 0 {
		str = str[:len(str)-1]
	}
	str += "}"
	return str
}
