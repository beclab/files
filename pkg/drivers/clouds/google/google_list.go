package google

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/models"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// List implements base.DriverInterface.
func (g *GoogleStorage) List() (int, error) {

	var w = g.Base.Base.ResponseWriter
	var r = g.Base.Base.Request

	streamStr := r.URL.Query().Get("stream")
	stream := 0

	var res any
	var listFiles *models.GoogleDriveListResponse
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
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	var param = &models.ListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return common.ErrToStatus(err), err
	}
	klog.Infof("Google Drive List Params: %s, stream: %d, meta: %d", string(jsonBody), stream, meta)

	if stream == 1 {
		res, err = g.Service.List(param)
		if err != nil {
			klog.Errorf("List error: %v", err)
			return common.ErrToStatus(err), err
		}

		listFiles = res.(*models.GoogleDriveListResponse)
		if !listFiles.IsSuccess() {
			err = errors.New(listFiles.FailMessage())
			return common.ErrToStatus(err), err
		}

		g.streamGoogleDriveFiles(listFiles, param)
		return common.RenderSuccess(w, r)
	}

	if meta == 1 {
		res, err = g.Service.GetFileMetaData(param)
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

	res, err = g.Service.List(param)
	if err != nil {
		klog.Errorf("List error: %v", err)
		return common.ErrToStatus(err), err
	}

	listFiles = res.(*models.GoogleDriveListResponse)
	if !listFiles.IsSuccess() {
		err = errors.New(listFiles.FailMessage())
		return common.ErrToStatus(err), err
	}

	klog.Infof("Google Drive List Result, count: %d", len(listFiles.Data))

	return common.RenderJSON(w, r, listFiles)

}

func (g *GoogleStorage) streamGoogleDriveFiles(files *models.GoogleDriveListResponse, param *models.ListParam) {
	var w = g.Base.Base.ResponseWriter
	var r = g.Base.Base.Request

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go g.generateGoogleDriveFilesData(files, stopChan, dataChan, param)

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

func (g *GoogleStorage) generateGoogleDriveFilesData(files *models.GoogleDriveListResponse, stopChan <-chan struct{}, dataChan chan<- string, param *models.ListParam) {
	defer close(dataChan)

	var A []*models.GoogleDriveResponseData
	files.Lock()
	A = append(A, files.Data...)
	files.Unlock()

	for len(A) > 0 {
		klog.Infoln("len(A): ", len(A))
		firstItem := A[0]
		klog.Infoln("firstItem Path: ", firstItem.Path)
		klog.Infoln("firstItem Name:", firstItem.Name)

		if firstItem.IsDir {
			pathId := firstItem.Meta.ID
			nextParam := &models.ListParam{
				Path:  pathId,
				Drive: param.Drive,
				Name:  param.Name,
			}

			klog.Infoln("firstParam pathId:", pathId)
			res, err := g.Service.List(nextParam)
			if err != nil {
				klog.Error(err)
				return
			}

			var listFiles = res.(*models.GoogleDriveListResponse)
			if !listFiles.IsSuccess() {
				klog.Error(listFiles.FailMessage())
				return
			}

			klog.Infof("Google Drive List Stream Result, count: %d", len(listFiles.Data))

			A = append(listFiles.Data, A[1:]...)
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
