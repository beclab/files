package seahub

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/drives"
	"files/pkg/redisutils"
	"fmt"
	"k8s.io/klog/v2"
	"net/http"
	"time"
)

var MIGRATED = false

func SeahubUsersGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	MigrateSeahubUserToRedis(r.Header)
	responseData, err := seaserv.ListAllUsers()
	if err != nil {
		klog.Errorf("ListAllUsers failed: %v", err)
		return http.StatusInternalServerError, err
	}
	return common.RenderJSON(w, r, responseData)
}

// temp func, just for temp compatible before repo CRUD func finished
func MigrateSeahubUserToRedis(header http.Header) error {
	if MIGRATED {
		return nil
	}
	req, err := http.NewRequest("GET", "http://127.0.0.1:80/seahub/api/v2.1/admin/users/", nil)
	if err != nil {
		klog.Errorf("Request creation failed: %v", err)
		return err
	}
	req.Header = header.Clone()
	drives.RemoveAdditionalHeaders(&req.Header)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		klog.Errorf("HTTP request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		klog.Errorf("Unexpected status code: %d", resp.StatusCode)
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Email        string `json:"email"`
			ContactEmail string `json:"contact_email"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		klog.Errorf("JSON decode failed: %v", err)
		return err
	}

	for _, user := range result.Data {
		if user.Email == user.ContactEmail {
			continue
		}
		email := user.ContactEmail
		username := user.Email

		if email == "" || username == "" {
			klog.Warningf("Skipping invalid data: email=%q, user=%q", email, username)
			continue
		}

		if err := redisutils.RedisClient.HSet("old_seahub_email_map", email, username).Err(); err != nil {
			klog.Errorf("Redis HSET failed: %v", err)
			continue
		}
	}

	resultMap, err := redisutils.RedisClient.HGetAll("old_seahub_email_map").Result()
	if err != nil {
		klog.Errorf("Failed to read from Redis: %v", err)
		return err
	}

	klog.Info("===== Redis old_seahub_email_map contents =====")
	for email, user := range resultMap {
		klog.Infof("Email: %s -> User: %s", email, user)
	}
	klog.Info("=============================================")

	MIGRATED = true
	return nil
}
