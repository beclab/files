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

type Awss3ListParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type Awss3ListResponse struct {
	StatusCode string                       `json:"status_code"`
	FailReason *string                      `json:"fail_reason,omitempty"`
	Data       []*Awss3ListResponseFileData `json:"data"`
	sync.Mutex
}

type Awss3ListResponseFileData struct {
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
	Meta      *Awss3ListResponseFileMeta `json:"meta,omitempty"`
}

type Awss3ListResponseFileMeta struct {
	ETag         string  `json:"e_tag"`
	Key          string  `json:"key"`
	LastModified *string `json:"last_modified,omitempty"`
	Owner        *string `json:"owner,omitempty"`
	Size         int     `json:"size"`
	StorageClass string  `json:"storage_class"`
}

type Awss3MetaResponseMeta struct {
	ETag         string  `json:"e_tag"`
	Key          string  `json:"key"`
	LastModified *string `json:"last_modified,omitempty"`
	Owner        *string `json:"owner"`
	Size         int64   `json:"size"`
	StorageClass *string `json:"storage_class"`
}

type Awss3MetaResponseData struct {
	Path      string                `json:"path"`
	Name      string                `json:"name"`
	Size      int64                 `json:"size"`
	FileSize  int64                 `json:"fileSize"`
	Extension string                `json:"extension"`
	Modified  *string               `json:"modified,omitempty"`
	Mode      string                `json:"mode"`
	IsDir     bool                  `json:"isDir"`
	IsSymlink bool                  `json:"isSymlink"`
	Type      string                `json:"type"`
	Meta      Awss3MetaResponseMeta `json:"meta"`
}

type Awss3MetaResponse struct {
	StatusCode string                `json:"status_code"`
	FailReason *string               `json:"fail_reason"`
	Data       Awss3MetaResponseData `json:"data"`
}

type Awss3FocusedMetaInfos struct {
	Key   string `json:"key"`
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

type Awss3PostParam struct {
	ParentPath string `json:"parent_path"`
	FolderName string `json:"folder_name"`
	Drive      string `json:"drive"`
	Name       string `json:"name"`
}

type Awss3PostResponseFileMeta struct {
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

type Awss3PostResponseFileData struct {
	Extension string                    `json:"extension"`
	FileSize  int64                     `json:"fileSize"`
	IsDir     bool                      `json:"isDir"`
	IsSymlink bool                      `json:"isSymlink"`
	Meta      Awss3PostResponseFileMeta `json:"meta"`
	Mode      string                    `json:"mode"`
	Modified  string                    `json:"modified"`
	Name      string                    `json:"name"`
	Path      string                    `json:"path"`
	Size      int64                     `json:"size"`
	Type      string                    `json:"type"`
}

type Awss3PostResponse struct {
	Data       Awss3PostResponseFileData `json:"data"`
	FailReason *string                   `json:"fail_reason,omitempty"`
	StatusCode string                    `json:"status_code"`
}

type Awss3PatchParam struct {
	Path        string `json:"path"`
	NewFileName string `json:"new_file_name"`
	Drive       string `json:"drive"`
	Name        string `json:"name"`
}

type Awss3DeleteParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type Awss3CopyFileParam struct {
	CloudFilePath     string `json:"cloud_file_path"`
	NewCloudDirectory string `json:"new_cloud_directory"`
	NewCloudFileName  string `json:"new_cloud_file_name"`
	Drive             string `json:"drive"`
	Name              string `json:"name"`
}

type Awss3MoveFileParam struct {
	CloudFilePath     string `json:"cloud_file_path"`
	NewCloudDirectory string `json:"new_cloud_directory"`
	Drive             string `json:"drive"`
	Name              string `json:"name"`
}

type Awss3DownloadFileParam struct {
	LocalFolder   string `json:"local_folder"`
	CloudFilePath string `json:"cloud_file_path"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
}

type Awss3UploadFileParam struct {
	ParentPath    string `json:"parent_path"`
	LocalFilePath string `json:"local_file_path"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
}

type Awss3TaskParameter struct {
	Drive         string `json:"drive"`
	LocalFilePath string `json:"local_file_path"`
	Name          string `json:"name"`
	ParentPath    string `json:"parent_path"`
}

type Awss3TaskPauseInfo struct {
	FileSize  int64  `json:"file_size"`
	Location  string `json:"location"`
	NextStart int64  `json:"next_start"`
}

type Awss3TaskResultData struct {
	FileInfo                 *Awss3ListResponseFileData `json:"file_info,omitempty"`
	UploadFirstOperationTime int64                      `json:"upload_first_operation_time"`
}

type Awss3TaskData struct {
	ID            string               `json:"id"`
	TaskType      string               `json:"task_type"`
	Status        string               `json:"status"`
	Progress      float64              `json:"progress"`
	TaskParameter Awss3TaskParameter   `json:"task_parameter"`
	PauseInfo     *Awss3TaskPauseInfo  `json:"pause_info"`
	ResultData    *Awss3TaskResultData `json:"result_data"`
	UserName      string               `json:"user_name"`
	DriverName    string               `json:"driver_name"`
	FailedReason  string               `json:"failed_reason"`
	WorkerName    *string              `json:"worker_name,omitempty"`
	CreatedAt     *int64               `json:"created_at,omitempty"`
	UpdatedAt     *int64               `json:"updated_at,omitempty"`
}

type Awss3TaskResponse struct {
	StatusCode string        `json:"status_code"`
	FailReason string        `json:"fail_reason"`
	Data       Awss3TaskData `json:"data"`
}

type Awss3TaskQueryParam struct {
	TaskIds []string `json:"task_ids"`
}

type Awss3TaskQueryResponse struct {
	StatusCode string          `json:"status_code"`
	FailReason string          `json:"fail_reason"`
	Data       []Awss3TaskData `json:"data"`
}

func getAwss3Metadata(src string, w http.ResponseWriter, r *http.Request) (*Awss3MetaResponseData, error) {
	srcDrive, srcName, srcPath := parseAwss3Path(src, true)

	param := Awss3ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil, err
	}
	fmt.Println("Awss3 List Params:", string(jsonBody))
	respBody, err := Awss3Call("/drive/get_file_meta_data", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/ls:", err)
		return nil, err
	}

	var bodyJson Awss3MetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		fmt.Println(err)
		return nil, err
	}
	return &bodyJson.Data, nil
}

func getAwss3FocusedMetaInfos(src string, w http.ResponseWriter, r *http.Request) (info *Awss3FocusedMetaInfos, err error) {
	src = strings.TrimSuffix(src, "/")
	info = nil
	err = nil

	srcDrive, srcName, srcPath := parseAwss3Path(src, true)

	param := Awss3ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	fmt.Println("Awss3 Awss3MetaResponseMeta Params:", string(jsonBody))
	respBody, err := Awss3Call("/drive/get_file_meta_data", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/get_file_meta_data:", err)
		return
	}

	var bodyJson Awss3MetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		fmt.Println(err)
		return
	}

	if bodyJson.StatusCode == "FAIL" {
		err = e.New(*bodyJson.FailReason)
		return
	}

	info = &Awss3FocusedMetaInfos{
		Key:   bodyJson.Data.Meta.Key,
		Path:  bodyJson.Data.Path,
		Name:  bodyJson.Data.Name,
		Size:  bodyJson.Data.FileSize,
		IsDir: bodyJson.Data.IsDir,
	}
	return
}

func generateAwss3FilesData(body []byte, stopChan <-chan struct{}, dataChan chan<- string,
	w http.ResponseWriter, r *http.Request, param Awss3ListParam) {
	defer close(dataChan)

	var bodyJson Awss3ListResponse
	if err := json.Unmarshal(body, &bodyJson); err != nil {
		fmt.Println(err)
		return
	}

	var A []*Awss3ListResponseFileData
	bodyJson.Lock()
	A = append(A, bodyJson.Data...)
	bodyJson.Unlock()

	for len(A) > 0 {
		fmt.Println("len(A): ", len(A))
		firstItem := A[0]
		fmt.Println("firstItem Path: ", firstItem.Path)
		fmt.Println("firstItem Name:", firstItem.Name)

		if firstItem.IsDir {
			firstParam := Awss3ListParam{
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
			firstRespBody, err = Awss3Call("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson Awss3ListResponse
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

func streamAwss3Files(w http.ResponseWriter, r *http.Request, body []byte, param Awss3ListParam) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateAwss3FilesData(body, stopChan, dataChan, w, r, param)

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

func copyAwss3SingleFile(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPath := parseAwss3Path(src, true)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		fmt.Println("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPath := parseAwss3Path(dst, true)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	dstDir, dstFilename := path.Split(dstPath)
	if dstDir == "" || dstFilename == "" {
		fmt.Println("Dst parse failed.")
		return nil
	}
	// 填充数据
	param := Awss3CopyFileParam{
		CloudFilePath:     srcPath,     // id of "path/to/cloud/file.txt",
		NewCloudDirectory: dstDir,      // id of "new/cloud/directory",
		NewCloudFileName:  dstFilename, // "new_file_name.txt",
		Drive:             dstDrive,    // "my_drive",
		Name:              dstName,     // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println("Copy File Params:", string(jsonBody))
	_, err = Awss3Call("/drive/copy_file", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/copy_file:", err)
		return err
	}
	return nil
}

func copyAwss3Folder(src, dst string, w http.ResponseWriter, r *http.Request, srcPath, srcPathName string) error {
	srcDrive, srcName, srcPath := parseAwss3Path(src, true)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		fmt.Println("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPath := parseAwss3Path(dst, true)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	dstDir, dstFilename := path.Split(dstPath)
	if dstDir == "" || dstFilename == "" {
		fmt.Println("Dst parse failed.")
		return nil
	}

	var recursivePath = srcPath
	var A []*Awss3ListResponseFileData
	for {
		fmt.Println("len(A): ", len(A))

		var isDir = true
		var firstItem *Awss3ListResponseFileData
		if len(A) > 0 {
			firstItem = A[0]
			recursivePath = firstItem.Path
			isDir = firstItem.IsDir
		}

		if isDir {
			var parentPath string
			var folderName string
			if srcPath == recursivePath {
				parentPath = dstPath
				folderName = dstFilename
			} else {
				parentPath = filepath.Dir(firstItem.Path)
				folderName = filepath.Base(firstItem.Path)
			}
			postParam := Awss3PostParam{
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
			postRespBody, err = Awss3Call("/drive/create_folder", "POST", postJsonBody, w, r, true)
			if err != nil {
				fmt.Println("Error calling drive/create_folder:", err)
				return err
			}
			var postBodyJson Awss3PostResponse
			if err = json.Unmarshal(postRespBody, &postBodyJson); err != nil {
				fmt.Println(err)
				return err
			}

			firstParam := Awss3ListParam{
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
			firstRespBody, err = Awss3Call("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson Awss3ListResponse
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
				//fmt.Println(CopyTempGoogleDrivePathIdCache)
				copyPathPrefix := "/Drive/" + srcDrive + "/" + srcName + "/"
				copySrc := copyPathPrefix + firstItem.Path + "/"
				parentPath := filepath.Dir(firstItem.Path)
				copyDst := copyPathPrefix + parentPath + "/" + firstItem.Name
				fmt.Println("copySrc: ", copySrc)
				fmt.Println("copyDst: ", copyDst)
				err := copyAwss3SingleFile(copySrc, copyDst, w, r)
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

func awss3FileToBuffer(src, bufferFilePath string, w http.ResponseWriter, r *http.Request) error {
	src = strings.TrimSuffix(src, "/")
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}
	srcDrive, srcName, srcPath := parseAwss3Path(src, true)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		fmt.Println("Src parse failed.")
		return nil
	}

	param := Awss3DownloadFileParam{
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
	_, err = Awss3Call("/drive/download_async", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/download_async:", err)
		return err
	}

	var respJson Awss3TaskResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		fmt.Println(err)
		return err
	}
	taskId := respJson.Data.ID
	taskParam := Awss3TaskQueryParam{
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
		taskRespBody, err = Awss3Call("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/download_async:", err)
			return err
		}
		var taskRespJson Awss3TaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			fmt.Println(err)
			return err
		}
		if len(taskRespJson.Data) == 0 {
			return e.New("Task Info Not Found")
		}
		if taskRespJson.Data[0].Status != "Waiting" && taskRespJson.Data[0].Status != "InProgress" {
			if taskRespJson.Data[0].Status == "Completed" {
				return nil
			}
			return e.New(taskRespJson.Data[0].Status)
		}
	}
}

func awss3BufferToFile(bufferFilePath, dst string, w http.ResponseWriter, r *http.Request) (int, error) {
	dstDrive, dstName, dstPath := parseAwss3Path(dst, true)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	if dstPath == "" {
		fmt.Println("Src parse failed.")
		return http.StatusBadRequest, nil
	}

	param := Awss3UploadFileParam{
		ParentPath:    dstPath,
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
	respBody, err = Awss3Call("/drive/upload_async", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/upload_async:", err)
		return errToStatus(err), err
	}
	var respJson Awss3TaskResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		fmt.Println(err)
		return errToStatus(err), err
	}
	taskId := respJson.Data.ID
	taskParam := Awss3TaskQueryParam{
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
		taskRespBody, err = Awss3Call("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/upload_async:", err)
			return errToStatus(err), err
		}
		var taskRespJson Awss3TaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			fmt.Println(err)
			return errToStatus(err), err
		}
		if len(taskRespJson.Data) == 0 {
			err = e.New("Task Info Not Found")
			return errToStatus(err), err
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

func moveAwss3FolderOrFiles(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPath := parseAwss3Path(src, true)
	_, _, dstPath := parseAwss3Path(dst, true)

	param := Awss3MoveFileParam{
		CloudFilePath:     srcPath,
		NewCloudDirectory: dstPath,
		Drive:             srcDrive, // "my_drive",
		Name:              srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println("Awss3 Move File Params:", string(jsonBody))
	_, err = Awss3Call("/drive/move_file", "POST", jsonBody, w, r, false)
	if err != nil {
		fmt.Println("Error calling drive/move_file:", err)
		return err
	}
	return nil
}

func Awss3Call(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return nil, os.ErrPermission
	}

	authority := r.Header.Get("Authority")
	fmt.Println("*****Awss3 Call URL authority:", authority)
	host := r.Header.Get("Origin")
	if host == "" {
		host = getHost(w, r) // r.Header.Get("Origin")
	}
	fmt.Println("*****Awss3 Call URL referer:", host)
	if host == "" {
		host = "https://files." + bflName + ".olares.cn"
	}
	fmt.Println("*****Awss3 Call URL forced:", host)
	dstUrl := host + dst // "/api/resources%2FHome%2FDocuments%2F"

	fmt.Println("dstUrl:", dstUrl)

	var req *http.Request
	var err error
	if reqBodyJson != nil {
		req, err = http.NewRequest(method, dstUrl, bytes.NewBuffer(reqBodyJson))
	} else {
		req, err = http.NewRequest(method, dstUrl, nil)
	}

	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, err
	}

	// 设置请求头
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	defer resp.Body.Close()

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		fmt.Println("Awss3 Call Response is not JSON format:", contentType)
	}

	// 读取响应体
	var body []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		// 如果响应体被gzip压缩
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
		// 如果响应体没有被压缩
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return nil, err
		}
	}

	// 解析JSON
	var datas map[string]interface{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		fmt.Println("Error unmarshaling JSON response:", err)
		return nil, err
	}

	// 打印解析后的数据
	fmt.Println("Parsed JSON response:", datas)
	// 将解析后的JSON响应体转换为字符串（格式化输出）
	responseText, err := json.MarshalIndent(datas, "", "  ")
	if err != nil {
		http.Error(w, "Error marshaling JSON response to text: "+err.Error(), http.StatusInternalServerError)
		return nil, err
	}

	if returnResp {
		return responseText, nil
	}
	// 设置响应头并写入响应体
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(responseText))
	return nil, nil
}

func parseAwss3Path(src string, trimSuffix bool) (drive, name, path string) {
	//if strings.HasPrefix(src, "/Drive/awss3") {
	//	src = src[12:]
	//	drive = "awss3"
	//}

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

func resourceGetAwss3(w http.ResponseWriter, r *http.Request, stream int, meta int) (int, error) {
	src := r.URL.Path
	fmt.Println("src Path:", src)

	srcDrive, srcName, srcPath := parseAwss3Path(src, true)
	fmt.Println("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)

	param := Awss3ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Awss3 List Params:", string(jsonBody))
	if stream == 1 {
		var body []byte
		body, err = Awss3Call("/drive/ls", "POST", jsonBody, w, r, true)
		streamAwss3Files(w, r, body, param)
		return 0, nil
	}
	if meta == 1 {
		_, err = Awss3Call("/drive/get_file_meta_data", "POST", jsonBody, w, r, false)
	} else {
		_, err = Awss3Call("/drive/ls", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/ls:", err)
		return errToStatus(err), err
	}
	return 0, nil
}

//func splitPath(path string) (dir, name string) {
//	// 去掉结尾的"/"
//	trimmedPath := strings.TrimRight(path, "/")
//
//	// 查找最后一个"/"的位置
//	lastIndex := strings.LastIndex(trimmedPath, "/")
//
//	if lastIndex == -1 {
//		// 如果没有找到"/"，则dir为"/"，name为整个trimmedPath
//		return "/", trimmedPath
//	}
//
//	// 分割dir和name，注意这里dir不包括最后的"/"
//	dir = trimmedPath[:lastIndex+1] // 包括到最后一个"/"之前的部分
//	// 如果路径只有根目录和"/"，则name应为空
//	if lastIndex+1 == len(trimmedPath) {
//		name = ""
//	} else {
//		name = trimmedPath[lastIndex+1:]
//	}
//
//	// 如果dir只有一个"/"，则表示根目录
//	if dir == "/" {
//		// 特殊处理根目录情况，此时name应为整个trimmedPath
//		name = strings.TrimPrefix(trimmedPath, "/")
//		dir = "/"
//	}
//
//	return dir, name
//}

func resourcePostAwss3(src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	fmt.Println("src Path:", src)

	srcDrive, srcName, srcPath := parseAwss3Path(src, true)
	fmt.Println("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)
	path, newName := path.Split(srcPath)

	param := Awss3PostParam{
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
	fmt.Println("Awss3 Post Params:", string(jsonBody))
	var respBody []byte = nil
	if returnResp {
		respBody, err = Awss3Call("/drive/create_folder", "POST", jsonBody, w, r, true)
	} else {
		_, err = Awss3Call("/drive/create_folder", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/create_folder:", err)
		return respBody, errToStatus(err), err
	}
	return respBody, 0, nil
}

func resourcePatchAwss3(fileCache FileCache, w http.ResponseWriter, r *http.Request) (int, error) {
	src := r.URL.Path
	dst := r.URL.Query().Get("destination")
	//action := r.URL.Query().Get("action")
	dst, err := url.QueryUnescape(dst)

	srcDrive, srcName, srcPath := parseAwss3Path(src, true)
	_, _, dstPath := parseAwss3Path(dst, true)
	_, dstFilename := path.Split(dstPath)

	param := Awss3PatchParam{
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
	fmt.Println("Awss3 Patch Params:", string(jsonBody))

	// del thumbnails for awss3
	err = delThumbsAwss3(r.Context(), fileCache, src, w, r)
	if err != nil {
		return errToStatus(err), err
	}

	_, err = Awss3Call("/drive/rename", "POST", jsonBody, w, r, false)
	if err != nil {
		fmt.Println("Error calling drive/rename:", err)
		return errToStatus(err), err
	}
	return 0, nil
}

func resourceDeleteAwss3(fileCache FileCache, src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	fmt.Println("src Path:", src)
	if strings.HasSuffix(src, "/") {
		src = strings.TrimSuffix(src, "/")
	}

	srcDrive, srcName, srcPath := parseAwss3Path(src, true)

	param := Awss3DeleteParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil, errToStatus(err), err
	}
	fmt.Println("Awss3 Delete Params:", string(jsonBody))

	// del thumbnails for awss3
	err = delThumbsAwss3(r.Context(), fileCache, src, w, r)
	if err != nil {
		return nil, errToStatus(err), err
	}

	var respBody []byte = nil
	if returnResp {
		respBody, err = Awss3Call("/drive/delete", "POST", jsonBody, w, r, true)
		fmt.Println(string(respBody))
	} else {
		_, err = Awss3Call("/drive/delete", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/delete:", err)
		return nil, errToStatus(err), err
	}
	return respBody, 0, nil
}

// TODO: can be one
func setContentDispositionAwss3(w http.ResponseWriter, r *http.Request, fileName string) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// As per RFC6266 section 4.3
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

func previewCacheKeyAwss3(f *Awss3MetaResponseData, previewSize PreviewSize) string {
	//return stringMD5(fmt.Sprintf("%s%d%s", f.Path, f.Modified.Unix(), previewSize))
	return fmt.Sprintf("%x%x%x", f.Path, ParseTimeString(f.Modified).Unix(), previewSize)
}

func createPreviewAwss3(w http.ResponseWriter, r *http.Request, src string, imgSvc ImgService, fileCache FileCache,
	file *Awss3MetaResponseData, previewSize PreviewSize, bflName string) ([]byte, error) {
	fmt.Println("!!!!CreatePreview:", previewSize)

	var err error
	diskSize := file.Size
	if diskSize >= 4*1024*1024*1024 {
		fmt.Println("file size exceeds 4GB")
		return nil, e.New("file size exceeds 4GB") //os.ErrPermission
	}
	fmt.Println("Will reserve disk size: ", diskSize)
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
	err = awss3FileToBuffer(src, bufferFilePath, w, r)
	//bufferPath = filepath.Join(bufferFilePath, bufferFilename)
	//fmt.Println("Buffer file path: ", bufferFilePath)
	//fmt.Println("Buffer path: ", bufferPath)
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
		cacheKey := previewCacheKeyAwss3(file, previewSize)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			fmt.Printf("failed to cache resized image: %v", err)
		}
	}()

	fmt.Println("Begin to remove buffer")
	removeDiskBuffer(bufferPath, "awss3")
	return buf.Bytes(), nil
}

func rawFileHandlerAwss3(src string, w http.ResponseWriter, r *http.Request, file *Awss3MetaResponseData, bflName string) (int, error) {
	var err error
	diskSize := file.Size
	if diskSize >= 4*1024*1024*1024 {
		fmt.Println("file size exceeds 4GB")
		return http.StatusForbidden, e.New("file size exceeds 4GB") //os.ErrPermission
	}
	fmt.Println("Will reserve disk size: ", diskSize)
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
	err = awss3FileToBuffer(src, bufferFilePath, w, r)
	//bufferPath = filepath.Join(bufferFilePath, bufferFilename)
	//fmt.Println("Buffer file path: ", bufferFilePath)
	//fmt.Println("Buffer path: ", bufferPath)
	if err != nil {
		return errToStatus(err), err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return errToStatus(err), err
	}
	defer fd.Close()

	setContentDispositionAwss3(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, ParseTimeString(file.Modified), fd)

	fmt.Println("Begin to remove buffer")
	removeDiskBuffer(bufferPath, "awss3")
	return 0, nil
}

func handleImagePreviewAwss3(
	w http.ResponseWriter,
	r *http.Request,
	src string,
	imgSvc ImgService,
	fileCache FileCache,
	file *Awss3MetaResponseData,
	previewSize PreviewSize,
	enableThumbnails, resizePreview bool,
) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return errToStatus(os.ErrPermission), os.ErrPermission
	}

	if (previewSize == PreviewSizeBig && !resizePreview) ||
		(previewSize == PreviewSizeThumb && !enableThumbnails) {
		return rawFileHandlerAwss3(src, w, r, file, bflName)
	}

	format, err := imgSvc.FormatFromExtension(file.Extension)
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return rawFileHandlerAwss3(src, w, r, file, bflName)
	}
	if err != nil {
		return errToStatus(err), err
	}

	cacheKey := previewCacheKeyAwss3(file, previewSize)
	fmt.Println("cacheKey:", cacheKey)
	fmt.Println("f.RealPath:", file.Path)
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return errToStatus(err), err
	}
	if !ok {
		resizedImage, err = createPreviewAwss3(w, r, src, imgSvc, fileCache, file, previewSize, bflName)
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

func previewGetAwss3(w http.ResponseWriter, r *http.Request, previewSize PreviewSize, path string,
	imgSvc ImgService, fileCache FileCache, enableThumbnails, resizePreview bool) (int, error) {
	src := path
	if strings.HasSuffix(src, "/") {
		src = strings.TrimSuffix(src, "/")
	}

	metaData, err := getAwss3Metadata(src, w, r)
	if err != nil {
		fmt.Println(err)
		return errToStatus(err), err
	}

	setContentDispositionAwss3(w, r, metaData.Name)

	fileType := parser.MimeTypeByExtension(metaData.Name)
	if strings.HasPrefix(fileType, "image") {
		return handleImagePreviewAwss3(w, r, src, imgSvc, fileCache, metaData, previewSize, enableThumbnails, resizePreview)
	} else {
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", fileType)
	}
}

func delThumbsAwss3(ctx context.Context, fileCache FileCache, src string, w http.ResponseWriter, r *http.Request) error {
	metaData, err := getAwss3Metadata(src, w, r)
	if err != nil {
		fmt.Println("Error calling drive/get_file_meta_data:", err)
		return err
	}

	for _, previewSizeName := range PreviewSizeNames() {
		size, _ := ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKeyAwss3(metaData, size)
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
