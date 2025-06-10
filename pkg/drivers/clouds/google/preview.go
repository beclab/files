package google

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
	"files/pkg/utils"
	"fmt"
	"net/url"
	"path"
	"strings"

	"k8s.io/klog/v2"
)

func (s *GoogleStorage) Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error) {
	var owner = s.Base.Handler.Owner
	var w = s.Base.Handler.ResponseWriter
	var r = s.Base.Handler.Request

	var enableThumbnails = s.Base.Handler.Data.Server.EnableThumbnails
	var resizePreview = s.Base.Handler.Data.Server.ResizePreview

	klog.Infof("GOOGLE preview, owner: %s, enableThumbnails: %v, resizePreview: %v, param: %s", owner, enableThumbnails, resizePreview, fileParam.Json())

	var inline = r.URL.Query().Get("inline")

	var path = fileParam.Path
	path = strings.Trim(path, "/")

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  path,
	}

	res, err := s.Service.GetFileMetaData(data)
	if err != nil {
		klog.Errorf("google get file meta error: %v", err)
		return 503, err
	}

	var resp = res.(*models.GoogleDriveResponse)
	if resp.StatusCode != "SUCCESS" {
		klog.Errorf("google get file meta %s, msg: %v", resp.StatusCode, resp.Message)
		return 503, fmt.Errorf("%s", *resp.Message)
	}

	klog.Infof("google get file meta, name: %s", resp.Data.Name)

	if inline == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(resp.Data.Name))
	}

	var fileMetaType = resp.Data.Type

	if strings.HasPrefix(fileMetaType, "image/") { // image/jpeg
		// return imgSvc.Preview(resp.Data, w, r)
		s.handlerImagePreview(resp.Data, imgSvc, fileCache)
	}

	return 0, nil
}

func (s *GoogleStorage) handlerImagePreview(fileInfo *models.GoogleDriveResponseData, imgSvc preview.ImgService, fileCache fileutils.FileCache) error {
	var r = s.Base.Handler.Request
	var owner = s.Base.Handler.Owner
	var size = r.URL.Query().Get("size")
	var previewSize, err = preview.ParsePreviewSize(size)
	if err != nil {
		return err
	}

	klog.Infof("google preview, size, owner: %s, size: %s", owner, utils.ToJson(previewSize))

	format, err := imgSvc.FormatFromExtension(path.Ext(strings.TrimSuffix(fileInfo.Name, "/")))
	if err != nil {
		return err
	}

	klog.Infof("google preview, format, owner: %s, format: %s", owner, utils.ToJson(format))

	return nil
}
