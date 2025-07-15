package goseahub

import (
	"files/pkg/goseahub/models"
	"files/pkg/goseaserv"
	"k8s.io/klog/v2"
	"log"
	"strconv"
)

func ListAllUsers() (map[string]map[string]interface{}, error) {
	if goseaserv.GlobalCcnetAPI == nil {
		klog.Errorf("~~~Debug log: GlobalCcnetAPI is nil")
		goseaserv.GlobalCcnetAPI = goseaserv.NewCcnetAPI(goseaserv.SeafservThreadedRpc)
	}

	emailUserCount, err := goseaserv.GlobalCcnetAPI.CountEmailusers("DB")
	if err != nil {
		klog.Errorf("count email users failed: %v", err.Error())
		return nil, err
	}
	inactiveEmailUserCount, err := goseaserv.GlobalCcnetAPI.CountInactiveEmailusers("DB")
	if err != nil {
		klog.Errorf("count inactive email users failed: %v", err.Error())
		return nil, err
	}
	totalCount := emailUserCount + inactiveEmailUserCount

	users, err := goseaserv.GlobalCcnetAPI.GetEmailusers("DB", 0, totalCount, nil)

	for key, value := range users {
		klog.Infof("~~~Debug log: Key: %v, Value: %v (Type: %T)\n", key, value, value)
	}

	data := make(map[string]map[string]interface{})

	for _, user := range users {
		profile, err := models.GlobalProfileManager.GetProfileByUser(user["email"])
		if err != nil {
			klog.Errorf("get profile by user failed: %v", err.Error())
			profile = nil
		}

		info := make(map[string]interface{})
		userEmail := user["email"]
		info["email"] = userEmail
		info["name"] = models.Email2Nickname(models.Email2ContactEmail(userEmail))
		info["contact_email"] = models.Email2ContactEmail(userEmail)

		if profile != nil {
			info["login_id"] = profile.LoginID
		} else {
			info["login_id"] = ""
		}

		info["is_staff"] = user["is_staff"]
		info["is_active"] = user["is_active"]

		data[info["contact_email"].(string)] = info
	}

	// just for test
	total := make(map[string]interface{})
	total["total"] = totalCount
	data["total"] = total

	log.Println("User data:", dataToString(data))

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
