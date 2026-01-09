package seahub

import (
	"bytes"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/models"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"k8s.io/klog/v2"
)

var MAX_UPLOAD_FILE_NAME_LEN = 255

func HandleFileOperation(owner, repoId, pathParam, destName, operation string) ([]byte, error) {
	pathParam = path.Clean(path.Clean(strings.ReplaceAll(pathParam, "\\", "/")))
	if pathParam == "" || pathParam[0] != '/' {
		klog.Errorf("invalid path param: %s", pathParam)
		return nil, errors.New("p invalid")
	}

	if pathParam == "/" {
		klog.Errorf("invalid path param: %s", pathParam)
		return nil, errors.New("Can not operate root dir.")
	}

	operation = strings.ToLower(operation)
	validOperations := map[string]bool{
		"create": true, "rename": true, "move": true,
		"copy": true, "revert": true,
	}
	if !validOperations[operation] {
		return nil, errors.New("invalid operation")
	}

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("repo %s not found", repoId)
		return nil, errors.New("repo not found")
	}

	// we only use rename now
	switch operation {
	case "create":
		return handleCreate(repoId, pathParam, owner)
	case "rename":
		return handleRename(repoId, pathParam, owner, destName)
	case "move":
		return nil, errors.New("operation not supported yet")
	case "copy":
		return nil, errors.New("operation not supported yet")
	case "revert":
		return nil, errors.New("operation not supported yet")
	default:
		return nil, errors.New("unknown operation")
	}
}

func handleCreate(repoId, pathParam, owner string) ([]byte, error) {
	username := owner + "@auth.local"

	pathParam = strings.TrimRight(pathParam, "/")
	parentDir := path.Dir(pathParam)
	filename := path.Base(pathParam)

	// resource check
	parentDirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, parentDir)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	if parentDirId == "" {
		return nil, fmt.Errorf("Folder %s not found.", parentDir)
	}

	// permission check
	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil {
		return nil, err
	}
	if permission != "rw" {
		return nil, errors.New("permission denied")
	}

	validName, err := isValidDirentName(filename)
	if err != nil {
		return nil, err
	}
	if !validName {
		return nil, errors.New("name invalid")
	}

	filename = CheckFilenameWithRename(repoId, parentDir, filename)

	_, err = seaserv.GlobalSeafileAPI.PostEmptyFile(repoId, parentDir, filename, username)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	fileInfo := GetFileInfo(repoId, path.Join(parentDir, filename))
	return common.ToBytes(fileInfo), nil
}

func handleRename(repoId, pathParam, owner, newName string) ([]byte, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, errors.New("newname invalid")
	}
	validName, err := isValidDirentName(newName)
	if err != nil {
		return nil, err
	}
	if !validName {
		return nil, errors.New("name invalid")
	}

	if len(newName) > MAX_UPLOAD_FILE_NAME_LEN {
		klog.Errorf("newname is too long.")
		return nil, errors.New("newname is too long.")
	}

	username := owner + "@auth.local"

	pathParam = strings.TrimRight(pathParam, "/")
	parentDir := path.Dir(pathParam)
	oldName := path.Base(pathParam)

	if oldName == newName {
		klog.Errorf("The new name is the same to the old")
		return nil, errors.New("new name is the same to the old")
	}

	fileId, err := seaserv.GlobalSeafileAPI.GetFileIdByPath(repoId, pathParam)
	if err != nil {
		return nil, errors.New("internal server error")
	}
	if fileId == "" {
		klog.Errorf("file %s not found", pathParam)
		return nil, errors.New("file not found")
	}

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil {
		return nil, err
	}
	if permission != "rw" {
		return nil, errors.New("permission denied")
	}

	newName = CheckFilenameWithRename(repoId, parentDir, newName)
	resultCode, err := seaserv.GlobalSeafileAPI.RenameFile(repoId, parentDir, oldName, newName, username)
	if err != nil {
		return nil, err
	}
	if resultCode != 0 {
		klog.Errorf("Failed to rename: result_code: %d, err: %v", resultCode, err)
		return nil, errors.New("failed to rename")
	}

	fileInfo := GetFileInfo(repoId, path.Join(parentDir, newName))
	return common.ToBytes(fileInfo), nil
}

func GetFileInfo(repoId, filePath string) map[string]interface{} {
	fileInfo := make(map[string]interface{})
	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Error(err)
		return nil
	}
	if repo == nil {
		klog.Errorf("repo %s not found", repoId)
		return nil
	}

	fileObj, err := seaserv.GlobalSeafileAPI.GetDirentByPath(repoId, filePath)
	if err != nil {
		klog.Error(err)
		return nil
	}

	var fileName string
	var fileSize int64

	if fileObj != nil {
		fileName = fileObj["obj_name"]
		fileSize, err = strconv.ParseInt(fileObj["size"], 10, 64)
		if err != nil {
			klog.Errorf("Error parsing repo size: %v", err)
			fileSize = 0
		}
	} else {
		fileName = path.Base(strings.TrimRight(filePath, "/"))
	}

	isLocked := false

	fileInfo["type"] = "file"
	fileInfo["repo_id"] = repoId
	fileInfo["parent_dir"] = path.Dir(filePath)
	fileInfo["obj_name"] = fileName
	fileInfo["obj_id"] = ""
	fileInfo["size"] = fileSize
	fileInfo["mtime"] = ""
	fileInfo["is_locked"] = isLocked

	if fileObj != nil {
		fileInfo["obj_id"] = fileObj["obj_id"]
		fileInfo["mtime"] = TimestampToISO(fileObj["mtime"])
	}

	return fileInfo
}

func HandleUpdateLink(fileParam *models.FileParam, from string) ([]byte, error) {
	repoId := fileParam.Extend
	parentDir := filepath.Dir(fileParam.Path)
	if parentDir == "" {
		parentDir = "/"
	}

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("repo %s not found", repoId)
		return nil, errors.New("repo not found")
	}

	dirId, err := seaserv.GlobalSeafileAPI.GetDirIdByPath(repoId, parentDir)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if dirId == "" {
		klog.Errorf("dir %s not exist", parentDir)
		return nil, errors.New(fmt.Sprintf("dir %s not exist", parentDir))
	}

	username := fileParam.Owner + "@auth.local"

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil {
		return nil, err
	}
	if permission != "rw" {
		return nil, errors.New("permission denied")
	}

	quota, err := seaserv.GlobalSeafileAPI.CheckQuota(repoId, 0)
	if err != nil {
		klog.Errorf("fail to check quota %s, err=%s", repoId, err)
		return nil, err
	}
	if quota < 0 {
		return nil, errors.New("quota exceeded") // original status_code=443 in seahub
	}

	token, err := seaserv.GlobalSeafileAPI.GetFileServerAccessToken(repoId, "dummy", "update", username, false)
	if err != nil {
		klog.Errorf("fail to get file server token err=%s", err)
		return nil, err
	}
	if token == "" {
		return nil, errors.New("internal server error")
	}

	var updateUrl string

	switch from {
	case "api":
		updateUrl = genFileUploadURL(token, "update-api", false)
	case "web":
		updateUrl = genFileUploadURL(token, "update-aj", false)
	default:
		return nil, errors.New("invalid 'from' parameter")
	}

	return []byte(updateUrl), nil
}

const (
	FILE_PREVIEW_MAX_SIZE   = 30 * 1024 * 1024
	OFFICE_PREVIEW_MAX_SIZE = 2 * 1024 * 1024
	HIGHLIGHT_KEYWORD       = false
	ENABLE_FILE_COMMENT     = true
	ENABLE_WATERMARK        = false
)

var FILE_ENCODING_TRY_LIST = []string{"utf-8", "gbk", "iso-8859-1"}

func ViewLibFile(fileParam *models.FileParam, op string) ([]byte, error) {
	repoId := fileParam.Extend
	filePath := fileParam.Path

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("repo %s not found", repoId)
		return nil, errors.New("repo not found")
	}

	filePath = filepath.Clean(filePath)
	fileId, err := seaserv.GlobalSeafileAPI.GetFileIdByPath(repoId, filePath)
	if err != nil {
		return nil, errors.New("internal server error")
	}
	if fileId == "" {
		klog.Errorf("file %s not found", filePath)
		return nil, errors.New("file not found")
	}

	username := fileParam.Owner + "@auth.local"

	parentDir := filepath.Dir(filePath)

	permission, err := CheckFolderPermission(username, repoId, parentDir)
	if err != nil {
		return nil, err
	}
	if permission != "rw" {
		return nil, errors.New("permission denied")
	}

	if op == "dl" {
		ret, err := handleFileDownload(repo, fileId, filePath, username, "download")
		if err != nil {
			return nil, err
		}
		return ret, nil
	} else if op == "raw" {
		ret, err := handleFileDownload(repo, fileId, filePath, username, "view")
		if err != nil {
			return nil, err
		}
		return ret, nil
	}

	// "dict" below
	returnDict := buildBaseResponse(repo, fileId, filePath, username, permission)

	getContributorInfo(repo, filePath, returnDict)

	handleFileType(repo, filePath, fileId, username, returnDict)

	return common.ToBytes(returnDict), nil
}

func GenFileGetURL(token, filename string) string {
	encodedFilename := url.PathEscape(filename)
	return fmt.Sprintf("%s/files/%s/%s", FILE_SERVER_ROOT, token, encodedFilename)
}

func handleFileDownload(repo map[string]string, fileId, filePath, username, operation string) ([]byte, error) {
	token, err := seaserv.GlobalSeafileAPI.GetFileServerAccessToken(repo["id"], fileId, operation, username, false)
	if err != nil {
		klog.Errorf("fail to get file server token err=%s", err)
		return nil, err
	}
	if token == "" {
		klog.Errorf("fail to get file server token err=%s", err)
		return nil, errors.New("internal server error")
	}

	dlURL := GenFileGetURL(token, filepath.Base(filePath))
	return []byte(dlURL), nil
}

func buildBaseResponse(repo map[string]string, fileId, filePath, username, permission string) map[string]interface{} {
	isOwner, err := seaserv.GlobalSeafileAPI.IsRepoOwner(username, repo["id"])
	if err != nil {
		klog.Error(err)
		return nil
	}
	repoMap := make(map[string]interface{})
	for k, v := range repo {
		repoMap[k] = v
		if k == "last_modify" || k == "size" {
			vInt64, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				klog.Error(err)
				continue
			}
			repoMap[k] = vInt64
		} else if k == "version" {
			vInt, err := strconv.Atoi(v)
			if err != nil {
				klog.Error(err)
				continue
			}
			repoMap[k] = vInt
		}
	}
	return map[string]interface{}{
		"repo":                repoMap,
		"file_id":             fileId,
		"path":                filePath,
		"filename":            filepath.Base(filePath),
		"file_perm":           permission,
		"highlight_keyword":   HIGHLIGHT_KEYWORD,
		"enable_file_comment": ENABLE_FILE_COMMENT,
		"enable_watermark":    ENABLE_WATERMARK,
		"can_download_file":   true,
		"can_share_file":      true,
		"last_commit_id":      repo["head_commit_id"],
		"is_repo_owner":       isOwner,
		"parent_dir":          filepath.Dir(filePath),
	}
}

func getContributorInfo(repo map[string]string, filePath string, returnDict map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			returnDict["latest_contributor"] = nil
			returnDict["last_modified"] = 0
		}
	}()

	realPath := repo["origin_path"] + filePath
	dirent, err := seaserv.GlobalSeafileAPI.GetDirentByPath(repo["store_id"], realPath)
	if err != nil {
		klog.Error(err)
		returnDict["latest_contributor"] = nil
		returnDict["last_modified"] = 0
	}
	if dirent != nil {
		mtimeInt64, err := strconv.ParseInt(dirent["mtime"], 10, 64)
		if err != nil {
			klog.Error(err)
			mtimeInt64 = 0
		}
		returnDict["latest_contributor"] = dirent["modifier"]
		returnDict["last_modified"] = mtimeInt64
	} else {
		returnDict["latest_contributor"] = nil
		returnDict["last_modified"] = 0
	}
}

func handleFileType(repo map[string]string, filePath, fileId, username string, returnDict map[string]interface{}) {
	parentDir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)
	fileType, fileExt := getFileTypeAndExt(filename)
	returnDict["fileext"] = fileExt
	returnDict["filetype"] = FileTypeName(fileType)

	useOnetime := true
	if fileType == VIDEO || fileType == AUDIO {
		useOnetime = false
	}

	token, err := seaserv.GlobalSeafileAPI.GetFileServerAccessToken(repo["id"], fileId, "view", username, useOnetime)
	if err != nil {
		klog.Error(err)
	}
	if token != "" {
		returnDict["raw_path"] = GenFileGetURL(token, filename) // for all type
	}

	version, err := strconv.Atoi(repo["version"])
	if err != nil {
		klog.Error(err)
		return
	}

	fileSize, err := seaserv.GlobalSeafileAPI.GetFileSize(repo["store_id"], version, fileId)
	if err != nil {
		klog.Error(err)
		return
	}

	switch fileType {
	case TEXT, MARKDOWN:
		handleTextFile(fileSize, returnDict)
	case VIDEO, AUDIO, PDF, SVG:
		handleMediaFile(fileType, returnDict)
	case IMAGE:
		handleImageFile(repo["id"], parentDir, filename, fileSize, returnDict)
	case DOCUMENT, SPREADSHEET:
		handleOfficeFile(fileSize, returnDict)
	default:
		returnDict["err"] = "File preview unsupported"
	}
}

func handleTextFile(fileSize int64, returnDict map[string]interface{}) {
	if fileSize > FILE_PREVIEW_MAX_SIZE {
		returnDict["err"] = fmt.Sprintf("File size surpasses %s", FileSizeFormat(FILE_PREVIEW_MAX_SIZE))
		return
	}

	errMsg, content, encoding := GetFileContent(TEXT, returnDict["raw_path"].(string), "auto")
	if errMsg != "" {
		returnDict["err"] = errMsg
		return
	}

	canEditFile := returnDict["file_perm"].(string) == "rw"

	returnDict["file_content"] = content
	returnDict["file_enc"] = encoding
	returnDict["can_edit_file"] = canEditFile
}

func GetFileContent(fileType FileType, rawPath, fileEnc string) (errorMsg string, content string, encoding string) {
	if IsTextualFile(fileType) {
		return repoFileGet(rawPath, fileEnc)
	}
	return "", "", ""
}

func IsTextualFile(fileType FileType) bool {
	return fileType == TEXT || fileType == MARKDOWN
}

func repoFileGet(rawPath, fileEnc string) (errorMsg string, content string, encoding string) {
	resp, err := http.Get("http://127.0.0.1:80/" + rawPath)
	if err != nil {
		klog.Errorf("HTTPError: failed to open file online: %v", err)
		return "HTTPError: failed to open file online", "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		klog.Errorf("HTTPError: status code %d", resp.StatusCode)
		return "HTTPError: failed to open file online", "", ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		klog.Errorf("Error reading response body: %v", err)
		return "URLError: failed to open file online", "", ""
	}

	if fileEnc != "auto" {
		return decodeWithEncoding(body, fileEnc)
	}
	return autoDetectEncoding(body)
}

func decodeWithEncoding(content []byte, enc string) (errorMsg string, decoded string, encoding string) {
	decoder, err := getDecoder(enc)
	if err != nil {
		return "The encoding you chose is not proper.", "", enc
	}

	result, err := decodeContent(content, decoder)
	if err != nil {
		return "The encoding you chose is not proper.", "", enc
	}
	return "", result, enc
}

func autoDetectEncoding(content []byte) (errorMsg string, contentStr string, encoding string) {
	for _, enc := range FILE_ENCODING_TRY_LIST {
		if decoder, err := getDecoder(enc); err == nil {
			if result, err := decodeContent(content, decoder); err == nil {
				return "", result, enc
			}
		}
	}

	detector := chardet.NewTextDetector()
	result, err := detector.DetectBest(content)
	if err != nil {
		klog.Errorf("Encoding detection failed: %v", err)
		return "Unknown file encoding", "", ""
	}

	detectedEncoding := strings.ToLower(result.Charset)
	decoder, err := getDecoder(detectedEncoding)
	if err != nil {
		return "Unknown file encoding", "", ""
	}

	decoded, err := decodeContent(content, decoder)
	if err != nil {
		if decoded, err = decodeContent(content, unicode.UTF8); err == nil {
			return "", decoded, "utf-8"
		}
		return "Unknown file encoding", "", ""
	}

	return "", decoded, detectedEncoding
}

func getDecoder(encodingName string) (encoding.Encoding, error) {
	switch strings.ToLower(encodingName) {
	case "utf-8":
		return unicode.UTF8, nil
	case "gbk":
		return simplifiedchinese.GBK, nil
	case "iso-8859-1":
		return charmap.ISO8859_1, nil
	default:
		return nil, fmt.Errorf("unsupported encoding")
	}
}

func decodeContent(content []byte, decoder encoding.Encoding) (string, error) {
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF}) // UTF-8
	content = bytes.TrimPrefix(content, []byte{0xFE, 0xFF})       // UTF-16BE
	content = bytes.TrimPrefix(content, []byte{0xFF, 0xFE})       // UTF-16LE

	reader := transform.NewReader(bytes.NewReader(content), decoder.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func handleMediaFile(fileType FileType, returnDict map[string]interface{}) {
	if fileType == VIDEO {
		returnDict["enable_video_thumbnail"] = ENABLE_VIDEO_THUMBNAIL
	}
}

func handleImageFile(repoId, parentDir, filename string, fileSize int64, returnDict map[string]interface{}) {
	if fileSize > FILE_PREVIEW_MAX_SIZE {
		returnDict["err"] = fmt.Sprintf("File size surpasses %s", FileSizeFormat(FILE_PREVIEW_MAX_SIZE))
		return
	}

	dirs, err := seaserv.GlobalSeafileAPI.ListDirByPath(repoId, parentDir, -1, -1)
	if err != nil {
		return
	}

	var imgList []string
	for _, dirent := range dirs {
		isDir, err := IsDirectory(dirent["mode"])
		if err != nil {
			klog.Error(err)
			return
		}
		if isDir {
			if flType, _ := getFileTypeAndExt(dirent["name"]); flType == IMAGE {
				imgList = append(imgList, dirent["name"])
			}
		}
	}

	if len(imgList) > 1 {
		sort.Slice(imgList, func(i, j int) bool {
			return strings.ToLower(imgList[i]) < strings.ToLower(imgList[j])
		})

		curIndex := indexOf(imgList, filename)
		if curIndex > 0 {
			returnDict["img_prev"] = path.Join(parentDir, imgList[curIndex-1])
		}
		if curIndex < len(imgList)-1 {
			returnDict["img_next"] = path.Join(parentDir, imgList[curIndex+1])
		}
	}
	return
}

func handleOfficeFile(fileSize int64, returnDict map[string]interface{}) {
	if fileSize > OFFICE_PREVIEW_MAX_SIZE {
		returnDict["err"] = fmt.Sprintf("File size surpasses %s", FileSizeFormat(OFFICE_PREVIEW_MAX_SIZE))
		return
	}
	return
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
