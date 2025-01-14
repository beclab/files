package http

import (
	"encoding/json"
	"errors"
	"github.com/filebrowser/filebrowser/v2/my_redis"
	"net/http"
	"time"
)

var smbHistoryGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return http.StatusBadRequest, errors.New("missing X-Bfl-User header")
	}

	key := bflName + "_smb_history"

	zset, err := my_redis.RedisZRevRangeWithScores(key, 0, -1)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	var result []map[string]interface{}

	for _, entry := range zset {
		member := entry.Value
		score := entry.Score

		hashKey := key + "_url_details:" + member
		urlInfo, err := my_redis.RedisHGetAll(hashKey)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		item := map[string]interface{}{
			"url":       urlInfo["url"],
			"username":  urlInfo["username"],
			"password":  urlInfo["password"],
			"timestamp": score,
		}

		result = append(result, item)
	}

	return renderJSON(w, r, result)
})

type SMBHistoryData struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

var smbHistoryPutHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
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
	for _, data := range requestData {
		if err := my_redis.RedisZAdd(key, data.URL, score); err != nil {
			return http.StatusInternalServerError, err
		}

		hashKey := key + "_url_details:" + data.URL
		if err := my_redis.RedisHMSet(hashKey, map[string]interface{}{
			"url":      data.URL,
			"username": data.Username,
			"password": data.Password,
		}); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	return renderJSON(w, r, "Successfully added/updated SMB history and hash")
})

//func stringSliceToInterfaceSlice(strings []string) []interface{} {
//	interfaces := make([]interface{}, len(strings))
//	for i, str := range strings {
//		interfaces[i] = str
//	}
//	return interfaces
//}

var smbHistoryDeleteHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
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
	for _, data := range requestData {
		urls = append(urls, data.URL)

		hashKey := key + "_url_details:" + data.URL
		if err := my_redis.RedisDelKey(hashKey); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	if err := my_redis.RedisZRem(key, urls); err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, "Successfully deleted SMB history")
})
