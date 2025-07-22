package goseahub

import (
	"errors"
	"files/pkg/goseaserv"
	"fmt"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
)

var UNUSABLE_PASSWORD = "!" // This will never be a valid hash

func SaveUser(username, password string, isStaff, isActive bool) int {
	var resultCode int

	emailuser, err := goseaserv.GlobalCcnetAPI.GetEmailuser(username)
	if err != nil {
		return -1
	}

	if emailuser != nil && (strings.EqualFold(emailuser["source"], "db") || strings.EqualFold(emailuser["source"], "ldapimport")) {
		if password == "" {
			password = UNUSABLE_PASSWORD
		}

		actualSource := "LDAP"
		if strings.EqualFold(emailuser["source"], "DB") {
			actualSource = "DB"
		}

		if !isActive {
			if _, err := goseaserv.GlobalSeafileAPI.DeleteRepoTokensByEmail(username); err != nil {
				klog.Infof("Error clearing token for user %s: %v", username, err)
			}
		}

		userId, err := strconv.Atoi(emailuser["id"])
		if err != nil {
			klog.Errorf("Error converting email user id %s: %v", emailuser["id"], err)
			return -1
		}
		resultCode, err = goseaserv.GlobalCcnetAPI.UpdateEmailuser(
			actualSource,
			userId,
			password,
			boolToInt(isStaff),
			boolToInt(isActive),
		)
		if err != nil {
			klog.Errorf("Error updating user %s: %v", username, err)
			return -1
		}
	} else {
		resultCode, err = goseaserv.GlobalCcnetAPI.AddEmailuser(
			username,
			password,
			boolToInt(isStaff),
			boolToInt(isActive),
		)
		if err != nil {
			klog.Errorf("Error adding user %s: %v", username, err)
			return -1
		}
	}

	return resultCode // -1 stands for failed; 0 stands for success
}

func DeleteUser(username string) error {
	var resultCode int

	emailuser, err := goseaserv.GlobalCcnetAPI.GetEmailuser(username)
	if err != nil {
		return err
	}

	actualSource := "LDAP"
	if strings.EqualFold(emailuser["source"], "DB") {
		actualSource = "DB"
	}

	ownedRepos, err := goseaserv.GlobalSeafileAPI.GetOwnedRepoList(username, false, -1, -1)

	for _, repo := range ownedRepos {
		resultCode, err = goseaserv.GlobalSeafileAPI.RemoveRepo(repo["id"])
		if err != nil || resultCode != 0 {
			return errors.New(fmt.Sprintf("Error removing repo owned by user %s", username))
		}
	}

	repos, err := goseaserv.GlobalSeafileAPI.GetShareInRepoList(username, -1, -1)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		resultCode, err = goseaserv.GlobalSeafileAPI.RemoveShare(repo["repo_id"], repo["user"], username)
		if err != nil || resultCode != 0 {
			return errors.New(fmt.Sprintf("Error removing share in repo for user %s", username))
		}
	}

	resultCode, err = goseaserv.GlobalSeafileAPI.DeleteRepoTokensByEmail(username)
	if err != nil || resultCode != 0 {
		return errors.New(fmt.Sprintf("Error clearing token for user %s", username))
	}

	resultCode, err = goseaserv.GlobalCcnetAPI.RemoveGroupUser(username)
	if err != nil || resultCode != 0 {
		return errors.New(fmt.Sprintf("Error clearing group user for user %s", username))
	}

	resultCode, err = goseaserv.GlobalCcnetAPI.RemoveEmailuser(actualSource, username)
	if err != nil || resultCode != 0 {
		return errors.New(fmt.Sprintf("Error clearing email user for user %s", username))
	}

	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
