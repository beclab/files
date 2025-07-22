package goseahub

import (
	"files/pkg/redisutils"
	"k8s.io/klog/v2"
	"net/url"
	"strings"
)

func normalizeCacheKey(value, prefix, token string, maxLength int) string {
	key := prefix + value
	if token != "" {
		key += "_" + token
	}

	encoded := url.PathEscape(key)
	if maxLength > 0 && len(encoded) > maxLength {
		encoded = encoded[:maxLength]
	}
	return encoded
}

func Email2ContactEmail(value string) string {
	//if value == "" {
	//	return ""
	//}
	//
	//key := normalizeCacheKey(value, CONTACT_CACHE_PREFIX, "", 200)
	//if cached, found := contactCache.Get(key); found {
	//	if email, ok := cached.(string); ok && email != "" && strings.TrimSpace(email) != "" {
	//		return email
	//	}
	//}
	//
	//email := GlobalProfileManager.GetContactEmailByUser(value)
	//if email != "" && strings.TrimSpace(email) != "" {
	//	contactCache.Set(key, email, CONTACT_CACHE_TIMEOUT)
	//	return email
	//}

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
	//if value == "" {
	//	return ""
	//}
	//
	//key := normalizeCacheKey(value, NICKNAME_CACHE_PREFIX, "", 200)
	//if cached, found := nicknameCache.Get(key); found {
	//	if nickname, ok := cached.(string); ok && nickname != "" && strings.TrimSpace(nickname) != "" {
	//		return strings.TrimSpace(nickname)
	//	}
	//}
	//
	//query := GlobalProfileManager.Filter(map[string]interface{}{"user": value})
	//
	//var profile *Profile
	//if err := GetFirstObjectOrNone(query, &profile); err == nil {
	//	klog.Infof("Found profile: %s", profile.User)
	//	profile = nil
	//} else {
	//	klog.Infoln("Not found")
	//}
	//
	var nickname string
	//if profile != nil && profile.Nickname != "" && strings.TrimSpace(profile.Nickname) != "" {
	//	nickname = strings.TrimSpace(profile.Nickname)
	//} else {
	//	parts := strings.Split(value, "@")
	//	nickname = parts[0]
	//}
	//
	////nickname := GlobalProfileManager.GetContactEmailByUser(value)
	////if nickname != "" && strings.TrimSpace(nickname) != "" && nickname != value && strings.TrimSpace(nickname) != value {
	////	nickname = strings.TrimSpace(nickname)
	////} else {
	parts := strings.Split(value, "@")
	nickname = parts[0]
	////}
	//
	//nicknameCache.Set(key, nickname, NICKNAME_CACHE_TIMEOUT)
	return nickname
}
