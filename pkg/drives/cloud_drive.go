package drives

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/fileutils"
	"files/pkg/img"
	"files/pkg/parser"
	"files/pkg/pool"
	"files/pkg/preview"
	"files/pkg/redisutils"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CloudDriveListParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type CloudDriveListResponse struct {
	StatusCode string                            `json:"status_code"`
	FailReason *string                           `json:"fail_reason,omitempty"`
	Data       []*CloudDriveListResponseFileData `json:"data"`
	sync.Mutex
}

type CloudDriveListResponseFileData struct {
	Path      string                          `json:"path"`
	Name      string                          `json:"name"`
	Size      int64                           `json:"size"`
	FileSize  int64                           `json:"fileSize"`
	Extension string                          `json:"extension"`
	Modified  *string                         `json:"modified,omitempty"`
	Mode      string                          `json:"mode"`
	IsDir     bool                            `json:"isDir"`
	IsSymlink bool                            `json:"isSymlink"`
	Type      string                          `json:"type"`
	Meta      *CloudDriveListResponseFileMeta `json:"meta,omitempty"`
}

type CloudDriveListResponseFileMeta struct {
	ETag         string  `json:"e_tag"`
	Key          string  `json:"key"`
	LastModified *string `json:"last_modified,omitempty"`
	Owner        *string `json:"owner,omitempty"`
	Size         int     `json:"size"`
	StorageClass string  `json:"storage_class"`
}

type CloudDriveMetaResponseMeta struct {
	ETag         string  `json:"e_tag"`
	Key          string  `json:"key"`
	LastModified *string `json:"last_modified,omitempty"`
	Owner        *string `json:"owner"`
	Size         int64   `json:"size"`
	StorageClass *string `json:"storage_class"`
}

type CloudDriveMetaResponseData struct {
	Path      string                     `json:"path"`
	Name      string                     `json:"name"`
	Size      int64                      `json:"size"`
	FileSize  int64                      `json:"fileSize"`
	Extension string                     `json:"extension"`
	Modified  *string                    `json:"modified,omitempty"`
	Mode      string                     `json:"mode"`
	IsDir     bool                       `json:"isDir"`
	IsSymlink bool                       `json:"isSymlink"`
	Type      string                     `json:"type"`
	Meta      CloudDriveMetaResponseMeta `json:"meta"`
}

type CloudDriveMetaResponse struct {
	StatusCode string                     `json:"status_code"`
	FailReason *string                    `json:"fail_reason"`
	Data       CloudDriveMetaResponseData `json:"data"`
}

type CloudDriveFocusedMetaInfos struct {
	Key   string `json:"key"`
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

type CloudDrivePostParam struct {
	ParentPath string `json:"parent_path"`
	FolderName string `json:"folder_name"`
	Drive      string `json:"drive"`
	Name       string `json:"name"`
}

type CloudDrivePostResponseFileMeta struct {
	Capabilities                 *bool       `json:"capabilities,omitempty"`
	CopyRequiresWriterPermission *bool       `json:"copyRequiresWriterPermission,omitempty"`
	CreatedTime                  *time.Time  `json:"createdTime,omitempty"`
	ExplicitlyTrashed            *bool       `json:"explicitlyTrashed,omitempty"`
	FileExtension                *string     `json:"fileExtension,omitempty"`
	FullFileExtension            *string     `json:"fullFileExtension,omitempty"`
	HasThumbnail                 *bool       `json:"hasThumbnail,omitempty"`
	HeadRevisionId               *string     `json:"headRevisionId,omitempty"`
	IconLink                     *string     `json:"iconLink,omitempty"`
	ID                           string      `json:"id"`
	IsAppAuthorized              *bool       `json:"isAppAuthorized,omitempty"`
	Kind                         string      `json:"kind"`
	LastModifyingUser            *struct{}   `json:"lastModifyingUser,omitempty"`
	LinkShareMetadata            *struct{}   `json:"linkShareMetadata,omitempty"`
	MD5Checksum                  *string     `json:"md5Checksum,omitempty"`
	MimeType                     string      `json:"mimeType"`
	ModifiedByMe                 *bool       `json:"modifiedByMe,omitempty"`
	ModifiedTime                 *time.Time  `json:"modifiedTime,omitempty"`
	Name                         string      `json:"name"`
	OriginalFilename             *string     `json:"originalFilename,omitempty"`
	OwnedByMe                    *bool       `json:"ownedByMe,omitempty"`
	Owners                       []*struct{} `json:"owners,omitempty"`
	QuotaBytesUsed               *int64      `json:"quotaBytesUsed,omitempty"`
	SHA1Checksum                 *string     `json:"sha1Checksum,omitempty"`
	SHA256Checksum               *string     `json:"sha256Checksum,omitempty"`
	Shared                       *bool       `json:"shared,omitempty"`
	SharedWithMeTime             *time.Time  `json:"sharedWithMeTime,omitempty"`
	Size                         *int64      `json:"size,omitempty"`
	Spaces                       *string     `json:"spaces,omitempty"`
	Starred                      *bool       `json:"starred,omitempty"`
	ThumbnailLink                *string     `json:"thumbnailLink,omitempty"`
	ThumbnailVersion             *int64      `json:"thumbnailVersion,omitempty"`
	Title                        *string     `json:"title,omitempty"`
	Trashed                      *bool       `json:"trashed,omitempty"`
	Version                      *int64      `json:"version,omitempty"`
	ViewedByMe                   *bool       `json:"viewedByMe,omitempty"`
	ViewedByMeTime               *time.Time  `json:"viewedByMeTime,omitempty"`
	ViewersCanCopyContent        *bool       `json:"viewersCanCopyContent,omitempty"`
	WebContentLink               *string     `json:"webContentLink,omitempty"`
	WebViewLink                  *string     `json:"webViewLink,omitempty"`
	WritersCanShare              *bool       `json:"writersCanShare,omitempty"`
}

type CloudDrivePostResponseFileData struct {
	Extension string                         `json:"extension"`
	FileSize  int64                          `json:"fileSize"`
	IsDir     bool                           `json:"isDir"`
	IsSymlink bool                           `json:"isSymlink"`
	Meta      CloudDrivePostResponseFileMeta `json:"meta"`
	Mode      string                         `json:"mode"`
	Modified  string                         `json:"modified"`
	Name      string                         `json:"name"`
	Path      string                         `json:"path"`
	Size      int64                          `json:"size"`
	Type      string                         `json:"type"`
}

type CloudDrivePostResponse struct {
	Data       CloudDrivePostResponseFileData `json:"data"`
	FailReason *string                        `json:"fail_reason,omitempty"`
	StatusCode string                         `json:"status_code"`
}

type CloudDrivePatchParam struct {
	Path        string `json:"path"`
	NewFileName string `json:"new_file_name"`
	Drive       string `json:"drive"`
	Name        string `json:"name"`
}

type CloudDriveDeleteParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type CloudDriveCopyFileParam struct {
	CloudFilePath     string `json:"cloud_file_path"`
	NewCloudDirectory string `json:"new_cloud_directory"`
	NewCloudFileName  string `json:"new_cloud_file_name"`
	Drive             string `json:"drive"`
	Name              string `json:"name"`
}

type CloudDriveMoveFileParam struct {
	CloudFilePath     string `json:"cloud_file_path"`
	NewCloudDirectory string `json:"new_cloud_directory"`
	Drive             string `json:"drive"`
	Name              string `json:"name"`
}

type CloudDriveDownloadFileParam struct {
	LocalFolder   string `json:"local_folder"`
	LocalFilename string `json:"local_file_name,omitempty"`
	CloudFilePath string `json:"cloud_file_path"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
}

type CloudDriveUploadFileParam struct {
	ParentPath    string `json:"parent_path"`
	LocalFilePath string `json:"local_file_path"`
	NewFileName   string `json:"new_file_name,omitempty"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
}

type CloudDriveTaskParameter struct {
	Drive         string `json:"drive"`
	CloudFilePath string `json:"cloud_file_path"`
	LocalFolder   string `json:"local_folder"`
	Name          string `json:"name"`
}

type CloudDriveTaskPauseInfo struct {
	FileSize  int64  `json:"file_size,omitempty"`
	Location  string `json:"location,omitempty"`
	NextStart int64  `json:"next_start,omitempty"`
}

type CloudDriveTaskResultData struct {
	FileInfo                 *CloudDriveListResponseFileData `json:"file_info,omitempty"`
	UploadFirstOperationTime int64                           `json:"upload_first_operation_time"`
}

type CloudDriveTaskData struct {
	ID            string                    `json:"id"`
	TaskType      string                    `json:"task_type"`
	Status        string                    `json:"status"`
	Progress      float64                   `json:"progress"`
	TaskParameter CloudDriveTaskParameter   `json:"task_parameter"`
	PauseInfo     *CloudDriveTaskPauseInfo  `json:"pause_info,omitempty"`
	ResultData    *CloudDriveTaskResultData `json:"result_data,omitempty"`
	UserName      string                    `json:"user_name"`
	DriverName    string                    `json:"driver_name"`
	FailedReason  *string                   `json:"failed_reason,omitempty"`
	WorkerName    *string                   `json:"worker_name,omitempty"`
	CreatedAt     *int64                    `json:"created_at,omitempty"`
	UpdatedAt     *int64                    `json:"updated_at,omitempty"`
}

type CloudDriveTaskResponse struct {
	StatusCode string             `json:"status_code"`
	FailReason *string            `json:"fail_reason,omitempty"`
	Data       CloudDriveTaskData `json:"data"`
}

type CloudDriveTaskQueryParam struct {
	TaskIds []string `json:"task_ids"`
}

type CloudDriveTaskQueryResponse struct {
	StatusCode string               `json:"status_code"`
	FailReason string               `json:"fail_reason"`
	Data       []CloudDriveTaskData `json:"data"`
}

type CloudDriveTaskIDResponse struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

func CloudDriveNormalizationPath(path, srcType string, same, addSuffix bool) string {
	if same != (srcType == SrcTypeAWSS3) {
		return path
	}

	if addSuffix && !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	if !addSuffix && strings.HasSuffix(path, "/") {
		return strings.TrimSuffix(path, "/")
	}

	return path
}

func GetCloudDriveMetadata(src string, w http.ResponseWriter, r *http.Request) (*CloudDriveMetaResponseData, error) {
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)

	param := CloudDriveListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return nil, err
	}
	klog.Infoln("Cloud Drive List Params:", string(jsonBody))
	respBody, err := CloudDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, nil, true)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return nil, err
	}

	var bodyJson CloudDriveMetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		klog.Error(err)
		return nil, err
	}
	return &bodyJson.Data, nil
}

func GetCloudDriveFocusedMetaInfos(src string, w http.ResponseWriter, r *http.Request) (info *CloudDriveFocusedMetaInfos, err error) {
	info = nil
	err = nil

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)

	param := CloudDriveListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return
	}
	klog.Infoln("Cloud Drive CloudDriveMetaResponseMeta Params:", string(jsonBody))
	respBody, err := CloudDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, nil, true)
	if err != nil {
		klog.Errorln("Error calling drive/get_file_meta_data:", err)
		return
	}

	var bodyJson CloudDriveMetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		klog.Error(err)
		return
	}

	if bodyJson.StatusCode == "FAIL" {
		err = e.New(*bodyJson.FailReason)
		return
	}

	info = &CloudDriveFocusedMetaInfos{
		Key:   bodyJson.Data.Meta.Key,
		Path:  bodyJson.Data.Path,
		Name:  bodyJson.Data.Name,
		Size:  bodyJson.Data.FileSize,
		IsDir: bodyJson.Data.IsDir,
	}
	return
}

func generateCloudDriveFilesData(srcType string, body []byte, stopChan <-chan struct{}, dataChan chan<- string,
	w http.ResponseWriter, r *http.Request, param CloudDriveListParam) {
	defer close(dataChan)

	var bodyJson CloudDriveListResponse
	if err := json.Unmarshal(body, &bodyJson); err != nil {
		klog.Error(err)
		return
	}

	var A []*CloudDriveListResponseFileData
	bodyJson.Lock()
	A = append(A, bodyJson.Data...)
	bodyJson.Unlock()

	for len(A) > 0 {
		klog.Infoln("len(A): ", len(A))
		firstItem := A[0]
		klog.Infoln("firstItem Path: ", firstItem.Path)
		klog.Infoln("firstItem Name:", firstItem.Name)
		firstItemPath := CloudDriveNormalizationPath(firstItem.Path, srcType, true, true)

		if firstItem.IsDir {
			firstParam := CloudDriveListParam{
				Path:  firstItemPath,
				Drive: param.Drive,
				Name:  param.Name,
			}
			firstJsonBody, err := json.Marshal(firstParam)
			if err != nil {
				klog.Errorln("Error marshalling JSON:", err)
				return
			}
			var firstRespBody []byte
			firstRespBody, err = CloudDriveCall("/drive/ls", "POST", firstJsonBody, w, r, nil, true)

			var firstBodyJson CloudDriveListResponse
			if err := json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
				klog.Error(err)
				return
			}

			A = append(firstBodyJson.Data, A[1:]...)
		} else {
			dataChan <- formatSSEvent(firstItem)

			A = A[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func streamCloudDriveFiles(w http.ResponseWriter, r *http.Request, srcType string, body []byte, param CloudDriveListParam) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateCloudDriveFilesData(srcType, body, stopChan, dataChan, w, r, param)

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

func CopyCloudDriveSingleFile(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		klog.Infoln("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPath := ParseCloudDrivePath(dst)
	klog.Infoln("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	dstDir, dstFilename := path.Split(dstPath)
	if dstDir == "" || dstFilename == "" {
		klog.Infoln("Dst parse failed.")
		return nil
	}
	trimmedDstDir := CloudDriveNormalizationPath(dstDir, srcDrive, false, false)
	if trimmedDstDir == "" {
		trimmedDstDir = "/"
	}

	param := CloudDriveCopyFileParam{
		CloudFilePath:     srcPath,       // id of "path/to/cloud/file.txt",
		NewCloudDirectory: trimmedDstDir, // id of "new/cloud/directory",
		NewCloudFileName:  dstFilename,   // "new_file_name.txt",
		Drive:             dstDrive,      // "my_drive",
		Name:              dstName,       // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Copy File Params:", string(jsonBody))
	_, err = CloudDriveCall("/drive/copy_file", "POST", jsonBody, w, r, nil, true)
	if err != nil {
		klog.Errorln("Error calling drive/copy_file:", err)
		return err
	}
	return nil
}

func CopyCloudDriveFolder(src, dst string, w http.ResponseWriter, r *http.Request, srcPath, srcPathName string) error {
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		klog.Infoln("Src parse failed.")
		return nil
	}
	srcPath = CloudDriveNormalizationPath(srcPath, srcDrive, true, true)

	dstDrive, dstName, dstPath := ParseCloudDrivePath(dst)
	klog.Infoln("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	dstDir, dstFilename := path.Split(strings.TrimSuffix(dstPath, "/"))
	if dstDir == "" || dstFilename == "" {
		klog.Infoln("Dst parse failed.")
		return nil
	}

	param := CloudDriveCopyFileParam{
		CloudFilePath:     srcPath,                              // id of "path/to/cloud/file.txt",
		NewCloudDirectory: dstDir,                               // id of "new/cloud/directory",
		NewCloudFileName:  strings.TrimSuffix(dstFilename, "/"), // "new_file_name.txt",
		Drive:             dstDrive,                             // "my_drive",
		Name:              dstName,                              // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Copy File Params:", string(jsonBody))
	_, err = CloudDriveCall("/drive/copy_file", "POST", jsonBody, w, r, nil, true)
	if err != nil {
		klog.Errorln("Error calling drive/copy_file:", err)
		return err
	}
	return nil
}

func CloudDriveFileToBuffer(src, bufferFilePath string, w http.ResponseWriter, r *http.Request) error {
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		klog.Infoln("Src parse failed.")
		return nil
	}

	param := CloudDriveDownloadFileParam{
		LocalFolder:   bufferFilePath,
		CloudFilePath: srcPath,
		Drive:         srcDrive,
		Name:          srcName,
	}
	if srcDrive == SrcTypeAWSS3 {
		param.LocalFilename = path.Base(srcPath)
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Download File Params:", string(jsonBody))

	var respBody []byte
	respBody, err = CloudDriveCall("/drive/download_async", "POST", jsonBody, w, r, nil, true)
	if err != nil {
		klog.Errorln("Error calling drive/download_async:", err)
		return err
	}

	var respJson CloudDriveTaskIDResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		klog.Error(err)
		return err
	}
	taskId := respJson.Data.ID
	taskParam := CloudDriveTaskQueryParam{
		TaskIds: []string{taskId},
	}
	taskJsonBody, err := json.Marshal(taskParam)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Task Params:", string(taskJsonBody))

	for {
		time.Sleep(1000 * time.Millisecond)
		var taskRespBody []byte
		taskRespBody, err = CloudDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, nil, true)
		if err != nil {
			klog.Errorln("Error calling drive/download_async:", err)
			return err
		}
		var taskRespJson CloudDriveTaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			klog.Error(err)
			return err
		}
		if len(taskRespJson.Data) == 0 {
			return e.New("Task Info Not Found")
		}
		if srcDrive == SrcTypeTencent && taskRespJson.Data[0].FailedReason != nil && *taskRespJson.Data[0].FailedReason == "Invalid task" {
			return nil
		}
		if taskRespJson.Data[0].Status != "Waiting" && taskRespJson.Data[0].Status != "InProgress" {
			if taskRespJson.Data[0].Status == "Completed" {
				return nil
			}
			return e.New(taskRespJson.Data[0].Status)
		}
	}
}

func CloudDriveBufferToFile(bufferFilePath, dst string, w http.ResponseWriter, r *http.Request) (int, error) {
	dstDrive, dstName, dstPath := ParseCloudDrivePath(dst)
	klog.Infoln("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	if dstPath == "" {
		klog.Infoln("Dst parse failed.")
		return http.StatusBadRequest, nil
	}
	dstDir, dstFileName := filepath.Split(strings.TrimSuffix(dstPath, "/"))

	trimmedDstDir := CloudDriveNormalizationPath(dstDir, dstDrive, false, false)
	if trimmedDstDir == "" {
		trimmedDstDir = "/"
	}

	//newBufferFilePath := bufferFilePath
	//bufferDir, bufferFileName := filepath.Split(bufferFilePath)
	//if bufferFileName != dstFileName {
	//	newBufferFilePath = filepath.Join(bufferDir, dstFileName)
	//	err := os.Rename(bufferFilePath, newBufferFilePath)
	//	if err != nil {
	//		klog.Errorln("Error renaming file:", err)
	//		return common.ErrToStatus(err), err
	//	}
	//}

	param := CloudDriveUploadFileParam{
		ParentPath:    trimmedDstDir,
		LocalFilePath: bufferFilePath,
		NewFileName:   dstFileName,
		Drive:         dstDrive,
		Name:          dstName,
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return common.ErrToStatus(err), err
	}
	klog.Infoln("Upload File Params:", string(jsonBody))

	var respBody []byte
	respBody, err = CloudDriveCall("/drive/upload_async", "POST", jsonBody, w, r, nil, true)
	if err != nil {
		klog.Errorln("Error calling drive/upload_async:", err)
		return common.ErrToStatus(err), err
	}
	var respJson CloudDriveTaskIDResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		klog.Error(err)
		return common.ErrToStatus(err), err
	}
	taskId := respJson.Data.ID
	taskParam := CloudDriveTaskQueryParam{
		TaskIds: []string{taskId},
	}
	taskJsonBody, err := json.Marshal(taskParam)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return common.ErrToStatus(err), err
	}
	klog.Infoln("Task Params:", string(taskJsonBody))

	for {
		time.Sleep(500 * time.Millisecond)
		var taskRespBody []byte
		taskRespBody, err = CloudDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, nil, true)
		if err != nil {
			klog.Errorln("Error calling drive/upload_async:", err)
			return common.ErrToStatus(err), err
		}
		var taskRespJson CloudDriveTaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			klog.Error(err)
			return common.ErrToStatus(err), err
		}
		if len(taskRespJson.Data) == 0 {
			err = e.New("Task Info Not Found")
			return common.ErrToStatus(err), err
		}
		if dstDrive == SrcTypeTencent && taskRespJson.Data[0].FailedReason != nil && *taskRespJson.Data[0].FailedReason == "Invalid task" {
			return http.StatusOK, nil
		}
		if taskRespJson.Data[0].Status != "Waiting" && taskRespJson.Data[0].Status != "InProgress" {
			if taskRespJson.Data[0].Status == "Completed" {
				return http.StatusOK, nil
			}
			err = e.New(taskRespJson.Data[0].Status)
			return common.ErrToStatus(err), err
		}
	}
}

func MoveCloudDriveFolderOrFiles(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	_, _, dstPath := ParseCloudDrivePath(dst)

	dstDir, _ := filepath.Split(dstPath)

	trimmedDstDir := CloudDriveNormalizationPath(dstDir, srcDrive, false, false)
	if trimmedDstDir == "" {
		trimmedDstDir = "/"
	}

	param := CloudDriveMoveFileParam{
		CloudFilePath:     srcPath,
		NewCloudDirectory: trimmedDstDir,
		Drive:             srcDrive, // "my_drive",
		Name:              srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Cloud Drive Move File Params:", string(jsonBody))
	_, err = CloudDriveCall("/drive/move_file", "POST", jsonBody, w, r, nil, false)
	if err != nil {
		klog.Errorln("Error calling drive/move_file:", err)
		return err
	}
	return nil
}

func CloudDriveCall(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, header *http.Header, returnResp bool) ([]byte, error) {
	var bflName = ""
	if header != nil {
		bflName = header.Get("X-Bfl-User")
	} else {
		bflName = r.Header.Get("X-Bfl-User")
	}
	if bflName == "" {
		return nil, os.ErrPermission
	}

	host := common.GetHost(bflName)
	dstUrl := host + dst

	klog.Infoln("dstUrl:", dstUrl)

	const maxRetries = 3
	const retryDelay = 2 * time.Second

	var resp *http.Response
	var err error
	var body []byte
	var datas map[string]interface{}

	for i := 0; i <= maxRetries; i++ {
		var req *http.Request
		if reqBodyJson != nil {
			req, err = http.NewRequest(method, dstUrl, bytes.NewBuffer(reqBodyJson))
		} else {
			req, err = http.NewRequest(method, dstUrl, nil)
		}

		if err != nil {
			klog.Errorln("Error creating request:", err)
			return nil, err
		}

		if header != nil {
			req.Header = *header
		} else {
			req.Header = r.Header.Clone()
			req.Header.Set("Content-Type", "application/json")
		}

		client := &http.Client{}
		resp, err = client.Do(req)
		if err != nil {
			klog.Errorln("Error making request:", err)
			return nil, err
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			klog.Infoln("Cloud Drive Call BflResponse is not JSON format:", contentType)
		}

		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				klog.Errorln("Error creating gzip reader:", err)
				return nil, err
			}
			defer reader.Close()

			body, err = ioutil.ReadAll(reader)
			if err != nil {
				klog.Errorln("Error reading gzipped response body:", err)
				return nil, err
			}
		} else {
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				klog.Errorln("Error reading response body:", err)
				return nil, err
			}
		}

		err = json.Unmarshal(body, &datas)
		if err != nil {
			klog.Errorln("Error unmarshaling JSON response:", err)
			if i == maxRetries {
				return nil, err
			}
			time.Sleep(retryDelay)
			continue
		}

		klog.Infoln("Parsed JSON response:", datas)

		if datas["status_code"].(string) != "SUCCESS" {
			err = e.New("Calling " + dst + " got the status " + datas["status_code"].(string))
			if i == maxRetries {
				return nil, err
			}
			time.Sleep(retryDelay)
			continue
		}

		break
	}

	responseText, err := json.MarshalIndent(datas, "", "  ")
	if err != nil {
		http.Error(w, "Error marshaling JSON response to text: "+err.Error(), http.StatusInternalServerError)
		return nil, err
	}

	if returnResp {
		return responseText, nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(responseText))
	return nil, nil
}

func ParseCloudDrivePath(src string) (drive, name, path string) {
	if strings.HasPrefix(src, "/Drive/") {
		src = src[7:]
	}
	parts := strings.SplitN(src, "/", 2)
	drive = parts[0]

	trimSuffix := true
	if drive == SrcTypeAWSS3 {
		trimSuffix = false
	}

	src = "/"
	if len(parts) > 1 {
		src += parts[1]
	}

	slashes := []int{}
	for i, char := range src {
		if char == '/' {
			slashes = append(slashes, i)
		}
	}

	if len(slashes) < 2 {
		klog.Infoln("Path does not contain enough slashes.")
		return drive, "", ""
	}

	name = src[1:slashes[1]]
	path = src[slashes[1]:]
	if trimSuffix && path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	return drive, name, path
}

type CloudDriveResourceService struct {
	BaseResourceService
}

func (rc *CloudDriveResourceService) GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
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

	param := CloudDriveListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return common.ErrToStatus(err), err
	}
	klog.Infoln("Cloud Drive List Params:", string(jsonBody))
	if stream == 1 {
		var body []byte
		body, err = CloudDriveCall("/drive/ls", "POST", jsonBody, w, r, nil, true)
		streamCloudDriveFiles(w, r, srcDrive, body, param)
		return 0, nil
	}
	if meta == 1 {
		_, err = CloudDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, nil, false)
	} else {
		_, err = CloudDriveCall("/drive/ls", "POST", jsonBody, w, r, nil, false)
	}
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return common.ErrToStatus(err), err
	}
	return 0, nil
}

func (rc *CloudDriveResourceService) DeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		_, status, err := ResourceDeleteCloudDrive(fileCache, r.URL.Path, w, r, true)
		return status, err
	}
}

func (rc *CloudDriveResourceService) PostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	_, status, err := ResourcePostCloudDrive(r.URL.Path, w, r, true)
	return status, err
}

func (rc *CloudDriveResourceService) PutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	// not public api for cloud drive, so it is not implemented
	return http.StatusNotImplemented, fmt.Errorf("cloud drive does not supoort editing files")
}

func (rc *CloudDriveResourceService) PatchHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		return ResourcePatchCloudDrive(fileCache, w, r)
	}
}

func (rs *CloudDriveResourceService) RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	src := r.URL.Path
	metaData, err := GetCloudDriveMetadata(src, w, r)
	if err != nil {
		klog.Error(err)
		return common.ErrToStatus(err), err
	}
	if metaData.IsDir {
		return http.StatusNotImplemented, fmt.Errorf("doesn't support directory download for cloud drive now")
	}
	return RawFileHandlerCloudDrive(src, w, r, metaData, bflName)
}

func (rs *CloudDriveResourceService) PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		vars := mux.Vars(r)

		previewSize, err := preview.ParsePreviewSize(vars["size"])
		if err != nil {
			return http.StatusBadRequest, err
		}
		path := "/" + vars["path"]

		return PreviewGetCloudDrive(w, r, previewSize, path, imgSvc, fileCache, enableThumbnails, resizePreview)
	}
}

func (rc *CloudDriveResourceService) PasteSame(task *pool.Task, action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	switch action {
	case "copy":
		metaInfo, err := GetCloudDriveFocusedMetaInfos(src, w, r)
		if err != nil {
			return err
		}

		if metaInfo.IsDir {
			return CopyCloudDriveFolder(src, dst, w, r, metaInfo.Path, metaInfo.Name)
		}
		return CopyCloudDriveSingleFile(src, dst, w, r)
	case "rename":
		return MoveCloudDriveFolderOrFiles(src, dst, w, r)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func (rs *CloudDriveResourceService) PasteDirFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	klog.Infof("~~~Temp log for Cloud Drive PasteDirFrom: srcType: %s, src: %s, dstType: %s, dst: %s", srcType, src, dstType, dst)
	klog.Infof("~~~Temp log for Cloud Drive PasteDirFrom: driveIdCache: %v", driveIdCache)
	mode := fileMode

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteDirTo(fs, src, dst, mode, w, r, d, driveIdCache)
	if err != nil {
		return err
	}

	var fdstBase string = dst
	if driveIdCache[src] != "" {
		fdstBase = filepath.Join(filepath.Dir(filepath.Dir(strings.TrimSuffix(dst, "/"))), driveIdCache[src])
	}
	klog.Infof("~~~Temp log for Cloud Drive PasteDirFrom: src: %s, fdstBase: %s, driveIdCache: %v", src, fdstBase, driveIdCache)

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	srcPath = CloudDriveNormalizationPath(srcPath, srcDrive, true, true)

	param := CloudDriveListParam{
		Path:  srcPath,
		Drive: srcDrive,
		Name:  srcName,
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Cloud Drive List Params:", string(jsonBody))
	var respBody []byte
	respBody, err = CloudDriveCall("/drive/ls", "POST", jsonBody, w, r, nil, true)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return err
	}
	var bodyJson CloudDriveListResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		klog.Error(err)
		return err
	}
	for _, item := range bodyJson.Data {
		fsrc := filepath.Join(src, item.Name)
		fdst := filepath.Join(fdstBase, item.Name)
		klog.Infoln(fsrc, fdst)
		if item.IsDir {
			fsrc = CloudDriveNormalizationPath(fsrc, srcType, true, true)
			fdst = CloudDriveNormalizationPath(fdst, dstType, true, true)
			err = rs.PasteDirFrom(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			err = rs.PasteFileFrom(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), item.FileSize, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *CloudDriveResourceService) PasteDirTo(fs afero.Fs, src, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	_, _, err := ResourcePostCloudDrive(dst, w, r, false)
	if err != nil {
		return err
	}
	return nil
}

func (rs *CloudDriveResourceService) PasteFileFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	klog.Infof("~~~Temp log for Cloud Drive PasteFileFrom: srcType: %s, src: %s, dstType: %s, dst: %s", srcType, srcType, dstType, dst)
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	var bufferPath string

	var err error
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return err
	}

	srcInfo, err := GetCloudDriveFocusedMetaInfos(src, w, r)
	bufferFilePath, err := GenerateBufferFolder(srcInfo.Path, bflName)
	if err != nil {
		return err
	}
	bufferPath = filepath.Join(bufferFilePath, srcInfo.Name)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)
	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return err
	}
	err = CloudDriveFileToBuffer(src, bufferFilePath, w, r)
	if err != nil {
		return err
	}

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(bufferPath, srcType)
	}()

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteFileTo(fs, bufferPath, dst, mode, w, r, d, diskSize)
	if err != nil {
		return err
	}
	return nil
}

func (rs *CloudDriveResourceService) PasteFileTo(fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	klog.Infoln("Begin to paste!")
	klog.Infoln("dst: ", dst)
	status, err := CloudDriveBufferToFile(bufferPath, dst, w, r)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *CloudDriveResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}

	metaInfo, err := GetCloudDriveFocusedMetaInfos(src, w, r)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return nil, metaInfo.Size, 0755, metaInfo.IsDir, nil
}

func (rs *CloudDriveResourceService) MoveDelete(fileCache fileutils.FileCache, src string, ctx context.Context, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	_, status, err := ResourceDeleteCloudDrive(fileCache, src, w, r, true)
	if status != http.StatusOK && status != 0 {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *CloudDriveResourceService) GeneratePathList(db *gorm.DB, rootPath string, processor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error {
	if rootPath == "" {
		rootPath = "/"
	}

	processedPaths := make(map[string]bool)

	for bflName, cookie := range common.BflCookieCache {
		klog.Infof("Key: %s, Value: %s\n", bflName, cookie)

		header := make(http.Header)

		header.Set("Content-Type", "application/json")
		header.Set("X-Bfl-User", bflName)
		header.Set("Cookie", cookie)

		// /drive/accounts logic is as same as google drive, but a little different from cloud drives. It is used only once, so call GoogleDriveCall here.
		repoRespBody, err := GoogleDriveCall("/drive/accounts", "POST", nil, nil, nil, &header, true)
		if err != nil {
			klog.Errorf("GoogleDriveCall failed: %v\n", err)
			return err
		}

		var data DriveAccountsResponse
		err = json.Unmarshal(repoRespBody, &data)
		if err != nil {
			klog.Errorf("unmarshal repo response failed: %v\n", err)
			return err
		}

		for _, datum := range data.Data {
			klog.Infof("datum=%v", datum)

			if datum.Type == SrcTypeGoogle {
				continue
			}

			rootParam := CloudDriveListParam{
				Path:  rootPath,
				Drive: datum.Type,
				Name:  datum.Name,
			}
			rootJsonBody, err := json.Marshal(rootParam)
			if err != nil {
				klog.Errorln("Error marshalling JSON:", err)
				return err
			}

			var direntRespBody []byte
			direntRespBody, err = CloudDriveCall("/drive/ls", "POST", rootJsonBody, nil, nil, &header, true)
			if err != nil {
				klog.Errorf("fetch repo response failed: %v\n", err)
				return err
			}

			generator := walkCloudDriveDirentsGenerator(direntRespBody, &header, nil, datum)

			for dirent := range generator {
				key := fmt.Sprintf("%s:%s", dirent.Drive, dirent.Path)
				processedPaths[key] = true

				_, err = processor(db, dirent.Drive, dirent.Path, dirent.Mtime)
				if err != nil {
					klog.Errorf("generate path list failed: %v\n", err)
					return err
				}
			}
		}
	}

	err := recordsStatusProcessor(db, processedPaths, []string{SrcTypeCloud, SrcTypeDropbox, SrcTypeAWSS3, SrcTypeTencent}, 1)
	if err != nil {
		klog.Errorf("records status processor failed: %v\n", err)
		return err
	}

	return nil
}

// just for complement, no need to use now
func (rs *CloudDriveResourceService) parsePathToURI(path string) (string, string) {
	return SrcTypeCloud, path
}

func ResourceDeleteCloudDrive(fileCache fileutils.FileCache, src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	klog.Infoln("src Path:", src)

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)

	param := CloudDriveDeleteParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return nil, common.ErrToStatus(err), err
	}
	klog.Infoln("Cloud Drive Delete Params:", string(jsonBody))

	// del thumbnails for Cloud Drive
	err = delThumbsCloudDrive(r.Context(), fileCache, src, w, r)
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	var respBody []byte = nil
	if returnResp {
		respBody, err = CloudDriveCall("/drive/delete", "POST", jsonBody, w, r, nil, true)
		klog.Infoln(string(respBody))
	} else {
		_, err = CloudDriveCall("/drive/delete", "POST", jsonBody, w, r, nil, false)
	}
	if err != nil {
		klog.Errorln("Error calling drive/delete:", err)
		return nil, common.ErrToStatus(err), err
	}
	return respBody, 0, nil
}

func ResourcePostCloudDrive(src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	klog.Infoln("src Path:", src)

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	klog.Infoln("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)
	srcPath = CloudDriveNormalizationPath(srcPath, srcDrive, true, false)
	path, newName := path.Split(srcPath)

	param := CloudDrivePostParam{
		ParentPath: path,
		FolderName: newName,
		Drive:      srcDrive, // "my_drive",
		Name:       srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return nil, common.ErrToStatus(err), err
	}
	klog.Infoln("Cloud Drive Post Params:", string(jsonBody))
	var respBody []byte = nil
	if returnResp {
		respBody, err = CloudDriveCall("/drive/create_folder", "POST", jsonBody, w, r, nil, true)
	} else {
		_, err = CloudDriveCall("/drive/create_folder", "POST", jsonBody, w, r, nil, false)
	}
	if err != nil {
		klog.Errorln("Error calling drive/create_folder:", err)
		return respBody, common.ErrToStatus(err), err
	}
	return respBody, 0, nil
}

func ResourcePatchCloudDrive(fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) (int, error) {
	src := r.URL.Path
	dst := r.URL.Query().Get("destination")
	dst, err := common.UnescapeURLIfEscaped(dst)

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	_, _, dstPath := ParseCloudDrivePath(dst)
	klog.Infoln("dstPath=", dstPath)
	dstDir, dstFilename := path.Split(dstPath)
	klog.Infoln("dstDir=", dstDir, ", dstFilename=", dstFilename)

	param := CloudDrivePatchParam{
		Path:        srcPath,
		NewFileName: dstFilename,
		Drive:       srcDrive, // "my_drive",
		Name:        srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return common.ErrToStatus(err), err
	}
	klog.Infoln("Cloud Drive Patch Params:", string(jsonBody))

	// del thumbnails for Cloud Drive
	err = delThumbsCloudDrive(r.Context(), fileCache, src, w, r)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	_, err = CloudDriveCall("/drive/rename", "POST", jsonBody, w, r, nil, false)
	if err != nil {
		klog.Errorln("Error calling drive/rename:", err)
		return common.ErrToStatus(err), err
	}
	return 0, nil
}

func setContentDispositionCloudDrive(w http.ResponseWriter, r *http.Request, fileName string) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(fileName))
	}
}

func ParseTimeString(s *string) time.Time {
	if s == nil || *s == "" {
		return time.Unix(0, 0)
	}
	parsed, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return time.Unix(0, 0)
	}
	return parsed
}

func previewCacheKeyCloudDrive(f *CloudDriveMetaResponseData, previewSize preview.PreviewSize) string {
	return fmt.Sprintf("%x%x%x", f.Path, ParseTimeString(f.Modified).Unix(), previewSize)
}

func createPreviewCloudDrive(w http.ResponseWriter, r *http.Request, src string, imgSvc preview.ImgService, fileCache fileutils.FileCache,
	file *CloudDriveMetaResponseData, previewSize preview.PreviewSize, bflName string) ([]byte, error) {
	klog.Infoln("!!!!CreatePreview:", previewSize)

	var err error
	diskSize := file.Size
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return nil, err
	}

	bufferFilePath, err := GenerateBufferFolder(file.Path, bflName)
	if err != nil {
		return nil, err
	}
	bufferPath := filepath.Join(bufferFilePath, file.Name)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)
	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return nil, err
	}
	err = CloudDriveFileToBuffer(src, bufferFilePath, w, r)
	if err != nil {
		return nil, err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	var (
		width   int
		height  int
		options []img.Option
	)

	switch {
	case previewSize == preview.PreviewSizeBig:
		width = 1080
		height = 1080
		options = append(options, img.WithMode(img.ResizeModeFit), img.WithQuality(img.QualityMedium))
	case previewSize == preview.PreviewSizeThumb:
		width = 256
		height = 256
		options = append(options, img.WithMode(img.ResizeModeFill), img.WithQuality(img.QualityLow), img.WithFormat(img.FormatJpeg))
	default:
		return nil, img.ErrUnsupportedFormat
	}

	buf := &bytes.Buffer{}
	if err := imgSvc.Resize(context.Background(), fd, width, height, buf, options...); err != nil {
		return nil, err
	}

	go func() {
		cacheKey := previewCacheKeyCloudDrive(file, previewSize)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			klog.Errorf("failed to cache resized image: %v", err)
		}
	}()

	klog.Infoln("Begin to remove buffer")
	RemoveDiskBuffer(bufferPath, SrcTypeCloud)
	return buf.Bytes(), nil
}

func RawFileHandlerCloudDrive(src string, w http.ResponseWriter, r *http.Request, file *CloudDriveMetaResponseData, bflName string) (int, error) {
	var err error
	diskSize := file.Size
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	bufferFilePath, err := GenerateBufferFolder(file.Path, bflName)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	bufferPath := filepath.Join(bufferFilePath, file.Name)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)
	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	err = CloudDriveFileToBuffer(src, bufferFilePath, w, r)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	defer fd.Close()

	setContentDispositionCloudDrive(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, ParseTimeString(file.Modified), fd)

	klog.Infoln("Begin to remove buffer")
	RemoveDiskBuffer(bufferPath, SrcTypeCloud)
	return 0, nil
}

func handleImagePreviewCloudDrive(
	w http.ResponseWriter,
	r *http.Request,
	src string,
	imgSvc preview.ImgService,
	fileCache fileutils.FileCache,
	file *CloudDriveMetaResponseData,
	previewSize preview.PreviewSize,
	enableThumbnails, resizePreview bool,
) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return common.ErrToStatus(os.ErrPermission), os.ErrPermission
	}

	if (previewSize == preview.PreviewSizeBig && !resizePreview) ||
		(previewSize == preview.PreviewSizeThumb && !enableThumbnails) {
		return RawFileHandlerCloudDrive(src, w, r, file, bflName)
	}

	format, err := imgSvc.FormatFromExtension(path.Ext(file.Name))
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return RawFileHandlerCloudDrive(src, w, r, file, bflName)
	}
	if err != nil {
		return common.ErrToStatus(err), err
	}

	cacheKey := previewCacheKeyCloudDrive(file, previewSize)
	klog.Infoln("cacheKey:", cacheKey)
	klog.Infoln("f.RealPath:", file.Path)
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	if !ok {
		resizedImage, err = createPreviewCloudDrive(w, r, src, imgSvc, fileCache, file, previewSize, bflName)
		if err != nil {
			return common.ErrToStatus(err), err
		}
	}

	err = redisutils.UpdateFileAccessTimeToRedis(redisutils.GetFileName(cacheKey))
	if err != nil {
		return common.ErrToStatus(err), err
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, ParseTimeString(file.Modified), bytes.NewReader(resizedImage))

	return 0, nil
}

func PreviewGetCloudDrive(w http.ResponseWriter, r *http.Request, previewSize preview.PreviewSize, path string,
	imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) (int, error) {
	src := path

	metaData, err := GetCloudDriveMetadata(src, w, r)
	if err != nil {
		klog.Error(err)
		return common.ErrToStatus(err), err
	}

	setContentDispositionCloudDrive(w, r, metaData.Name)

	fileType := parser.MimeTypeByExtension(metaData.Name)
	if strings.HasPrefix(fileType, "image") {
		return handleImagePreviewCloudDrive(w, r, src, imgSvc, fileCache, metaData, previewSize, enableThumbnails, resizePreview)
	} else {
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", fileType)
	}
}

func delThumbsCloudDrive(ctx context.Context, fileCache fileutils.FileCache, src string, w http.ResponseWriter, r *http.Request) error {
	metaData, err := GetCloudDriveMetadata(src, w, r)
	if err != nil {
		klog.Errorln("Error calling drive/get_file_meta_data:", err)
		return err
	}

	for _, previewSizeName := range preview.PreviewSizeNames() {
		size, _ := preview.ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKeyCloudDrive(metaData, size)
		if err := fileCache.Delete(ctx, cacheKey); err != nil {
			return err
		}
		err := redisutils.DelThumbRedisKey(redisutils.GetFileName(cacheKey))
		if err != nil {
			return err
		}
	}

	return nil
}

func walkCloudDriveDirentsGenerator(body []byte, header *http.Header, r *http.Request, datum DrivesAccounsResponseItem) <-chan DirentGeneratedEntry {
	ch := make(chan DirentGeneratedEntry)
	go func() {
		defer close(ch)

		var bodyJson CloudDriveListResponse
		if err := json.Unmarshal(body, &bodyJson); err != nil {
			klog.Error(err)
			return
		}

		queue := make([]*CloudDriveListResponseFileData, 0)
		bodyJson.Lock()
		queue = append(queue, bodyJson.Data...)
		bodyJson.Unlock()

		for len(queue) > 0 {
			firstItem := queue[0]
			queue = queue[1:]

			if firstItem.IsDir {
				fullPath := filepath.Join(datum.Type, datum.Name, firstItem.Path) + "/"
				klog.Infof("~~~Temp log: %s fullPath = %s, %s Path = %s", datum.Type, fullPath, datum.Type, firstItem.Path)
				var parsedTime time.Time
				var err error
				if firstItem.Modified != nil && *firstItem.Modified != "" {
					parsedTime, err = time.Parse(time.RFC3339, *firstItem.Modified)
					if err != nil {
						klog.Errorln("Parse time failed:", err)
						continue
					}
					klog.Infoln("Parsed Time:", parsedTime)
				} else {
					klog.Warningln("Modified is empty")
					parsedTime = time.Now()
				}
				entry := DirentGeneratedEntry{
					Drive: datum.Type,
					Path:  fullPath,
					Mtime: parsedTime,
				}
				ch <- entry

				paramPath := strings.TrimSuffix(firstItem.Path, "/") + "/"
				firstParam := CloudDriveListParam{
					Path:  paramPath,
					Drive: datum.Type,
					Name:  datum.Name,
				}
				firstJsonBody, err := json.Marshal(firstParam)
				if err != nil {
					klog.Errorln("Error marshalling JSON:", err)
					continue
				}

				firstRespBody, err := CloudDriveCall("/drive/ls", "POST", firstJsonBody, nil, r, header, true)
				if err != nil {
					klog.Error(err)
					continue
				}

				var firstBodyJson CloudDriveListResponse
				if err := json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
					klog.Error(err)
					continue
				}
				queue = append(queue, firstBodyJson.Data...)
			}
		}
	}()
	return ch
}
