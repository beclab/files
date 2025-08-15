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

func GetUploadLink(fileParam *models.FileParam, reqFrom string, replace bool) (string, error) {
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

	username := fileParam.Owner + "@auth.local"

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil || permission != "rw" {
		return "", errors.New("permission denied")
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
	token, err := seaserv.GlobalSeafileAPI.GetFileServerAccessToken(repoId, string(objId), "upload", username, false)
	if err != nil {
		klog.Errorf("fail to get file server token %s, err=%s", string(objId), err)
		return "", err
	}
	if token == "" {
		return "", errors.New("internal server error")
	}

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

	return url, nil
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

func GetUploadedBytes(fileParam *models.FileParam, fileName string) ([]byte, error) {
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
