package goseahub

import (
	"files/pkg/goseaserv"
	"k8s.io/klog/v2"
	"log"
	"strconv"
)

// 假设已存在的类型定义
type EmailUser struct {
	Email    string
	IsStaff  bool
	IsActive bool
}

type Profile struct {
	LoginID string
}

//// 假设的ccnet_api包方法
//var ccnet_api struct {
//	CountEmailUsers         func(string) int
//	CountInactiveEmailUsers func(string) int
//	GetEmailUsers           func(string, int, int) []*EmailUser
//}

// 假设的Profile模型方法
var ProfileModel struct {
	GetProfileByUser func(string) *Profile
}

// 假设的工具函数
func email2contact_email(email string) string {
	// 实现邮箱转换逻辑
	return email
}

func email2nickname(email string) string {
	// 实现昵称转换逻辑
	return email
}

func ListAllUsers() (map[string]map[string]interface{}, error) {
	// 计算总用户数
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

	// 获取所有用户
	users, err := goseaserv.GlobalCcnetAPI.GetEmailusers("DB", 0, totalCount, nil)

	data := make(map[string]map[string]interface{})

	for _, user := range users {
		// 获取用户资料
		//profile := ProfileModel.GetProfileByUser(user.Email)

		// 构建用户信息
		info := make(map[string]interface{})
		userEmail := user["email"]
		info["email"] = userEmail                                     // user.Email
		info["name"] = email2nickname(email2contact_email(userEmail)) //user.Email))
		info["contact_email"] = email2contact_email(userEmail)        //user.Email)

		// 处理可能为nil的profile
		//if profile != nil {
		//	info["login_id"] = profile.LoginID
		//} else {
		info["login_id"] = ""
		//}

		info["is_staff"] = user["is_staff"]   // user.IsStaff
		info["is_active"] = user["is_active"] // user.IsActive

		// 使用contact_email作为主键
		data[info["contact_email"].(string)] = info
	}

	// just for test
	total := make(map[string]interface{})
	total["total"] = totalCount
	data["total"] = total

	// 记录日志（需要实现String()方法）
	log.Println("User data:", dataToString(data))

	return data, nil
}

// 辅助函数用于日志输出
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
