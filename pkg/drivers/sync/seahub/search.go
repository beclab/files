package seahub

import (
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/hertz/biz/dal/database"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

func HandleSearch(owner, q string) ([]byte, error) {
	username := owner + "@auth.local"
	result := []map[string]string{}
	ownedRepos, err := seaserv.GlobalSeafileAPI.GetOwnedRepoList(username, false, -1, -1)
	if err != nil {
		klog.Errorln(err)
		return nil, nil
	}
	for _, repo := range ownedRepos {
		repoId := repo["id"]
		data, err := HandleSearchFile(username, repoId, q, false)
		if err != nil {
			klog.Errorln(err)
			return nil, err
		}
		result = append(result, data...)
	}
	sharedRepos, err := seaserv.GlobalSeafileAPI.GetShareInRepoList(username, -1, -1)
	if err != nil {
		klog.Errorln(err)
		return nil, nil
	}
	for _, repo := range sharedRepos {
		repoId := repo["id"]
		data, err := HandleSearchFile(username, repoId, q, true)
		if err != nil {
			klog.Errorln(err)
			return nil, err
		}
		result = append(result, data...)
	}
	return common.ToBytes(map[string][]map[string]string{"data": result}), nil
}

func HandleSearchFile(username, repoId, q string, shared bool) ([]map[string]string, error) {
	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, errors.New("repo not found")
	}

	perm, err := CheckFolderPermission(username, repoId, "/")
	if err != nil {
		return nil, err
	}
	if perm == "" {
		return nil, errors.New("permission denied")
	}

	shareId := ""
	if shared {
		originRepoId := repoId
		if repo["origin_repo_id"] != "" {
			originRepoId = repo["origin_repo_id"]
		}
		sharePath, err := database.QuerySharePathByExtendAndMember(originRepoId, strings.TrimSuffix(username, "@auth.local"))
		if err != nil {
			klog.Errorln(err)
			return nil, err
		}
		klog.Infof("sharePath: %+v", sharePath)
		if len(sharePath) > 0 {
			shareId = sharePath[0].ID
		}
		if shareId == "" {
			klog.Warningf("sharePath not found, repoId: %s, username: %s", originRepoId, username)
			return nil, nil
		}
	}

	searchedFiles, err := seaserv.GlobalSeafileAPI.SearchFiles(repoId, q)
	if err != nil {
		return nil, err
	}
	var folderList, fileList []map[string]string
	var direntInfo map[string]string

	if strings.Contains(repo["repo_name"], q) {
		mtimeInt, err := strconv.ParseInt(repo["last_modified"], 10, 64)
		if err != nil {
			return nil, err
		}
		if shared {
			direntInfo = map[string]string{
				"file_type":   common.Share,
				"file_extend": shareId,
				"path":        "/",
				"size":        "0",
				"mtime":       TimestampToISO(mtimeInt),
				"repo_name":   repo["name"],
				"type":        "folder",
				"title":       repo["name"],
			}
		} else {
			direntInfo = map[string]string{
				"file_type":   common.Sync,
				"file_extend": repoId,
				"path":        "/",
				"size":        "0",
				"mtime":       TimestampToISO(mtimeInt),
				"repo_name":   repo["name"],
				"type":        "folder",
				"title":       repo["name"],
			}
		}
		folderList = append(folderList, direntInfo)
	}

	for _, f := range searchedFiles {
		mtimeInt, err := strconv.ParseInt(f["mtime"], 10, 64)
		if err != nil {
			return nil, err
		}
		if shared {
			direntInfo = map[string]string{
				"file_type":   common.Share,
				"file_extend": shareId,
				"path":        f["path"],
				"size":        f["size"],
				"mtime":       TimestampToISO(mtimeInt),
				"repo_name":   repo["name"],
			}

		} else {
			direntInfo = map[string]string{
				"file_type":   common.Sync,
				"file_extend": repoId,
				"path":        f["path"],
				"size":        f["size"],
				"mtime":       TimestampToISO(mtimeInt),
				"repo_name":   repo["name"],
			}
		}

		if f["is_dir"] == "true" {
			direntInfo["type"] = "folder"
			direntInfo["title"] = filepath.Base(strings.TrimSuffix(f["path"], "/"))
			folderList = append(folderList, direntInfo)
		} else {
			direntInfo["type"] = "file"
			direntInfo["title"] = filepath.Base(f["path"])
			fileList = append(fileList, direntInfo)
		}
	}

	sort.Slice(folderList, func(i, j int) bool {
		return folderList[i]["mtime"] > folderList[j]["mtime"]
	})
	sort.Slice(fileList, func(i, j int) bool {
		return fileList[i]["mtime"] > fileList[j]["mtime"]
	})

	data := append(folderList, fileList...)
	return data, nil
}
