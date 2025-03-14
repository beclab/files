package http

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/redisutils"
	"fmt"
	"github.com/go-redis/redis"
	"k8s.io/klog/v2"
	"net/http"
	"time"
)

func resourceMountHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	respJson, err := files.MountPathIncluster(r)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	return common.RenderJSON(w, r, respJson)
}

func resourceUnmountHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       r.URL.Path,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})
	if err != nil {
		return common.ErrToStatus(err), err
	}

	respJson, err := files.UnmountPathIncluster(r, file.Path)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	return common.RenderJSON(w, r, respJson)
}

func smbHistoryGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
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

func smbHistoryPutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
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

func smbHistoryDeleteHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
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
