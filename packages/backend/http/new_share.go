package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/postgres"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ShareablePutRequestBody struct {
	Status int `json:"status"`
}

var shareableGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	//outputHeader(r)

	if !d.user.Perm.Modify || !d.Check(r.URL.Path) {
		return http.StatusForbidden, nil
	}

	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	exists, err := afero.Exists(d.user.Fs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	statusStr := r.URL.Query().Get("status")
	//path := r.URL.Query().Get("path")
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

	pathInfos, err := postgres.QueryPathInfos(status, &path, &md5, page, limit)
	if err != nil {
		http.Error(w, "Error querying path infos", http.StatusInternalServerError)
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	return renderJSON(w, r, pathInfos)
	//if err := json.NewEncoder(w).Encode(pathInfos); err != nil {
	//	http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
	//	return http.StatusInternalServerError, err
	//}
	//return http.StatusOK, nil
})

var shareablePutHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	//outputHeader(r)

	if !d.user.Perm.Modify || !d.Check(r.URL.Path) {
		return http.StatusForbidden, nil
	}

	// Only allow PUT for files.
	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	exists, err := afero.Exists(d.user.Fs, r.URL.Path)
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

	//fmt.Fprintf(w, "Received status: %d\n", requestBody.Status)

	path := r.URL.Path
	status := requestBody.Status
	ownerID, ownerName := getOwner(r)
	var pathInfo postgres.PathInfo
	if err := postgres.DBServer.Where("path = ?", path).First(&pathInfo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newPathInfo := postgres.PathInfo{
				Path:       path,
				SrcType:    "drive",
				MD5:        stringMD5(path),
				OwnerID:    ownerID,
				OwnerName:  ownerName,
				Status:     status,
				CreateTime: time.Now(),
				UpdateTime: time.Now(),
			}
			fmt.Println(newPathInfo)
			if err := postgres.DBServer.Create(&newPathInfo).Error; err != nil {
				panic(fmt.Sprintf("failed to create new PathInfo: %v", err))
			}
			fmt.Println("New record created successfully.")
		} else {
			panic(fmt.Sprintf("failed to query PathInfo: %v", err))
		}
	} else {
		postgres.DBServer.Model(&pathInfo).Update("status", status)
		fmt.Println("Record updated successfully.")
	}
	return errToStatus(err), err
})
