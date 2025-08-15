package seahub

import (
	"crypto/md5"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/models"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

var FILE_SERVER_ROOT = "/seafhttp"

func GenerateUniqueIdentifier(relativePath string) string {
	h := md5.New()
	io.WriteString(h, relativePath+time.Now().String())
	return fmt.Sprintf("%x%s", h.Sum(nil), relativePath)
}

func GetUploadLink(header http.Header, fileParam *models.FileParam, reqFrom string, replace bool) (string, error) {
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return "", err
	}
	path := uri + fileParam.Path
	klog.Infof("~~~Debug log: path=%s", path)

	repoId := fileParam.Extend
	parentDir := fileParam.Path
	if parentDir == "" {
		parentDir = "/"
	}

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Errorf("fail to get repo id %s, err=%s", repoId, err)
		return "", err
	}
	if repo == nil {
		klog.Errorf("fail to get repo id %s", repoId)
		return "", errors.New("library not found")
	}

	dirID, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, parentDir)
	if err != nil {
		klog.Errorf("fail to get dir id %s, err=%s", parentDir, err)
		return "", err
	}
	if dirID == "" {
		klog.Errorf("fail to get dir id %s", parentDir)
		return "", errors.New("folder not found")
	}

	bflName := header.Get("X-Bfl-User")
	username := bflName + "@auth.local"
	oldUsername := seaserv.GetOldUsername(bflName + "@seafile.com") // temp compatible
	useUsername := username

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil || permission != "rw" {
		permission, err = CheckFolderPermission(oldUsername, repoId, parentDir) // temp compatible
		if err != nil || permission != "rw" {
			return "", errors.New("permission denied")
		} else {
			useUsername = oldUsername
		}
	}

	quota, err := seaserv.GlobalSeafileAPI.CheckQuota(repoId, 0)
	if err != nil {
		klog.Errorf("fail to check quota %s, err=%s", repoId, err)
		return "", err
	}
	if quota < 0 {
		return "", errors.New("quota exceeded") // original status_code=443 in seahub
	}

	objId := common.ToBytes(map[string]string{"parent_dir": parentDir})
	token, err := seaserv.GlobalSeafileAPI.GetFileServerAccessToken(repoId, string(objId), "upload", useUsername, false)
	if err != nil {
		klog.Errorf("fail to get file server token %s, err=%s", string(objId), err)
		return "", err
	}
	if token == "" {
		return "", errors.New("internal server error")
	}

	klog.Infof("~~~Debug log: token=%s", token)

	var url string

	switch reqFrom {
	case "api":
		if strings.Contains(permission, "custom") {
			replace = false
		}
		url = genFileUploadURL(token, "upload-api", replace)
	case "web":
		url = genFileUploadURL(token, "upload-aj", false)
	default:
		return "", errors.New("invalid 'from' parameter")
	}

	klog.Infof("~~~Debug log: url=%s", url)
	return url, nil
}

func HandleUploadLink(w http.ResponseWriter, r *http.Request, d *common.HttpData) (int, error) {
	MigrateSeahubUserToRedis(r.Header)

	//owner, _, _, _, err := upload.GetPVC(r)
	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return http.StatusBadRequest, errors.New("bfl header missing or invalid")
	}

	p := r.URL.Query().Get("file_path")
	if p == "" {
		return http.StatusBadRequest, errors.New("missing path query parameter")
	}

	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	fileParam, err := models.CreateFileParam(owner, p)
	if err != nil {
		return http.StatusBadRequest, err
	}

	reqFrom := r.URL.Query().Get("from")
	replace := strings.ToLower(r.URL.Query().Get("replace")) == "true"

	url, err := GetUploadLink(r.Header.Clone(), fileParam, reqFrom, replace)
	if err != nil {
		return http.StatusBadRequest, err
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(url))
	return 0, nil
}

func genFileUploadURL(token, op string, replace bool) string {
	baseURL := fmt.Sprintf("%s/%s/%s", FILE_SERVER_ROOT, op, token)

	if replace {
		var builder strings.Builder
		builder.WriteString(baseURL)
		builder.WriteString("?replace=1")
		return builder.String()
	}

	return baseURL
}

func GetUploadedBytes(header http.Header, fileParam *models.FileParam, fileName string) ([]byte, error) {
	repoId := fileParam.Extend

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Errorf("fail to get repo id %s, err=%s", repoId, err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("fail to get repo id %s", repoId)
		return nil, errors.New("library not found")
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}
	path := uri + fileParam.Path
	klog.Infof("~~~Debug log: parentDir=%s", path)

	parentDir := fileParam.Path
	if parentDir == "" {
		parentDir = "/"
	}

	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, parentDir)
	if dirId == "" || err != nil {
		klog.Errorf("fail to check dir exists %s, err=%s", parentDir, err)
		return nil, errors.New("folder not found")
	}

	filePath := filepath.Join(parentDir, fileName)

	uploadedBytes, err := seaserv.GlobalSeafileAPI.GetUploadTmpFileOffset(repoId, filePath)
	if err != nil {
		klog.Errorf("Error getting upload offset: %v", err)
		return nil, err
	}

	response := map[string]interface{}{
		"uploadedBytes": uploadedBytes,
	}
	return common.ToBytes(response), nil
}

func HandleUploadedBytes(w http.ResponseWriter, r *http.Request, d *common.HttpData) (int, error) {
	MigrateSeahubUserToRedis(r.Header)

	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return http.StatusBadRequest, errors.New("bfl header missing or invalid")
	}

	p := r.URL.Query().Get("parent_dir")
	if p == "" {
		return http.StatusBadRequest, errors.New("missing parent_dir query parameter")
	}

	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	fileParam, err := models.CreateFileParam(owner, p)
	if err != nil {
		return http.StatusBadRequest, err
	}

	fileName := r.URL.Query().Get("file_name")
	if fileName == "" {
		return http.StatusBadRequest, errors.New("file_relative_path invalid")
	}

	response, err := GetUploadedBytes(r.Header, fileParam, fileName)

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", "application/json")
	return common.RenderJSON(w, r, response)
}
