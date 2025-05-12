package http

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/constant"
	"files/pkg/drives"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/postgres"
	"files/pkg/token"
	"files/pkg/upload"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

const secretKey = "pathmd5"

type userTokenData struct {
	PathMD5 string `json:"path_md5"`
}

type ShareablePutRequestBody struct {
	Status int `json:"status"`
}

func shareableGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	originalPath := r.URL.Path
	path := r.URL.Path
	if path != "" && path != "/" {
		fileParam, _, err := UrlPrep(r, path)
		if err != nil {
			return http.StatusBadRequest, err
		}
		uri, err := fileParam.GetResourceUri()
		if err != nil {
			return http.StatusBadRequest, err
		}
		path = strings.TrimPrefix(uri+fileParam.Path, "/data")

		exists, err := afero.Exists(files.DefaultFs, path)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !exists {
			return http.StatusNotFound, nil
		}
	}

	statusStr := r.URL.Query().Get("status")
	md5 := r.URL.Query().Get("md5")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	var status *int
	if statusStr != "" {
		s, err := strconv.Atoi(statusStr)
		if err != nil {
			http.Error(w, "Invalid status parameter", http.StatusBadRequest)
			return http.StatusBadRequest, err
		}
		status = &s
	}

	var page, limit *int
	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil {
			http.Error(w, "Invalid page parameter", http.StatusBadRequest)
			return http.StatusBadRequest, err
		}
		page = &p
	}

	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return http.StatusBadRequest, err
		}
		limit = &l
	}

	pathInfos, err := postgres.QueryPathInfos(status, originalPath, md5, page, limit)
	if err != nil {
		http.Error(w, "Error querying path infos", http.StatusInternalServerError)
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	return common.RenderJSON(w, r, pathInfos)
}

func shareablePutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	originalPath := r.URL.Path
	fileParam, _, err := UrlPrep(r, "")
	if err != nil {
		return http.StatusBadRequest, err
	}
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	urlPath := strings.TrimPrefix(uri+fileParam.Path, "/data")

	exists, err := afero.Exists(files.DefaultFs, urlPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read request body", http.StatusBadRequest)
		return http.StatusBadRequest, err
	}
	defer r.Body.Close()

	var requestBody ShareablePutRequestBody
	err = json.Unmarshal(body, &requestBody)
	if err != nil {
		http.Error(w, "Invalid request body format", http.StatusBadRequest)
		return http.StatusBadRequest, err
	}

	if requestBody.Status != postgres.STATUS_PRIVATE &&
		requestBody.Status != postgres.STATUS_PUBLIC &&
		requestBody.Status != postgres.STATUS_DELETED {
		http.Error(w, "Invalid status parameter", http.StatusBadRequest)
	}
	switch requestBody.Status {
	case postgres.STATUS_PRIVATE, postgres.STATUS_PUBLIC, postgres.STATUS_DELETED:
		// Valid status, proceed with logic
	default:
		http.Error(w, "Invalid status parameter", http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}

	path := urlPath
	status := requestBody.Status
	ownerID, ownerName := getOwner(r)
	var pathInfo postgres.PathInfo
	if err := postgres.DBServer.Where("path = ?", path).First(&pathInfo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newPathInfo := postgres.PathInfo{
				Path:       originalPath, // have to save original url
				SrcType:    drives.SrcTypeDrive,
				MD5:        common.Md5String(path),
				OwnerID:    ownerID,
				OwnerName:  ownerName,
				Status:     status,
				CreateTime: time.Now(),
				UpdateTime: time.Now(),
			}
			klog.Infoln(newPathInfo)
			if err := postgres.DBServer.Create(&newPathInfo).Error; err != nil {
				panic(fmt.Sprintf("failed to create new PathInfo: %v", err))
			}
			klog.Infoln("New record created successfully.")
		} else {
			panic(fmt.Sprintf("failed to query PathInfo: %v", err))
		}
	} else {
		if pathInfo.Status != status {
			postgres.DBServer.Model(&pathInfo).Update("status", status)
			klog.Infoln("Record updated successfully.")
		} else {
			klog.Infoln("No need to update record.")
		}
	}
	return common.ErrToStatus(err), err
}

func shareLinkGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	originalPath := r.URL.Path
	urlPath := ""
	if r.URL.Path != "" {
		fileParam, _, err := UrlPrep(r, "")
		if err != nil {
			return http.StatusBadRequest, err
		}
		uri, err := fileParam.GetResourceUri()
		if err != nil {
			return http.StatusBadRequest, err
		}
		urlPath = strings.TrimPrefix(uri+fileParam.Path, "/data")

		exists, err := afero.Exists(files.DefaultFs, urlPath)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !exists {
			return http.StatusNotFound, nil
		}
	}

	statusStr := r.URL.Query().Get("status")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	var status *int
	if statusStr != "" {
		s, err := strconv.Atoi(statusStr)
		if err != nil {
			http.Error(w, "Invalid status parameter", http.StatusBadRequest)
			return http.StatusBadRequest, err
		}
		status = &s
	}

	var page, limit *int
	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil {
			http.Error(w, "Invalid page parameter", http.StatusBadRequest)
			return http.StatusBadRequest, err
		}
		page = &p
	}

	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return http.StatusBadRequest, err
		}
		limit = &l
	}

	ownerID, _ := getOwner(r)
	shareLinks, err := postgres.QueryShareLinks(originalPath, ownerID, status, page, limit)
	if err != nil {
		http.Error(w, "Error querying path infos", http.StatusInternalServerError)
		return http.StatusInternalServerError, err
	}

	result := map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"count": len(shareLinks),
			"items": shareLinks,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	return common.RenderJSON(w, r, result)
}

type ShareLinkPostRequestBody struct {
	Permission int    `json:"permission"` // 3=download, 4=upload
	ExpireIn   uint64 `json:"expire_in"`  // millisecond
	Password   string `json:"password"`
}

func checkPathAndConvertToPathID(path, ownerID string) (uint64, error) {
	pathIDMap, err := postgres.GetPathMapFromDB()
	if err != nil {
		return 0, err
	}

	if pathInfo, exists := pathIDMap[path]; exists {
		if pathInfo.OwnerID != ownerID {
			return 0, errors.New("path doesn't belong to you")
		}
		if pathInfo.Status != postgres.STATUS_PUBLIC {
			return 0, errors.New("path not public, cannot be shared")
		}
		return pathInfo.ID, nil
	} else {
		klog.Infof("Path %s not found in database\n", path)
		return 0, fmt.Errorf("path %s not found in database", path)
	}
}

func shareLinkPostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	originalPath := r.URL.Path

	host := common.GetHost(r.Header.Get("X-Bfl-User"))
	ownerID, ownerName := getOwner(r)

	fileParam, _, err := UrlPrep(r, "")
	if err != nil {
		return http.StatusBadRequest, err
	}
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	urlPath := strings.TrimPrefix(uri+fileParam.Path, "/data")

	exists, err := afero.Exists(files.DefaultFs, urlPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read request body", http.StatusBadRequest)
		return http.StatusBadRequest, err
	}
	defer r.Body.Close()

	var requestBody ShareLinkPostRequestBody
	err = json.Unmarshal(body, &requestBody)
	if err != nil {
		http.Error(w, "Invalid request body format", http.StatusBadRequest)
		return http.StatusBadRequest, err
	}

	if requestBody.Permission != postgres.PERMISSION_READONLY && requestBody.Permission != postgres.PERMISSION_UPLOADABLE {
		http.Error(w, "Invalid permission", http.StatusForbidden)
		return http.StatusForbidden, nil
	}
	if requestBody.ExpireIn == 0 {
		http.Error(w, "Invalid expire time", http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}
	if requestBody.Password == "" {
		http.Error(w, "Invalid password", http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}

	// Convert path to PathID
	pathID, err := checkPathAndConvertToPathID(urlPath, ownerID)
	if err != nil {
		klog.Errorln("Error converting path to path ID:", err)
		return http.StatusForbidden, err
	}

	// Calculate expire time
	expireTime := time.Now().Add(time.Duration(requestBody.ExpireIn) * time.Millisecond)
	pathMD5 := common.Md5String(urlPath + fmt.Sprint(time.Now().UnixNano()))
	newShareLink := postgres.ShareLink{
		LinkURL:    host + "/share_link/" + pathMD5,
		PathID:     pathID,
		Path:       originalPath,
		PathMD5:    pathMD5,
		Password:   common.Md5String(requestBody.Password),
		OwnerID:    ownerID,
		OwnerName:  ownerName,
		Permission: requestBody.Permission,
		ExpireIn:   requestBody.ExpireIn,
		ExpireTime: expireTime,
		Count:      0,
		Status:     postgres.STATUS_ACTIVE,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}

	result := postgres.DBServer.Create(&newShareLink)
	if result.Error != nil {
		return http.StatusInternalServerError, result.Error
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonData := map[string]string{
		"share_link": newShareLink.LinkURL,
	}
	return common.RenderJSON(w, r, jsonData)
}

func shareLinkDeleteHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	linkIDStr := r.URL.Query().Get("link_id")
	if linkIDStr == "" {
		return http.StatusBadRequest, nil
	}
	linkID, err := strconv.Atoi(linkIDStr)
	if err != nil {
		http.Error(w, "Invalid status parameter", http.StatusBadRequest)
		return http.StatusBadRequest, err
	}

	var shareLink postgres.ShareLink
	err = postgres.DBServer.Where("id = ?", linkID).First(&shareLink).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return http.StatusNotFound, fmt.Errorf("share link not found with ID: %d", linkID)
		} else {
			return http.StatusInternalServerError, fmt.Errorf("failed to query share link: %v", err)
		}
	}

	if shareLink.Status != postgres.STATUS_DELETED {
		shareLink.Status = postgres.STATUS_DELETED
		err = postgres.DBServer.Save(&shareLink).Error
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to update share link status: %v", err)
		}
	}

	return http.StatusOK, nil
}

func useShareLinkGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	password := r.URL.Query().Get("password")
	if password == "" {
		return http.StatusBadRequest, nil
	}

	pathMD5 := strings.Trim(r.URL.Path, "/")
	var shareLink postgres.ShareLink
	err := postgres.DBServer.Where("path_md5 = ? AND password = ?", pathMD5, common.Md5String(password)).First(&shareLink).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return http.StatusNotFound, fmt.Errorf("share link not found")
		} else {
			return http.StatusInternalServerError, fmt.Errorf("failed to query share link: %v", err)
		}
	}

	expireDuration := time.Until(shareLink.ExpireTime)
	if expireDuration <= 0 {
		return http.StatusBadRequest, fmt.Errorf("share link has already expired")
	}

	userTokenData := userTokenData{
		PathMD5: pathMD5,
	}
	data, err := json.Marshal(userTokenData)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to marshal user data: %v", err)
	}

	userToken, err := token.GenerateToken(data, secretKey, expireDuration)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to generate token: %v", err)
	}
	result := map[string]interface{}{
		"code":    0,
		"message": "success",
		"token":   userToken,
		"data": map[string]interface{}{
			"permission": shareLink.Permission,
			"expire_in":  shareLink.ExpireIn,
			"paths":      []string{shareLink.Path},
			"owner_id":   shareLink.OwnerID,
			"owner_name": shareLink.OwnerName,
		},
	}

	return common.RenderJSON(w, r, result)
}

func shareLinkDownloadHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	userData := r.Context().Value("userData")

	if data, ok := userData.(userTokenData); ok {
		fmt.Printf("User Data: %+v\n", data)
	} else {
		fmt.Println("No user data found in context")
	}

	var owner = r.Header.Get(constant.REQUEST_HEADER_OWNER)
	klog.Infof("~~~Debug log: owner=%s", owner)

	vars := mux.Vars(r)
	path := vars["path"]

	urlPath := r.URL.Path
	pathSplit := strings.Split(strings.Trim(urlPath, "/"), "/")
	pathMD5 := pathSplit[0]
	klog.Infof("~~~Debug log: md5Path=%s, path=%s", pathMD5, path)

	var shareLink postgres.ShareLink
	err := postgres.DBServer.Where("path_md5 = ?", pathMD5).First(&shareLink).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return http.StatusNotFound, fmt.Errorf("share link not found")
		} else {
			return http.StatusInternalServerError, fmt.Errorf("failed to query share link: %v", err)
		}
	}

	expireDuration := time.Until(shareLink.ExpireTime)
	if expireDuration <= 0 {
		return http.StatusBadRequest, fmt.Errorf("share link has already expired")
	}

	originalPath := shareLink.Path
	fullPath := filepath.Join(originalPath, path)
	fileParam, handler, err := UrlPrep(r, fullPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	klog.Infof("~~~Debug log: originalPath=%s", originalPath)

	return handler.RawHandler(fileParam)(w, r, d)
}

func shareLinkUploadLinkHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	userData := r.Context().Value("userData")

	if data, ok := userData.(userTokenData); ok {
		fmt.Printf("User Data: %+v\n", data)
	} else {
		fmt.Println("No user data found in context")
	}

	urlPath := r.URL.Path
	pathSplit := strings.Split(strings.Trim(urlPath, "/"), "/")
	pathMD5 := pathSplit[0]
	klog.Infof("~~~Debug log: md5Path=%s", pathMD5)

	var shareLink postgres.ShareLink
	err := postgres.DBServer.Where("path_md5 = ?", pathMD5).First(&shareLink).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return http.StatusNotFound, fmt.Errorf("share link not found")
		} else {
			return http.StatusInternalServerError, fmt.Errorf("failed to query share link: %v", err)
		}
	}

	expireDuration := time.Until(shareLink.ExpireTime)
	if expireDuration <= 0 {
		return http.StatusBadRequest, fmt.Errorf("share link has already expired")
	}

	originalPath := shareLink.Path

	owner, _, _, uploadsDir, err := upload.GetPVC(r)
	if err != nil {
		return http.StatusBadRequest, errors.New("bfl header missing or invalid")
	}
	klog.Infof("~~~Debug log: owner=%s, uploadsDir=%s", owner, uploadsDir)

	fileParam, err := models.CreateFileParam(owner, originalPath)
	if err != nil {
		return http.StatusBadRequest, err
	}
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	path := uri + fileParam.Path
	klog.Infof("~~~Debug log: path=%s", path)

	// change temp file location
	extracted := upload.ExtractPart(path)
	if extracted != "" {
		uploadsDir = upload.ExternalPathPrefix + extracted + "/.uploadstemp"
	}

	if !upload.PathExists(uploadsDir) {
		if err := os.MkdirAll(uploadsDir, os.ModePerm); err != nil {
			klog.Warning("err:", err)
			return http.StatusInternalServerError, err
		}
	}

	if !upload.CheckDirExist(path) {
		if err := fileutils.MkdirAllWithChown(nil, path, os.ModePerm); err != nil {
			klog.Error("err:", err)
			return http.StatusInternalServerError, err
		}
	}

	uploadID := upload.MakeUid(path)
	uploadLink := fmt.Sprintf("/share_link/%s/upload/upload_link/%s", pathMD5, uploadID)

	w.Write([]byte(uploadLink))
	return 0, nil
}

func shareLinkUploadedBytesHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	userData := r.Context().Value("userData")

	if data, ok := userData.(userTokenData); ok {
		fmt.Printf("User Data: %+v\n", data)
	} else {
		fmt.Println("No user data found in context")
	}

	urlPath := r.URL.Path
	pathSplit := strings.Split(strings.Trim(urlPath, "/"), "/")
	pathMD5 := pathSplit[0]
	klog.Infof("~~~Debug log: md5Path=%s", pathMD5)

	var shareLink postgres.ShareLink
	err := postgres.DBServer.Where("path_md5 = ?", pathMD5).First(&shareLink).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return http.StatusNotFound, fmt.Errorf("share link not found")
		} else {
			return http.StatusInternalServerError, fmt.Errorf("failed to query share link: %v", err)
		}
	}

	expireDuration := time.Until(shareLink.ExpireTime)
	if expireDuration <= 0 {
		return http.StatusBadRequest, fmt.Errorf("share link has already expired")
	}

	originalPath := shareLink.Path

	owner, _, _, uploadsDir, err := upload.GetPVC(r)
	if err != nil {
		return http.StatusBadRequest, errors.New("bfl header missing or invalid")
	}

	fileParam, err := models.CreateFileParam(owner, originalPath)
	if err != nil {
		return http.StatusBadRequest, err
	}
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	parentDir := uri + fileParam.Path
	klog.Infof("~~~Debug log: parentDir=%s", parentDir)

	if !upload.CheckDirExist(parentDir) {
		klog.Warningf("Storage path %s is not exist or is not a dir", parentDir)
		return http.StatusBadRequest, errors.New("storage path is not exist or is not a dir")
	}

	fileName := r.URL.Query().Get("file_name")
	if fileName == "" {
		return http.StatusBadRequest, errors.New("file_relative_path invalid")
	}

	responseData := make(map[string]interface{})
	responseData["uploadedBytes"] = 0

	extracted := upload.ExtractPart(parentDir)
	if extracted != "" {
		uploadsDir = upload.ExternalPathPrefix + extracted + "/.uploadstemp"
	}

	if !upload.PathExists(uploadsDir) {
		return common.RenderJSON(w, r, responseData)
	}
	klog.Infof("r:%+v", r)

	fullPath := filepath.Join(parentDir, fileName)
	dirPath := filepath.Dir(fullPath)

	if !upload.CheckDirExist(dirPath) {
		return common.RenderJSON(w, r, responseData)
	}

	if strings.HasSuffix(fileName, "/") {
		return http.StatusBadRequest, errors.New(fmt.Sprintf("full path %s is a dir", fullPath))
	}

	innerIdentifier := upload.MakeUid(fullPath)
	tmpName := innerIdentifier
	upload.UploadsFiles[innerIdentifier] = filepath.Join(uploadsDir, tmpName)

	exist, info := upload.FileInfoManager.ExistFileInfo(innerIdentifier)

	if !exist {
		fileInfo := upload.FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
		}

		if err := upload.FileInfoManager.AddFileInfo(innerIdentifier, fileInfo); err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return http.StatusInternalServerError, errors.New("Error save file info")
		}

		klog.Infof("innerIdentifier:%s, fileInfo:%+v", innerIdentifier, fileInfo)
		info = fileInfo
		exist = true
	}

	fileExist, fileLen := upload.FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
	klog.Info("***exist: ", exist, " , fileExist: ", fileExist, " , fileLen: ", fileLen)

	if exist {
		if fileExist {
			if info.Offset != fileLen {
				info.Offset = fileLen
				upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
			}
			klog.Infof("innerIdentifier:%s, info.Offset:%d", innerIdentifier, info.Offset)
			responseData["uploadedBytes"] = info.Offset
		} else if info.Offset == 0 {
			klog.Warningf("innerIdentifier:%s, info.Offset:%d", innerIdentifier, info.Offset)
		} else {
			upload.FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)
		}
	}

	return common.RenderJSON(w, r, responseData)
}

func shareLinkUploadChunksHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	startTime := time.Now()
	klog.Infof("Function UploadChunks started at: %s\n", startTime)
	defer func() {
		endTime := time.Now()
		klog.Infof("Function UploadChunks ended at: %s\n", endTime)
	}()

	userData := r.Context().Value("userData")

	if data, ok := userData.(userTokenData); ok {
		fmt.Printf("User Data: %+v\n", data)
	} else {
		fmt.Println("No user data found in context")
	}

	urlPath := r.URL.Path
	pathSplit := strings.Split(strings.Trim(urlPath, "/"), "/")
	pathMD5 := pathSplit[0]
	klog.Infof("~~~Debug log: md5Path=%s", pathMD5)

	var shareLink postgres.ShareLink
	err := postgres.DBServer.Where("path_md5 = ?", pathMD5).First(&shareLink).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return http.StatusNotFound, fmt.Errorf("share link not found")
		} else {
			return http.StatusInternalServerError, fmt.Errorf("failed to query share link: %v", err)
		}
	}

	expireDuration := time.Until(shareLink.ExpireTime)
	if expireDuration <= 0 {
		return http.StatusBadRequest, fmt.Errorf("share link has already expired")
	}

	originalPath := shareLink.Path
	klog.Infof("~~~Debug log: originalPath=%s", originalPath)

	vars := mux.Vars(r)
	uploadID := vars["uid"]

	owner, _, _, uploadsDir, err := upload.GetPVC(r)
	if err != nil {
		return http.StatusBadRequest, errors.New("bfl header missing or invalid")
	}

	responseData := map[string]interface{}{
		"success": true,
	}

	var resumableInfo upload.ResumableInfo
	if err := ParseFormData(r, &resumableInfo); err != nil {
		klog.Warningf("uploadID:%s, err:%v", uploadID, err)
		return http.StatusBadRequest, errors.New("param invalid")
	}

	p := resumableInfo.ParentDir
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	fileParam, err := models.CreateFileParam(owner, p)
	if err != nil {
		return http.StatusBadRequest, err
	}
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	parentDir := uri + fileParam.Path
	klog.Infof("~~~Debug log: parentDir=%s", parentDir)

	extracted := upload.ExtractPart(parentDir)
	if extracted != "" {
		uploadsDir = upload.ExternalPathPrefix + extracted + "/.uploadstemp"
	}
	if !upload.PathExists(uploadsDir) {
		if err := os.MkdirAll(uploadsDir, os.ModePerm); err != nil {
			klog.Warningf("uploadID:%s, err:%v", uploadID, err)
			return http.StatusInternalServerError, err
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		klog.Warningf("uploadID:%s, Failed to parse file: %v\n", uploadID, err)
		return http.StatusBadRequest, errors.New("param invalid")
	}
	defer file.Close()

	resumableInfo.File = header

	fullPath := filepath.Join(parentDir, resumableInfo.ResumableRelativePath)
	innerIdentifier := upload.MakeUid(fullPath)
	tmpName := innerIdentifier
	upload.UploadsFiles[innerIdentifier] = filepath.Join(uploadsDir, tmpName)

	exist, info := upload.FileInfoManager.ExistFileInfo(innerIdentifier)
	if !exist {
		klog.Warningf("innerIdentifier %s not exist", innerIdentifier)
	}

	if !exist || innerIdentifier != info.ID {
		upload.RemoveTempFileAndInfoFile(tmpName, uploadsDir)
		if info.Offset != 0 {
			info.Offset = 0
			upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
		}

		if !upload.CheckDirExist(parentDir) {
			klog.Warningf("Parent dir %s is not exist or is not a dir", resumableInfo.ParentDir)
			return http.StatusBadRequest, errors.New("parent dir is not exist or is not a dir")
		}

		dirPath := filepath.Dir(fullPath)
		if !upload.CheckDirExist(dirPath) {
			if err := fileutils.MkdirAllWithChown(nil, dirPath, os.ModePerm); err != nil {
				klog.Error("err:", err)
				return http.StatusInternalServerError, err
			}
		}

		if resumableInfo.ResumableRelativePath == "" {
			return http.StatusBadRequest, errors.New("file_relative_path invalid")
		}

		if strings.HasSuffix(resumableInfo.ResumableRelativePath, "/") {
			klog.Warningf("full path %s is a dir", fullPath)
			return http.StatusBadRequest, errors.New(fmt.Sprintf("full path %s is a dir", fullPath))
		}

		if !upload.CheckType(resumableInfo.ResumableType) {
			klog.Warningf("unsupported filetype:%s", resumableInfo.ResumableType)
			return http.StatusBadRequest, errors.New(fmt.Sprintf("unsupported filetype:%s", resumableInfo.ResumableType))
		}

		if !upload.CheckSize(resumableInfo.ResumableTotalSize) {
			if info.Offset == info.FileSize {
				klog.Warningf("All file chunks have been uploaded, skip upload")
				finishData := []map[string]interface{}{
					{
						"name": resumableInfo.ResumableFilename,
						"id":   upload.MakeUid(info.FullPath),
						"size": info.FileSize,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(finishData)
			}
			klog.Warningf("Unsupported file size uploadSize:%d", resumableInfo.ResumableTotalSize)
			return http.StatusBadRequest, errors.New("unsupported file size")
		}

		info.FileSize = resumableInfo.ResumableTotalSize
		upload.FileInfoManager.UpdateInfo(innerIdentifier, info)

		oExist, oInfo := upload.FileInfoManager.ExistFileInfo(innerIdentifier)
		oFileExist, oFileLen := upload.FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
		if oExist {
			if oFileExist {
				if oInfo.Offset != oFileLen {
					oInfo.Offset = oFileLen
					upload.FileInfoManager.UpdateInfo(innerIdentifier, oInfo)
				}
			} else if oInfo.Offset == 0 {
			} else {
				upload.FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)
			}
		}

		fileInfo := upload.FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
			FileMetaData: upload.FileMetaData{
				FileRelativePath: resumableInfo.ResumableRelativePath,
				FileType:         resumableInfo.ResumableType,
				FileSize:         resumableInfo.ResumableTotalSize,
				StoragePath:      parentDir,
				FullPath:         fullPath,
			},
		}

		if oFileExist {
			fileInfo.Offset = oFileLen
		}

		if err = upload.FileInfoManager.AddFileInfo(innerIdentifier, fileInfo); err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return http.StatusInternalServerError, errors.New("error save file info")
		}

		info = fileInfo
	} else {
		fileInfo := upload.FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
			FileMetaData: upload.FileMetaData{
				FileRelativePath: resumableInfo.ResumableRelativePath,
				FileType:         resumableInfo.ResumableType,
				FileSize:         resumableInfo.ResumableTotalSize,
				StoragePath:      parentDir,
				FullPath:         fullPath,
			},
		}

		upload.FileInfoManager.UpdateInfo(innerIdentifier, fileInfo)
		if err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return http.StatusInternalServerError, errors.New("error save file info")
		}

		info = fileInfo
	}

	fileExist, fileLen := upload.FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
	if fileExist {
		if info.Offset != fileLen {
			info.Offset = fileLen
			upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
		}
	}

	fileHeader := resumableInfo.File
	size := fileHeader.Size

	ranges := r.Header.Get("Content-Range")
	var offsetStart, offsetEnd int64
	var parsed bool
	if ranges != "" {
		offsetStart, offsetEnd, parsed = upload.ParseContentRange(ranges)
		if !parsed {
			return http.StatusBadRequest, errors.New(fmt.Sprintf("Invalid content range:%s", ranges))
		}
	}

	var newFile bool = false
	if info.Offset != offsetStart && offsetStart == 0 {
		newFile = true
		info.Offset = offsetStart
		upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
	}

	if !upload.CheckSize(size) || offsetEnd >= info.FileSize {
		if info.Offset == info.FileSize {
			finishData := []map[string]interface{}{
				{
					"name": resumableInfo.ResumableFilename,
					"id":   upload.MakeUid(info.FullPath),
					"size": info.FileSize,
				},
			}
			return common.RenderJSON(w, r, finishData)
		}
		return http.StatusBadRequest, errors.New("Unsupported file size")
	}

	const maxRetries = 100
	for retry := 0; retry < maxRetries; retry++ {
		if info.Offset-offsetEnd > 0 {
			return common.RenderJSON(w, r, responseData)
		}

		if info.Offset == offsetStart || (info.Offset-offsetEnd < 0 && info.Offset-offsetStart > 0) {
			if resumableInfo.MD5 != "" {
				md5, err := upload.CalculateMD5(fileHeader)
				if err != nil {
					return http.StatusInternalServerError, err
				}
				if md5 != resumableInfo.MD5 {
					msg := fmt.Sprintf("Invalid MD5, accepted %s, calculated %s", resumableInfo.MD5, md5)
					return http.StatusBadRequest, errors.New(msg)
				}
			}

			fileSize, err := upload.SaveFile(fileHeader, filepath.Join(uploadsDir, tmpName), newFile, offsetStart)
			if err != nil {
				klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
				return http.StatusInternalServerError, err
			}
			info.Offset = fileSize
			upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
			break
		}

		time.Sleep(1000 * time.Millisecond)

		if retry < maxRetries-1 {
			exist, info = upload.FileInfoManager.ExistFileInfo(innerIdentifier)
			if !exist {
				return http.StatusBadRequest, errors.New("invalid innerIdentifier")
			}
			continue
		}

		return http.StatusInternalServerError, errors.New("Failed to match offset after multiple retries")
	}

	if err = upload.UpdateFileInfo(info, uploadsDir); err != nil {
		klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
		return http.StatusInternalServerError, err
	}

	klog.Info("offsetStart:", offsetStart, ", offsetEnd:", offsetEnd, ", info.Offset:", info.Offset, ", info.FileSize:", info.FileSize)
	if offsetEnd == info.FileSize-1 {
		if err = upload.MoveFileByInfo(info, uploadsDir); err != nil {
			klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
			return http.StatusInternalServerError, err
		}
		upload.FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)

		finishData := []map[string]interface{}{
			{
				"name": resumableInfo.ResumableFilename,
				"id":   upload.MakeUid(info.FullPath),
				"size": info.FileSize,
			},
		}
		return common.RenderJSON(w, r, finishData)
	}
	return common.RenderJSON(w, r, responseData)
}
