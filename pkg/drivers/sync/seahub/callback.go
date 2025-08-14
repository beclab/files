package seahub

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"k8s.io/klog/v2"
	"net/http"
	"strings"
)

func createDefaultLibrary(newUsername string) (string, error) {
	username := newUsername
	if username == "" {
		return "", errors.New("username is empty")
	}
	klog.Infof("Create Default Library: username=%s", username)

	var defaultRepo string
	var err error

	klog.Infoln("Create Default Library: before create")
	defaultRepo, err = seaserv.GlobalSeafileAPI.CreateRepo("My Library", "My Library", username, nil, 2)
	if err != nil {
		klog.Infof("Create repo failed: %v", err)
		return "", err
	}
	klog.Infof("Create Default Library: create repo success, repo_id=%s", defaultRepo)

	sysRepoId := getSystemDefaultRepoId()
	klog.Infof("Create Default Library: create repo success, sys_repo_id=%s", sysRepoId)
	if sysRepoId == "" {
		return defaultRepo, nil
	}

	dirents, err := seaserv.GlobalSeafileAPI.ListDirByPath(sysRepoId, "/", -1, -1)
	if err != nil {
		klog.Infof("List dir failed: %v", err)
		return defaultRepo, err
	}

	for _, e := range dirents {
		objName := e["obj_name"]
		_, err = seaserv.GlobalSeafileAPI.CopyFile(
			sysRepoId, "/", string(common.ToBytes(objName)),
			defaultRepo, "/", string(common.ToBytes(objName)),
			username, 0, 0,
		)
		if err != nil {
			klog.Infof("Copy file failed: %v", err)
			return defaultRepo, err
		}
	}
	return defaultRepo, nil
}

func CallbackCreateHandler(w http.ResponseWriter, r *http.Request, d *common.HttpData) (int, error) {
	MigrateSeahubUserToRedis(r.Header)
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		klog.Infof("Error parsing request body: %v", err)
		return http.StatusBadRequest, err
	}
	klog.Infof("Received data: %v", data)

	bflName, ok := data["name"].(string)
	if !ok {
		klog.Infoln("Name field is missing or not a string")
		return http.StatusBadRequest, nil
	}

	bflName = strings.TrimSpace(bflName)
	if bflName != "" {
		newUsername := bflName + "@auth.local"
		klog.Infof("Try to create user for %s", newUsername)

		isNew, err := createUser(newUsername)
		if err != nil {
			klog.Infof("Error creating user: %v", err)
			return http.StatusInternalServerError, err
		}

		if isNew {
			repoId, err := createDefaultLibrary(newUsername)
			if err != nil {
				klog.Infof("Create default library for %s failed: %v", newUsername, err)
			} else {
				klog.Infof("Create default library %s for %s successfully!", repoId, newUsername)
			}
		}
	}

	return 0, nil
}

func createUser(username string) (bool, error) {
	allUsers, err := seaserv.ListAllUsers()
	if err != nil {
		klog.Errorf("Error listing users: %v", err)
		return false, err
	}

	if existedUser, ok := allUsers[username]; ok {
		if existedUsername, ok := existedUser["username"]; ok && existedUsername != "" {
			klog.Infof("Username %s already exist. Ignore this procedure!", username)
			return false, nil
		}
	}

	klog.Infof("Username %s not exist in memory cache. Will do this procedure!", username)

	resultCode := seaserv.SaveUser(username, "abcd123456", true, true)
	if resultCode != 0 {
		klog.Infof("Error creating user: %s", username)
		return false, errors.New("error creating user")
	}

	klog.Infof("User %s created successfully", username)
	return true, nil
}

func CallbackDeleteHandler(w http.ResponseWriter, r *http.Request, d *common.HttpData) (int, error) {
	MigrateSeahubUserToRedis(r.Header)
	var requestData map[string]string
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		return http.StatusBadRequest, err
	}
	defer r.Body.Close()

	bflName, exists := requestData["name"]
	if !exists {
		return http.StatusBadRequest, errors.New("Missing name field")
	}
	username := bflName + "@auth.local"

	err := removeUser(username)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

func removeUser(username string) error {
	allUsers, err := seaserv.ListAllUsers()
	if err != nil {
		klog.Errorf("Error listing users: %v", err)
		return err
	}

	existedUser, exists := allUsers[username]
	if !exists {
		klog.Infof("Username %s not existed. Ignore procedure.", username)
		return nil
	}
	klog.Infof("User %v with username %s existed!", existedUser, username)

	err = seaserv.DeleteUser(username)
	if err != nil {
		klog.Errorf("Error deleting user: %v", err)
		return err
	}
	klog.Infof("Successfully deleted user %s", username)
	return nil
}
