package base

import (
	"files/pkg/common"
	"files/pkg/models"
	"fmt"
	"net/http"
	"strconv"

	"k8s.io/klog/v2"
)

func (s *CloudStorage) List() (int, error) {
	var w = s.Base.ResponseWriter
	var r = s.Base.Request

	streamStr := r.URL.Query().Get("stream")
	stream := 0

	var err error
	if streamStr != "" {
		stream, err = strconv.Atoi(streamStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

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

	var param = &models.ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}
	klog.Infof("Cloud Drive List Params: %+v, stream: %d, meta: %d", param, stream, meta)
	if stream == 1 {
		var res any
		res, err = s.Service.List(param)
		listResult := res.(*models.CloudListResponse)
		s.streamCloudDriveFiles(srcDrive, listResult, param)
		return common.RenderSuccess(w, r)
	}
	if meta == 1 {
		res, err := s.Service.GetFileMetaData(param)
		if err != nil {
			klog.Errorf("GetFileMetaData error: %v", err)
			return common.ErrToStatus(err), err
		}

		var fileMetadata = res.(*models.CloudResponse)
		return common.RenderJSON(w, r, fileMetadata)
	}

	res, err := s.Service.List(param)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return common.ErrToStatus(err), err
	}
	listRes := res.(*models.CloudListResponse)
	return common.RenderJSON(w, r, listRes)
}

func (s *CloudStorage) streamCloudDriveFiles(srcType string, filesResult *models.CloudListResponse, param *models.ListParam) {
	var w = s.Base.ResponseWriter
	var r = s.Base.Request
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go s.generateCloudDriveFilesData(srcType, filesResult, stopChan, dataChan, param)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case event, ok := <-dataChan:
			if !ok {
				return
			}
			_, err := w.Write([]byte(event))
			if err != nil {
				klog.Error(err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			close(stopChan)
			return
		}
	}
}

func (s *CloudStorage) generateCloudDriveFilesData(srcType string, fileResult *models.CloudListResponse, stopChan <-chan struct{}, dataChan chan<- string,
	param *models.ListParam) {
	defer close(dataChan)

	var A []*models.CloudResponseData
	fileResult.Lock()
	A = append(A, fileResult.Data...)
	fileResult.Unlock()

	for len(A) > 0 {
		klog.Infoln("len(A): ", len(A))
		firstItem := A[0]
		klog.Infoln("firstItem Path: ", firstItem.Path)
		klog.Infoln("firstItem Name:", firstItem.Name)
		firstItemPath := CloudDriveNormalizationPath(firstItem.Path, srcType, true, true)

		if firstItem.IsDir {
			firstParam := &models.ListParam{
				Path:  firstItemPath,
				Drive: param.Drive,
				Name:  param.Name,
			}

			res, err := s.Service.List(firstParam)
			if err != nil {
				klog.Error(err)
				return
			}
			files := res.(*models.CloudListResponse)

			A = append(files.Data, A[1:]...)
		} else {
			dataChan <- fmt.Sprintf("data: %s\n\n", firstItem.String())

			A = A[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}
