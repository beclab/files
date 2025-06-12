package base

import (
	"files/pkg/common"
	"files/pkg/models"
	"net/http"
	"strconv"

	"k8s.io/klog/v2"
)

func (s *CloudStorage) List(fileParam *models.FileParam) (int, error) {
	var w = s.Handler.ResponseWriter
	var r = s.Handler.Request
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

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	klog.Infoln("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)

	var data = &models.ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}
	klog.Infof("Cloud Drive List Params: %+v, meta: %d", data, meta)

	if meta == 1 {
		res, err := s.Service.GetFileMetaData(data)
		if err != nil {
			klog.Errorf("GetFileMetaData error: %v", err)
			return common.ErrToStatus(err), err
		}

		var fileMetadata = res.(*models.CloudResponse)
		return common.RenderJSON(w, r, fileMetadata)
	}

	res, err := s.Service.List(data)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return common.ErrToStatus(err), err
	}
	listRes := res.(*models.CloudListResponse)
	return common.RenderJSON(w, r, listRes)
}
