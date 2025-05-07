package http

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/files"
	"files/pkg/postgres"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/afero"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

type ShareablePutRequestBody struct {
	Status int `json:"status"`
}

func shareableGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	statusStr := r.URL.Query().Get("status")
	path := r.URL.Path
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

	pathInfos, err := postgres.QueryPathInfos(status, path, md5, page, limit)
	if err != nil {
		http.Error(w, "Error querying path infos", http.StatusInternalServerError)
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	return common.RenderJSON(w, r, pathInfos)
}

func shareablePutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
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

	path := r.URL.Path
	status := requestBody.Status
	ownerID, ownerName := getOwner(r)
	var pathInfo postgres.PathInfo
	if err := postgres.DBServer.Where("path = ?", path).First(&pathInfo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newPathInfo := postgres.PathInfo{
				Path:       path,
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
	if r.URL.Path == "" {
		return http.StatusNotFound, nil
	}

	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
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
	shareLinks, err := postgres.QueryShareLinks(r.URL.Path, ownerID, status, page, limit)
	if err != nil {
		http.Error(w, "Error querying path infos", http.StatusInternalServerError)
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	return common.RenderJSON(w, r, shareLinks)
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
	host := common.GetHost(r)
	ownerID, ownerName := getOwner(r)

	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
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
	pathID, err := checkPathAndConvertToPathID(r.URL.Path, ownerID)
	if err != nil {
		klog.Errorln("Error converting path to path ID:", err)
		return http.StatusForbidden, err
	}

	// Calculate expire time
	expireTime := time.Now().Add(time.Duration(requestBody.ExpireIn) * time.Millisecond)

	newShareLink := postgres.ShareLink{
		LinkURL:    host + "/share_link/" + common.Md5String(r.URL.Path+fmt.Sprint(time.Now().UnixNano())),
		PathID:     pathID,
		Path:       r.URL.Path,
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
