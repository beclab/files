package goseahub

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/goseaserv"
	"k8s.io/klog/v2"
	"log"
	"net/http"
	"strings"
)

func createDefaultLibrary(request *http.Request, newUsername string) (string, error) {
	username := newUsername
	if username == "" {
		return "", errors.New("username is empty")
	}
	klog.Infof("Create Default Library: username=%s", username)

	var defaultRepo string
	var err error

	klog.Infoln("Create Default Library: before create")
	defaultRepo, err = goseaserv.GlobalSeafileAPI.CreateRepo("My Library", "My Library", username, nil, 2)
	if err != nil {
		klog.Infof("Create repo failed: %v", err)
		return "", err
	}
	klog.Infof("Create Default Library: create repo success, repo_id=%s", defaultRepo)

	sysRepoID := getSystemDefaultRepoID()
	klog.Infof("Create Default Library: create repo success, sys_repo_id=%s", sysRepoID)
	if sysRepoID == "" {
		return defaultRepo, nil
	}

	dirents, err := goseaserv.GlobalSeafileAPI.ListDirByPath(sysRepoID, "/", -1, -1)
	if err != nil {
		klog.Infof("List dir failed: %v", err)
		return defaultRepo, err
	}

	for _, e := range dirents {
		objName := e["obj_name"]
		objNameBytes, err := json.Marshal(objName)
		if err != nil {
			klog.Infof("Parse obj_name failed: %v", err)
			return defaultRepo, err
		}
		_, err = goseaserv.GlobalSeafileAPI.CopyFile(
			sysRepoID, "/", string(objNameBytes),
			defaultRepo, "/", string(objNameBytes),
			username, 0, 0,
		)
		if err != nil {
			klog.Infof("Copy file failed: %v", err)
			return defaultRepo, err
		}
	}
	return defaultRepo, nil
}

func CallbackCreateHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		klog.Infof("Error parsing request body: %v", err)
		return http.StatusBadRequest, err
	}
	log.Printf("Received data: %v", data)

	name, ok := data["name"].(string)
	if !ok {
		klog.Infoln("Name field is missing or not a string")
		return http.StatusBadRequest, nil
	}

	newUserUsername := strings.TrimSpace(name)
	if newUserUsername != "" {
		newUserEmail := newUserUsername + "@auth.local"
		klog.Infof("Try to create user for %s", newUserEmail)

		isNew, err := createUser(newUserEmail)
		if err != nil {
			klog.Infof("Error creating user: %v", err)
			return http.StatusInternalServerError, err
		}

		if isNew {
			virtualID := newUserEmail
			klog.Infof("Try to create default library for %s with virtual_id %s", newUserEmail, virtualID)

			repoID, err := createDefaultLibrary(r, virtualID) // TODO
			if err != nil {
				klog.Infof("Create default library for %s failed: %v", newUserEmail, err)
			} else {
				klog.Infof("Create default library %s for %s successfully!", repoID, newUserEmail)
			}
		}
	}

	return 0, nil
}

func createUser(email string) (bool, error) {
	allUsers, err := ListAllUsers()
	if err != nil {
		klog.Errorf("Error listing users: %v", err)
		return false, err
	}

	if existedUser, ok := allUsers[email]; ok {
		if existedEmail, ok := existedUser["email"]; ok && existedEmail != "" {
			klog.Infof("Contact Email %s with Virtual Email %s already exist. Ignore this procedure!", email, existedEmail)
			return false, nil
		}
	}

	klog.Infof("Email %s not exist in memory cache. Will do this procedure!", email)

	resultCode := SaveUser(email, "abcd123456", true, true)
	if resultCode != 0 {
		klog.Infof("Error creating user: %v", email)
		return false, errors.New("error creating user")
	}

	klog.Infof("User %s created successfully", email)
	return true, nil
}

func CallbackDeleteHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	var requestData map[string]string
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		return http.StatusBadRequest, err
	}
	defer r.Body.Close()

	username, exists := requestData["name"]
	if !exists {
		return http.StatusBadRequest, errors.New("Missing name field")
	}
	email := username + "@auth.local"

	err := removeUser(email)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

func removeUser(email string) error {
	allUsers, err := ListAllUsers()
	if err != nil {
		klog.Errorf("Error listing users: %v", err)
		return err
	}

	existedUser, exists := allUsers[email]
	if !exists {
		klog.Infof("Contact Email %s not existed. Ignore procedure.", email)
		return nil
	}
	klog.Infof("User %v with Contact Email %s existed!", existedUser, email)

	virtualID := email // existedUser["email"]
	klog.Infof("Contact Email %s with Virtual Email %s exists. Proceeding...", email, virtualID)

	klog.Infof("Deleting user %s with virtual_id %s...", email, virtualID)
	err = DeleteUser(virtualID)
	if err != nil {
		klog.Errorf("Error deleting user: %v", err)
		return err
	}
	klog.Infof("Successfully deleted user %s with virtual_id %s", email, virtualID)
	return nil
}
