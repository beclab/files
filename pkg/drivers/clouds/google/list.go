package google

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/models"
	"net/http"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// List implements base.DriverInterface.
func (g *GoogleStorage) List(fileParam *models.FileParam) (int, error) {
	var w = g.Base.Handler.ResponseWriter
	var r = g.Base.Handler.Request
	var err error

	metaStr := r.URL.Query().Get("meta")
	meta := 0
	if metaStr != "" {
		meta, err = strconv.Atoi(metaStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	src := r.URL.Path
	klog.Infoln("src Path:", src)
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	var data = &models.ListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(data)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return common.ErrToStatus(err), err
	}
	klog.Infof("Google Drive List Params: %s, meta: %d", string(jsonBody), meta)

	if meta == 1 {
		res, err := g.Service.GetFileMetaData(data)
		if err != nil {
			klog.Errorf("GetFileMetaData error: %v", err)
			return common.ErrToStatus(err), err
		}

		var fileMetadata = res.(*models.GoogleDriveResponse)
		if !fileMetadata.IsSuccess() {
			err = errors.New(fileMetadata.FailMessage())
			return common.ErrToStatus(err), err
		}
		return common.RenderJSON(w, r, fileMetadata)
	}

	res, err := g.Service.List(data)
	if err != nil {
		klog.Errorf("List error: %v", err)
		return common.ErrToStatus(err), err
	}

	listFiles := res.(*models.GoogleDriveListResponse)
	if !listFiles.IsSuccess() {
		err = errors.New(listFiles.FailMessage())
		return common.ErrToStatus(err), err
	}

	klog.Infof("Google Drive List Result, count: %d", len(listFiles.Data))

	return common.RenderJSON(w, r, listFiles)

}
