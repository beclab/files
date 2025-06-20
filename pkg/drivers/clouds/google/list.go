package google

import (
	"errors"
	"files/pkg/common"
	"files/pkg/models"
	"files/pkg/utils"
	"net/http"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// List implements base.DriverInterface.
func (g *GoogleStorage) List(fileParam *models.FileParam) (int, error) {
	klog.Infof("CLOUD GOOGLE list, owner: %s, param: %s", g.Base.Handler.Owner, fileParam.Json())
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

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  fileParam.Path,
	}

	klog.Infof("GOOGLE BASE list, owner: %s, get: %s", g.Base.Handler.Owner, utils.ToJson(data))

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

	klog.Infof("CLOUD GOOGLE list result, count: %d", len(listFiles.Data))

	return common.RenderJSON(w, r, listFiles)

}
