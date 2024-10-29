package http

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	e "errors"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/img"
	"github.com/filebrowser/filebrowser/v2/my_redis"
	"github.com/filebrowser/filebrowser/v2/parser"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
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
	CloudFilePath string `json:"cloud_file_path"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
}

type CloudDriveUploadFileParam struct {
	ParentPath    string `json:"parent_path"`
	LocalFilePath string `json:"local_file_path"`
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

func getCloudDriveMetadata(src string, w http.ResponseWriter, r *http.Request) (*CloudDriveMetaResponseData, error) {
	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)

	param := CloudDriveListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil, err
	}
	fmt.Println("Cloud Drive List Params:", string(jsonBody))
	respBody, err := CloudDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/ls:", err)
		return nil, err
	}

	var bodyJson CloudDriveMetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		fmt.Println(err)
		return nil, err
	}
	return &bodyJson.Data, nil
}

func getCloudDriveFocusedMetaInfos(src string, w http.ResponseWriter, r *http.Request) (info *CloudDriveFocusedMetaInfos, err error) {
	src = strings.TrimSuffix(src, "/")
	info = nil
	err = nil

	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)

	param := CloudDriveListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	fmt.Println("Cloud Drive CloudDriveMetaResponseMeta Params:", string(jsonBody))
	respBody, err := CloudDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/get_file_meta_data:", err)
		return
	}

	var bodyJson CloudDriveMetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		fmt.Println(err)
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

func generateCloudDriveFilesData(body []byte, stopChan <-chan struct{}, dataChan chan<- string,
	w http.ResponseWriter, r *http.Request, param CloudDriveListParam) {
	defer close(dataChan)

	var bodyJson CloudDriveListResponse
	if err := json.Unmarshal(body, &bodyJson); err != nil {
		fmt.Println(err)
		return
	}

	var A []*CloudDriveListResponseFileData
	bodyJson.Lock()
	A = append(A, bodyJson.Data...)
	bodyJson.Unlock()

	for len(A) > 0 {
		fmt.Println("len(A): ", len(A))
		firstItem := A[0]
		fmt.Println("firstItem Path: ", firstItem.Path)
		fmt.Println("firstItem Name:", firstItem.Name)

		if firstItem.IsDir {
			firstParam := CloudDriveListParam{
				Path:  firstItem.Path,
				Drive: param.Drive,
				Name:  param.Name,
			}
			firstJsonBody, err := json.Marshal(firstParam)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				fmt.Println(err)
				return
			}
			var firstRespBody []byte
			firstRespBody, err = CloudDriveCall("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson CloudDriveListResponse
			if err := json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
				fmt.Println(err)
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

func streamCloudDriveFiles(w http.ResponseWriter, r *http.Request, body []byte, param CloudDriveListParam) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateCloudDriveFilesData(body, stopChan, dataChan, w, r, param)

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
				fmt.Println(err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			close(stopChan)
			return
		}
	}
}

func copyCloudDriveSingleFile(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		fmt.Println("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPath := parseCloudDrivePath(dst, true)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	dstDir, dstFilename := path.Split(dstPath)
	if dstDir == "" || dstFilename == "" {
		fmt.Println("Dst parse failed.")
		return nil
	}
	trimmedDstDir := strings.TrimSuffix(dstDir, "/")
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
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println("Copy File Params:", string(jsonBody))
	_, err = CloudDriveCall("/drive/copy_file", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/copy_file:", err)
		return err
	}
	return nil
}

func copyCloudDriveFolder(src, dst string, w http.ResponseWriter, r *http.Request, srcPath, srcPathName string) error {
	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		fmt.Println("Src parse failed.")
		return nil
	}
	srcDir, srcFilename := path.Split(srcPath)
	if srcDir == "" || srcFilename == "" {
		fmt.Println("Src parse failed.")
		return nil
	}

	dstDrive, dstName, dstPath := parseCloudDrivePath(dst, true)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	if dstPath == "" {
		fmt.Println("Dst parse failed.")
		return nil
	}
	dstDir, dstFilename := path.Split(dstPath)
	if dstDir == "" || dstFilename == "" {
		fmt.Println("Dst parse failed.")
		return nil
	}

	var recursivePath = srcPath
	var A []*CloudDriveListResponseFileData
	for {
		fmt.Println("len(A): ", len(A))

		var isDir = true
		var firstItem *CloudDriveListResponseFileData
		if len(A) > 0 {
			firstItem = A[0]
			recursivePath = firstItem.Path
			isDir = firstItem.IsDir
		}

		if isDir {
			var parentPath string
			var folderName string
			if srcPath == recursivePath {
				parentPath = dstDir
				folderName = dstFilename
			} else {
				parentPath = dstPath + strings.TrimPrefix(filepath.Dir(firstItem.Path), srcPath)
				folderName = filepath.Base(firstItem.Path)
			}
			postParam := CloudDrivePostParam{
				ParentPath: parentPath,
				FolderName: folderName,
				Drive:      srcDrive,
				Name:       srcName,
			}
			postJsonBody, err := json.Marshal(postParam)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				return err
			}
			fmt.Println("Google Drive Post Params:", string(postJsonBody))
			var postRespBody []byte
			postRespBody, err = CloudDriveCall("/drive/create_folder", "POST", postJsonBody, w, r, true)
			if err != nil {
				fmt.Println("Error calling drive/create_folder:", err)
				return err
			}
			var postBodyJson CloudDrivePostResponse
			if err = json.Unmarshal(postRespBody, &postBodyJson); err != nil {
				fmt.Println(err)
				return err
			}

			firstParam := CloudDriveListParam{
				Path:  recursivePath,
				Drive: srcDrive,
				Name:  srcName,
			}

			fmt.Println("firstParam path:", recursivePath)
			var firstJsonBody []byte
			firstJsonBody, err = json.Marshal(firstParam)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				return err
			}
			var firstRespBody []byte
			firstRespBody, err = CloudDriveCall("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson CloudDriveListResponse
			if err = json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
				fmt.Println(err)
				return err
			}

			if len(A) == 0 {
				A = firstBodyJson.Data
			} else {
				A = append(firstBodyJson.Data, A[1:]...)
			}
		} else {
			if len(A) > 0 {
				copyPathPrefix := "/Drive/" + srcDrive + "/" + srcName
				copySrc := copyPathPrefix + firstItem.Path
				parentPath := dstPath + strings.TrimPrefix(filepath.Dir(firstItem.Path), srcPath)
				copyDst := copyPathPrefix + parentPath + "/" + firstItem.Name
				fmt.Println("copySrc: ", copySrc)
				fmt.Println("copyDst: ", copyDst)
				err := copyCloudDriveSingleFile(copySrc, copyDst, w, r)
				if err != nil {
					return err
				}
				A = A[1:]
			}
		}
		if len(A) == 0 {
			return nil
		}
	}
}

func cloudDriveFileToBuffer(src, bufferFilePath string, w http.ResponseWriter, r *http.Request) error {
	src = strings.TrimSuffix(src, "/")
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}
	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		fmt.Println("Src parse failed.")
		return nil
	}

	param := CloudDriveDownloadFileParam{
		LocalFolder:   bufferFilePath,
		CloudFilePath: srcPath,
		Drive:         srcDrive,
		Name:          srcName,
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println("Download File Params:", string(jsonBody))

	var respBody []byte
	respBody, err = CloudDriveCall("/drive/download_async", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/download_async:", err)
		return err
	}

	var respJson CloudDriveTaskIDResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		fmt.Println(err)
		return err
	}
	taskId := respJson.Data.ID
	taskParam := CloudDriveTaskQueryParam{
		TaskIds: []string{taskId},
	}
	taskJsonBody, err := json.Marshal(taskParam)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println("Task Params:", string(taskJsonBody))

	for {
		time.Sleep(1000 * time.Millisecond)
		var taskRespBody []byte
		taskRespBody, err = CloudDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/download_async:", err)
			return err
		}
		var taskRespJson CloudDriveTaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			fmt.Println(err)
			return err
		}
		if len(taskRespJson.Data) == 0 {
			return e.New("Task Info Not Found")
		}
		if srcDrive == "tencent" && taskRespJson.Data[0].FailedReason != nil && *taskRespJson.Data[0].FailedReason == "Invalid task" {
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

func cloudDriveBufferToFile(bufferFilePath, dst string, w http.ResponseWriter, r *http.Request) (int, error) {
	dstDrive, dstName, dstPath := parseCloudDrivePath(dst, true)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	if dstPath == "" {
		fmt.Println("Src parse failed.")
		return http.StatusBadRequest, nil
	}
	dstDir, _ := filepath.Split(dstPath)

	trimmedDstDir := strings.TrimSuffix(dstDir, "/")
	if trimmedDstDir == "" {
		trimmedDstDir = "/"
	}

	param := CloudDriveUploadFileParam{
		ParentPath:    trimmedDstDir,
		LocalFilePath: bufferFilePath,
		Drive:         dstDrive,
		Name:          dstName,
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Upload File Params:", string(jsonBody))

	var respBody []byte
	respBody, err = CloudDriveCall("/drive/upload_async", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/upload_async:", err)
		return errToStatus(err), err
	}
	var respJson CloudDriveTaskIDResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		fmt.Println(err)
		return errToStatus(err), err
	}
	taskId := respJson.Data.ID
	taskParam := CloudDriveTaskQueryParam{
		TaskIds: []string{taskId},
	}
	taskJsonBody, err := json.Marshal(taskParam)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Task Params:", string(taskJsonBody))

	for {
		time.Sleep(500 * time.Millisecond)
		var taskRespBody []byte
		taskRespBody, err = CloudDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/upload_async:", err)
			return errToStatus(err), err
		}
		var taskRespJson CloudDriveTaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			fmt.Println(err)
			return errToStatus(err), err
		}
		if len(taskRespJson.Data) == 0 {
			err = e.New("Task Info Not Found")
			return errToStatus(err), err
		}
		if dstDrive == "tencent" && taskRespJson.Data[0].FailedReason != nil && *taskRespJson.Data[0].FailedReason == "Invalid task" {
			return http.StatusOK, nil
		}
		if taskRespJson.Data[0].Status != "Waiting" && taskRespJson.Data[0].Status != "InProgress" {
			if taskRespJson.Data[0].Status == "Completed" {
				return http.StatusOK, nil
			}
			err = e.New(taskRespJson.Data[0].Status)
			return errToStatus(err), err
		}
	}
}

func moveCloudDriveFolderOrFiles(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)
	_, _, dstPath := parseCloudDrivePath(dst, true)

	dstDir, _ := filepath.Split(dstPath)

	trimmedDstDir := strings.TrimSuffix(dstDir, "/")
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
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println("Cloud Drive Move File Params:", string(jsonBody))
	_, err = CloudDriveCall("/drive/move_file", "POST", jsonBody, w, r, false)
	if err != nil {
		fmt.Println("Error calling drive/move_file:", err)
		return err
	}
	return nil
}

func CloudDriveCall(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return nil, os.ErrPermission
	}

	host := getHost(r)
	dstUrl := host + dst

	fmt.Println("dstUrl:", dstUrl)

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
			fmt.Println("Error creating request:", err)
			return nil, err
		}

		req.Header = r.Header.Clone()
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err = client.Do(req)
		if err != nil {
			fmt.Println("Error making request:", err)
			return nil, err
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			fmt.Println("Cloud Drive Call BflResponse is not JSON format:", contentType)
		}

		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				fmt.Println("Error creating gzip reader:", err)
				return nil, err
			}
			defer reader.Close()

			body, err = ioutil.ReadAll(reader)
			if err != nil {
				fmt.Println("Error reading gzipped response body:", err)
				return nil, err
			}
		} else {
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("Error reading response body:", err)
				return nil, err
			}
		}

		err = json.Unmarshal(body, &datas)
		if err != nil {
			fmt.Println("Error unmarshaling JSON response:", err)
			if i == maxRetries {
				return nil, err
			}
			time.Sleep(retryDelay)
			continue
		}

		fmt.Println("Parsed JSON response:", datas)

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

func parseCloudDrivePath(src string, trimSuffix bool) (drive, name, path string) {
	if strings.HasPrefix(src, "/Drive/") {
		src = src[7:]
	}
	parts := strings.SplitN(src, "/", 2)
	drive = parts[0]
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
		fmt.Println("Path does not contain enough slashes.")
		return drive, "", ""
	}

	name = src[1:slashes[1]]
	path = src[slashes[1]:]
	if trimSuffix && path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	return drive, name, path
}

func resourceGetCloudDrive(w http.ResponseWriter, r *http.Request, stream int, meta int) (int, error) {
	src := r.URL.Path
	fmt.Println("src Path:", src)

	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)
	fmt.Println("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)

	param := CloudDriveListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Cloud Drive List Params:", string(jsonBody))
	if stream == 1 {
		var body []byte
		body, err = CloudDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
		streamCloudDriveFiles(w, r, body, param)
		return 0, nil
	}
	if meta == 1 {
		_, err = CloudDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, false)
	} else {
		_, err = CloudDriveCall("/drive/ls", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/ls:", err)
		return errToStatus(err), err
	}
	return 0, nil
}

func resourcePostCloudDrive(src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	fmt.Println("src Path:", src)

	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)
	fmt.Println("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)
	path, newName := path.Split(srcPath)

	param := CloudDrivePostParam{
		ParentPath: path,
		FolderName: newName,
		Drive:      srcDrive, // "my_drive",
		Name:       srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil, errToStatus(err), err
	}
	fmt.Println("Cloud Drive Post Params:", string(jsonBody))
	var respBody []byte = nil
	if returnResp {
		respBody, err = CloudDriveCall("/drive/create_folder", "POST", jsonBody, w, r, true)
	} else {
		_, err = CloudDriveCall("/drive/create_folder", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/create_folder:", err)
		return respBody, errToStatus(err), err
	}
	return respBody, 0, nil
}

func resourcePatchCloudDrive(fileCache FileCache, w http.ResponseWriter, r *http.Request) (int, error) {
	src := r.URL.Path
	dst := r.URL.Query().Get("destination")
	//action := r.URL.Query().Get("action")
	dst, err := unescapeURLIfEscaped(dst) // url.QueryUnescape(dst)

	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)
	_, _, dstPath := parseCloudDrivePath(dst, true)
	fmt.Println("dstPath=", dstPath)
	dstDir, dstFilename := path.Split(dstPath)
	fmt.Println("dstDir=", dstDir, ", dstFilename=", dstFilename)

	param := CloudDrivePatchParam{
		Path:        srcPath,
		NewFileName: dstFilename,
		Drive:       srcDrive, // "my_drive",
		Name:        srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Cloud Drive Patch Params:", string(jsonBody))

	// del thumbnails for Cloud Drive
	err = delThumbsCloudDrive(r.Context(), fileCache, src, w, r)
	if err != nil {
		return errToStatus(err), err
	}

	_, err = CloudDriveCall("/drive/rename", "POST", jsonBody, w, r, false)
	if err != nil {
		fmt.Println("Error calling drive/rename:", err)
		return errToStatus(err), err
	}
	return 0, nil
}

func resourceDeleteCloudDrive(fileCache FileCache, src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	fmt.Println("src Path:", src)
	if strings.HasSuffix(src, "/") {
		src = strings.TrimSuffix(src, "/")
	}

	srcDrive, srcName, srcPath := parseCloudDrivePath(src, true)

	param := CloudDriveDeleteParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil, errToStatus(err), err
	}
	fmt.Println("Cloud Drive Delete Params:", string(jsonBody))

	// del thumbnails for Cloud Drive
	err = delThumbsCloudDrive(r.Context(), fileCache, src, w, r)
	if err != nil {
		return nil, errToStatus(err), err
	}

	var respBody []byte = nil
	if returnResp {
		respBody, err = CloudDriveCall("/drive/delete", "POST", jsonBody, w, r, true)
		fmt.Println(string(respBody))
	} else {
		_, err = CloudDriveCall("/drive/delete", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/delete:", err)
		return nil, errToStatus(err), err
	}
	return respBody, 0, nil
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

func previewCacheKeyCloudDrive(f *CloudDriveMetaResponseData, previewSize PreviewSize) string {
	return fmt.Sprintf("%x%x%x", f.Path, ParseTimeString(f.Modified).Unix(), previewSize)
}

func createPreviewCloudDrive(w http.ResponseWriter, r *http.Request, src string, imgSvc ImgService, fileCache FileCache,
	file *CloudDriveMetaResponseData, previewSize PreviewSize, bflName string) ([]byte, error) {
	fmt.Println("!!!!CreatePreview:", previewSize)

	var err error
	diskSize := file.Size
	_, err = checkBufferDiskSpace(diskSize)
	if err != nil {
		return nil, err
	}

	bufferFilePath, err := generateBufferFolder(file.Path, bflName)
	if err != nil {
		return nil, err
	}
	bufferPath := filepath.Join(bufferFilePath, file.Name)
	fmt.Println("Buffer file path: ", bufferFilePath)
	fmt.Println("Buffer path: ", bufferPath)
	err = makeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return nil, err
	}
	err = cloudDriveFileToBuffer(src, bufferFilePath, w, r)
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
	case previewSize == PreviewSizeBig:
		width = 1080
		height = 1080
		options = append(options, img.WithMode(img.ResizeModeFit), img.WithQuality(img.QualityMedium))
	case previewSize == PreviewSizeThumb:
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
			fmt.Printf("failed to cache resized image: %v", err)
		}
	}()

	fmt.Println("Begin to remove buffer")
	removeDiskBuffer(bufferPath, "cloud")
	return buf.Bytes(), nil
}

func rawFileHandlerCloudDrive(src string, w http.ResponseWriter, r *http.Request, file *CloudDriveMetaResponseData, bflName string) (int, error) {
	var err error
	diskSize := file.Size
	_, err = checkBufferDiskSpace(diskSize)
	if err != nil {
		return errToStatus(err), err
	}

	bufferFilePath, err := generateBufferFolder(file.Path, bflName)
	if err != nil {
		return errToStatus(err), err
	}
	bufferPath := filepath.Join(bufferFilePath, file.Name)
	fmt.Println("Buffer file path: ", bufferFilePath)
	fmt.Println("Buffer path: ", bufferPath)
	err = makeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return errToStatus(err), err
	}
	err = cloudDriveFileToBuffer(src, bufferFilePath, w, r)
	if err != nil {
		return errToStatus(err), err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return errToStatus(err), err
	}
	defer fd.Close()

	setContentDispositionCloudDrive(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, ParseTimeString(file.Modified), fd)

	fmt.Println("Begin to remove buffer")
	removeDiskBuffer(bufferPath, "cloud")
	return 0, nil
}

func handleImagePreviewCloudDrive(
	w http.ResponseWriter,
	r *http.Request,
	src string,
	imgSvc ImgService,
	fileCache FileCache,
	file *CloudDriveMetaResponseData,
	previewSize PreviewSize,
	enableThumbnails, resizePreview bool,
) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return errToStatus(os.ErrPermission), os.ErrPermission
	}

	if (previewSize == PreviewSizeBig && !resizePreview) ||
		(previewSize == PreviewSizeThumb && !enableThumbnails) {
		return rawFileHandlerCloudDrive(src, w, r, file, bflName)
	}

	format, err := imgSvc.FormatFromExtension(path.Ext(file.Name))
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return rawFileHandlerCloudDrive(src, w, r, file, bflName)
	}
	if err != nil {
		return errToStatus(err), err
	}

	cacheKey := previewCacheKeyCloudDrive(file, previewSize)
	fmt.Println("cacheKey:", cacheKey)
	fmt.Println("f.RealPath:", file.Path)
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return errToStatus(err), err
	}
	if !ok {
		resizedImage, err = createPreviewCloudDrive(w, r, src, imgSvc, fileCache, file, previewSize, bflName)
		if err != nil {
			return errToStatus(err), err
		}
	}

	err = my_redis.UpdateFileAccessTimeToRedis(my_redis.GetFileName(cacheKey))
	if err != nil {
		return errToStatus(err), err
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, ParseTimeString(file.Modified), bytes.NewReader(resizedImage))

	return 0, nil
}

func previewGetCloudDrive(w http.ResponseWriter, r *http.Request, previewSize PreviewSize, path string,
	imgSvc ImgService, fileCache FileCache, enableThumbnails, resizePreview bool) (int, error) {
	src := path
	if strings.HasSuffix(src, "/") {
		src = strings.TrimSuffix(src, "/")
	}

	metaData, err := getCloudDriveMetadata(src, w, r)
	if err != nil {
		fmt.Println(err)
		return errToStatus(err), err
	}

	setContentDispositionCloudDrive(w, r, metaData.Name)

	fileType := parser.MimeTypeByExtension(metaData.Name)
	if strings.HasPrefix(fileType, "image") {
		return handleImagePreviewCloudDrive(w, r, src, imgSvc, fileCache, metaData, previewSize, enableThumbnails, resizePreview)
	} else {
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", fileType)
	}
}

func delThumbsCloudDrive(ctx context.Context, fileCache FileCache, src string, w http.ResponseWriter, r *http.Request) error {
	metaData, err := getCloudDriveMetadata(src, w, r)
	if err != nil {
		fmt.Println("Error calling drive/get_file_meta_data:", err)
		return err
	}

	for _, previewSizeName := range PreviewSizeNames() {
		size, _ := ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKeyCloudDrive(metaData, size)
		if err := fileCache.Delete(ctx, cacheKey); err != nil {
			return err
		}
		err := my_redis.DelThumbRedisKey(my_redis.GetFileName(cacheKey))
		if err != nil {
			return err
		}
	}

	return nil
}
