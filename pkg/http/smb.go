package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/redisutils"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"k8s.io/klog/v2"
)

type MountRequestData struct {
	SMBPath  string `json:"smbPath"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func ResourceMountedHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	global.GlobalMounted.Updated()
	return common.RenderJSON(w, r, map[string]interface{}{
		"code":         0,
		"message":      "success",
		"mounted_data": global.GlobalMounted.GetMountedData(),
	})
}

func ResourceMountHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	var data MountRequestData
	err = json.Unmarshal(bodyBytes, &data)
	if err != nil {
		klog.Errorln("Error unmarshalling JSON:", err)
		return common.ErrToStatus(err), err
	}

	respJson, err := files.MountPathIncluster(r)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	if int(respJson["code"].(float64)) != http.StatusOK {
		klog.Warningf(respJson["message"].(string))
		if strings.Contains(respJson["message"].(string), "mount error(13)") {
			respJson["message"] = "Incorrect username or password"
		}
		if strings.Contains(respJson["message"].(string), "mount error(113)") {
			respJson["message"] = "Unable to find suitable address"
		}
		if strings.Contains(respJson["message"].(string), "mount error(115)") {
			respJson["message"] = "Cannot connect to samba server"
		}
	}

	global.GlobalMounted.Updated()
	return common.RenderJSON(w, r, respJson)
}

func ResourceUnmountHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	var p = r.URL.Path
	var path = strings.TrimPrefix(p, "/api/unmount")
	if path == "" {
		return http.StatusBadRequest, errors.New("path invalid")
	}

	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return http.StatusBadRequest, errors.New("user not found")
	}
	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if fileParam.FileType == common.Sync {
		return md5Sync(fileParam, w, r)
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	urlPath := uri + fileParam.Path

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       strings.TrimPrefix(urlPath, "/data"),
		Modify:     true,
		Expand:     false,
		ReadHeader: true,
	})
	if err != nil {
		return common.ErrToStatus(err), err
	}

	respJson, err := files.UnmountPathIncluster(r, file.Path)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	global.GlobalMounted.Updated()
	return common.RenderJSON(w, r, respJson)
}

func SmbHistoryGetHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return http.StatusBadRequest, errors.New("missing X-Bfl-User header")
	}

	key := bflName + "_smb_history"

	zset, err := redisutils.RedisClient.ZRevRangeWithScores(key, 0, -1).Result()
	if err != nil {
		return common.ErrToStatus(err), fmt.Errorf("get reverse range with scores from zset failed: %v", err)
	}

	var result []map[string]interface{}

	for _, entry := range zset {
		member := entry.Member.(string)
		score := entry.Score

		hashKey := key + "_url_details:" + member
		var urlInfo map[string]string
		urlInfo, err = redisutils.RedisClient.HGetAll(hashKey).Result()
		if err != nil {
			return common.ErrToStatus(err), err
		}

		item := map[string]interface{}{
			"url":       urlInfo["url"],
			"username":  urlInfo["username"],
			"password":  urlInfo["password"],
			"timestamp": score,
		}

		result = append(result, item)
	}

	return common.RenderJSON(w, r, result)
}

type SMBHistoryData struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func SmbHistoryPutHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return http.StatusBadRequest, errors.New("missing X-Bfl-User header")
	}

	key := bflName + "_smb_history"

	var requestData []SMBHistoryData
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		return http.StatusBadRequest, err
	}

	score := float64(time.Now().Unix())
	for _, datum := range requestData {
		err := redisutils.RedisClient.ZAdd(key, redis.Z{Score: score, Member: datum.URL}).Err()
		if err != nil {
			klog.Errorln("add new member to zset failed: ", err)
			return common.ErrToStatus(err), err
		}

		hashKey := key + "_url_details:" + datum.URL

		var fields = map[string]interface{}{
			"url":      datum.URL,
			"username": datum.Username,
			"password": datum.Password,
		}
		for field, value := range fields {
			_, err = redisutils.RedisClient.HSet(hashKey, field, value).Result()
			if err != nil {
				klog.Errorf("set hash field '%s' failed: %v\n", field, err)
				return common.ErrToStatus(err), err
			}
		}
	}

	return common.RenderJSON(w, r, "Successfully added/updated SMB history and hash")
}

func SmbHistoryDeleteHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return http.StatusBadRequest, errors.New("missing X-Bfl-User header")
	}

	key := bflName + "_smb_history"

	var requestData []SMBHistoryData
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		return http.StatusBadRequest, err
	}

	var urls []string
	for _, datum := range requestData {
		urls = append(urls, datum.URL)

		hashKey := key + "_url_details:" + datum.URL
		_, err := redisutils.RedisClient.Del(hashKey).Result()
		if err != nil {
			klog.Errorf("Delete key failed: %v\n", err)
			return common.ErrToStatus(err), err
		}
	}

	err := redisutils.RedisClient.ZRem(key, urls).Err()
	if err != nil {
		klog.Errorln("remove member for zset failed: ", err)
		return common.ErrToStatus(err), err
	}

	return common.RenderJSON(w, r, "Successfully deleted SMB history")
}
