package goseahub

import (
	"errors"
	"files/pkg/common"
	"files/pkg/goseahub/goseafile"
	"files/pkg/postgres"
	"files/pkg/redisutils"
	"k8s.io/klog/v2"
	"net/http"
)

func SeahubUsersGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	responseData, err := goseafile.ListAllUsers()
	if err != nil {
		klog.Errorf("ListAllUsers failed: %v", err)
		return http.StatusInternalServerError, err
	}
	return common.RenderJSON(w, r, responseData)
}

// temp func, just for temp compatible before repo CRUD func finished
func MigrateSeahubEmailToRedis() error {
	var profileResults []map[string]interface{}

	if postgres.SeahubDBServer == nil {
		return errors.New("no seahub db server")
	}

	if err := postgres.SeahubDBServer.Raw("SELECT contact_email, \"user\" FROM profile_profile").Scan(&profileResults).Error; err != nil {
		klog.Errorf("Database query failed: %v", err)
		return err
	}

	for _, row := range profileResults {
		email, emailOk := row["contact_email"].(string)
		user, userOk := row["user"].(string)

		if !emailOk || !userOk || email == "" || user == "" {
			klog.Warningf("Skipping invalid data: email=%v, user=%v", email, user)
			continue
		}

		if err := redisutils.RedisClient.HSet("old_seahub_email_map", email, user).Err(); err != nil {
			klog.Errorf("Redis HSET failed: %v", err)
			continue
		}
	}

	result, err := redisutils.RedisClient.HGetAll("old_seahub_email_map").Result()
	if err != nil {
		klog.Errorf("Failed to read from Redis: %v", err)
		return err
	}

	klog.Info("===== Redis old_seahub_email_map contents =====")
	for email, user := range result {
		klog.Infof("Email: %s -> User: %s", email, user)
	}
	klog.Info("=============================================")

	return nil
}
