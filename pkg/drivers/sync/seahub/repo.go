package seahub

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/models"
	"fmt"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var (
	PERMISSION_PREVIEW       = "preview"    // preview only on the web, can not be downloaded
	PERMISSION_PREVIEW_EDIT  = "cloud-edit" // preview only with edit on the web
	PERMISSION_READ          = "r"
	PERMISSION_READ_WRITE    = "rw"
	PERMISSION_ADMIN         = "admin"
	CUSTOM_PERMISSION_PREFIX = "custom"
)

func CheckFolderPermission(username, repoId, path string) (string, error) {
	repoStatus, err := seaserv.GlobalSeafileAPI.GetRepoStatus(repoId)
	if err != nil {
		return "", err
	}
	if repoStatus == 1 {
		return PERMISSION_READ, nil
	}
	result, err := seaserv.GlobalSeafileAPI.CheckPermissionByPath(repoId, path, username)
	if err != nil {
		return "", err
	}
	klog.Infof("%s!!", result)
	return result, nil
}

func repoHasBeenSharedOut(repoId string) (bool, error) {
	shared, err := seaserv.GlobalSeafileAPI.RepoHasBeenShared(repoId, true)
	if err != nil {
		return false, err
	}
	inner, err := seaserv.GlobalSeafileAPI.IsInnerPubRepo(repoId)
	if err != nil {
		return false, err
	}
	if shared || inner {
		return true, nil
	}
	return false, nil
}

// repo meta data, used in the future
func RepoGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	MigrateSeahubUserToRedis(r.Header)
	vars := mux.Vars(r)
	repoId := vars["repo_id"]
	klog.Infof("~~~Debug log: repoId: %s", repoId)

	bflName := r.Header.Get("X-Bfl-User")
	username := bflName + "@auth.local"
	oldUsername := seaserv.GetOldUsername(bflName + "@seafile.com") // temp compatible

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return http.StatusNotFound, err
	}

	permission, err := CheckFolderPermission(username, repoId, "/")
	if err != nil || permission == "" {
		permission, err = CheckFolderPermission(oldUsername, repoId, "/") // temp compatible
		if err != nil || permission == "" {
			return http.StatusForbidden, err
		}
		// return http.StatusForbidden, err
	}

	libNeedDecrypt := false
	encrypted, err := strconv.ParseBool(repo["encrypted"])
	if err != nil {
		return http.StatusBadRequest, err
	}
	passwordSet, err := seaserv.GlobalSeafileAPI.IsPasswordSet(repoId, username)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	oldPasswordSet, err := seaserv.GlobalSeafileAPI.IsPasswordSet(repoId, oldUsername) // temp compatible
	if err != nil {
		return http.StatusInternalServerError, err
	}
	passwordSet = passwordSet || oldPasswordSet

	if encrypted && passwordSet {
		libNeedDecrypt = true
	}

	repoOwner, err := seaserv.GlobalSeafileAPI.GetRepoOwner(repoId)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	hasSharedOut, err := repoHasBeenSharedOut(repoId)
	if err != nil {
		klog.Error(err)
		hasSharedOut = false
	}

	quota, err := seaserv.GlobalSeafileAPI.CheckQuota(repoId, 0)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	isVirtual, err := strconv.ParseBool(repo["is_virtual"])
	if err != nil {
		klog.Errorf("Error parsing is_virtual flag: %v", err)
		isVirtual = false
	}

	response := map[string]interface{}{
		"repo_id":       repo["id"],
		"repo_name":     repo["name"],
		"owner_email":   repoOwner,
		"owner_name":    seaserv.Email2Nickname(seaserv.Email2ContactEmail(repoOwner)),
		"owner_contact": seaserv.Email2ContactEmail(repoOwner),
		"size:":         repo["size"],
		"encrypted":     encrypted,
		"file_count":    repo["file_count"],
		"permission":    permission,
		"no_quota":      quota < 0,
		"is_admin":      bflName == seaserv.Email2Nickname(seaserv.Email2ContactEmail(repoOwner)),
		"is_virtual":    isVirtual,
		"shared_out":    hasSharedOut,
		"need_decrypt":  libNeedDecrypt,
		"last_modified": TimestampToISO(repo["last_modify"]),
		"status":        NormalizeRepoStatusCode(repo["status"]),
	}

	return common.RenderJSON(w, r, response)
}

func HandleGetRepoDir(header http.Header, fileParam *models.FileParam) ([]byte, error) {
	MigrateSeahubUserToRedis(header)
	repoId := fileParam.Extend
	klog.Infof("~~~Debug log: repoId: %s", repoId)

	thumbnailSize := 48

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil || repo == nil {
		return nil, errors.New("repo not found")
	}

	parentDir := normalizeDirPath(fileParam.Path)
	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, parentDir)
	if dirId == "" || err != nil {
		return nil, errors.New("folder not found")
	}

	bflName := header.Get("X-Bfl-User")
	username := bflName + "@auth.local"
	oldUsername := seaserv.GetOldUsername(bflName + "@seafile.com") // temp compatible
	useUsername := username

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil || permission == "" {
		permission, err = CheckFolderPermission(oldUsername, repoId, parentDir) // temp compatible
		if err != nil || permission == "" {
			return nil, err
		} else {
			useUsername = oldUsername
		}
	}

	parentDirs := generateParentDirs(parentDir, false)

	allDirInfo := []map[string]interface{}{}
	allFileInfo := []map[string]interface{}{}

	for _, dir := range parentDirs {
		klog.Infof("~~~Debug log: dir: %s", dir)
		dirInfo, fileInfo, err := getDirFileInfoList(
			useUsername, // username,
			repo,
			dir,
			true,
			thumbnailSize,
		)
		if err != nil {
			klog.Error(err)
			continue
		}
		allDirInfo = append(allDirInfo, dirInfo...)
		allFileInfo = append(allFileInfo, fileInfo...)
	}

	response := map[string]interface{}{
		"user_perm": permission,
		"dir_id":    dirId,
	}

	response["dirent_list"] = append(allDirInfo, allFileInfo...)

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

func normalizeDirPath(p string) string {
	if p == "" {
		return "/"
	}
	return "/" + strings.Trim(p, "/")
}

func generateParentDirs(parentDir string, withParents bool) []string {
	if !withParents {
		return []string{parentDir}
	}

	var dirs []string
	if parentDir == "/" {
		return []string{"/"}
	}

	current := "/"
	dirs = append(dirs, current)
	for _, part := range strings.Split(strings.Trim(parentDir, "/"), "/") {
		current = path.Join(current, part)
		dirs = append(dirs, current)
	}
	return dirs
}

const (
	THUMBNAIL_ROOT         = "/thumbnails"
	ENABLE_VIDEO_THUMBNAIL = true
)

func IsDirectory(modeStr string) (bool, error) {
	mode, err := strconv.ParseUint(modeStr, 10, 32)
	if err != nil {
		return false, err
	}

	return (mode & syscall.S_IFMT) == syscall.S_IFDIR, nil
}

var FILEEXT_TYPE_MAP = map[string]string{
	"jpg":   "image",
	"xmind": "xmind",
	"mp4":   "video",
}

func ConvertMap(input map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(input))
	for k, v := range input {
		result[k] = v
	}
	return result
}

func getThumbnailSrc(repoId string, size int, path string) string {
	trimmedPath := strings.TrimLeft(path, "/")

	return filepath.Join(
		"thumbnail",
		repoId,
		strconv.Itoa(size),
		trimmedPath,
	)
}

func getDirFileInfoList(username string, repoObj map[string]string, parentDir string,
	withThumbnail bool, thumbnailSize int) ([]map[string]interface{}, []map[string]interface{}, error) {

	repoId := repoObj["id"]
	var dirInfoList []map[string]interface{}
	var fileInfoList []map[string]interface{}

	klog.Infof("~~~Debug log: username=%s, repoId=%s, parent_dir=%s", username, repoId, parentDir)

	parentDirID, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, parentDir)
	if err != nil {
		return nil, nil, err
	}

	dirFileList, err := seaserv.GlobalSeafileAPI.ListDirWithPerm(repoId, parentDir, parentDirID, username, -1, -1)
	if err != nil {
		return nil, nil, err
	}

	var dirList []map[string]interface{}
	for _, dirent := range dirFileList {
		klog.Infof("~~~Debug log: dirent.mode=%s", dirent["mode"])
		isDir, err := IsDirectory(dirent["mode"])
		if err != nil {
			klog.Error(err)
			continue
		}
		if isDir {
			dirList = append(dirList, ConvertMap(dirent))
		}
	}

	for _, dirent := range dirList {
		dirInfo := map[string]interface{}{
			"type":       "dir",
			"id":         dirent["obj_id"],
			"name":       dirent["obj_name"],
			"mtime":      dirent["mtime"],
			"permission": dirent["permission"],
			"parent_dir": parentDir,
			"mode":       dirent["mode"],
		}
		dirInfoList = append(dirInfoList, dirInfo)
	}

	var fileList []map[string]interface{}
	for _, dirent := range dirFileList {
		klog.Infof("~~~Debug log: dirent.mode=%s", dirent["mode"])
		isDir, err := IsDirectory(dirent["mode"])
		if err != nil {
			klog.Error(err)
			continue
		}
		if !isDir {
			fileList = append(fileList, ConvertMap(dirent))
		}
	}

	nicknameDict := make(map[string]string)
	contactEmailDict := make(map[string]string)
	modifierSet := make(map[string]struct{})
	lockOwnerSet := make(map[string]struct{})

	for _, f := range fileList {
		modifierSet[f["modifier"].(string)] = struct{}{}
		lockOwnerSet[f["lock_owner"].(string)] = struct{}{}
	}

	for e := range modifierSet {
		if _, exists := nicknameDict[e]; !exists {
			nicknameDict[e] = seaserv.Email2Nickname(seaserv.Email2ContactEmail(e))
		}
		if _, exists := contactEmailDict[e]; !exists {
			contactEmailDict[e] = seaserv.Email2ContactEmail(e)
		}
	}

	for e := range lockOwnerSet {
		if _, exists := nicknameDict[e]; !exists {
			nicknameDict[e] = seaserv.Email2Nickname(seaserv.Email2ContactEmail(e))
		}
		if _, exists := contactEmailDict[e]; !exists {
			contactEmailDict[e] = seaserv.Email2ContactEmail(e)
		}
	}

	for _, dirent := range fileList {
		fileName := dirent["obj_name"].(string)
		filePath := path.Join(parentDir, fileName)
		fileObjID := dirent["obj_id"]
		lockedByMe := "false"
		if username == dirent["lock_owner"] {
			lockedByMe = "true"
		}

		sizeInt, err := strconv.ParseInt(dirent["size"].(string), 10, 64)
		if err != nil {
			klog.Errorf("Error parsing size flag: %v", err)
			sizeInt = 0
		}

		fileInfo := map[string]interface{}{
			"type":                     "file",
			"id":                       fileObjID,
			"name":                     fileName,
			"mtime":                    dirent["mtime"],
			"permission":               dirent["permission"],
			"parent_dir":               parentDir,
			"size":                     sizeInt,
			"modifier_email":           dirent["modifier"],
			"modifier_name":            nicknameDict[dirent["modifier"].(string)],
			"modifier_contact_email":   contactEmailDict[dirent["modifier"].(string)],
			"is_locked":                dirent["is_locked"],
			"lock_time":                dirent["lock_time"],
			"lock_owner":               dirent["lock_owner"],
			"lock_owner_name":          nicknameDict[dirent["lock_owner"].(string)],
			"lock_owner_contact_email": contactEmailDict[dirent["lock_owner"].(string)],
			"locked_by_me":             lockedByMe,
			"mode":                     dirent["mode"],
		}

		encrypted, err := strconv.ParseBool(repoObj["encrypted"])
		if err != nil {
			klog.Error(err)
			continue
		}
		if withThumbnail && !encrypted {
			fileExt := strings.ToLower(strings.TrimPrefix(path.Ext(fileName), "."))
			if fileType, exists := FILEEXT_TYPE_MAP[fileExt]; exists {
				if fileType == "image" || fileType == "xmind" ||
					(fileType == "video" && ENABLE_VIDEO_THUMBNAIL) {

					thumbnailPath := path.Join(THUMBNAIL_ROOT,
						fmt.Sprintf("%d", thumbnailSize), fileObjID.(string))
					if _, err := os.Stat(thumbnailPath); err == nil {
						src := getThumbnailSrc(repoId, thumbnailSize, filePath)
						fileInfo["encoded_thumbnail_src"] = src
					}
				}
			}
		}

		fileInfoList = append(fileInfoList, fileInfo)
	}

	sort.Slice(dirInfoList, func(i, j int) bool {
		return strings.ToLower(dirInfoList[i]["name"].(string)) < strings.ToLower(dirInfoList[j]["name"].(string))
	})
	sort.Slice(fileInfoList, func(i, j int) bool {
		return strings.ToLower(fileInfoList[i]["name"].(string)) < strings.ToLower(fileInfoList[j]["name"].(string))
	})

	return dirInfoList, fileInfoList, nil
}
