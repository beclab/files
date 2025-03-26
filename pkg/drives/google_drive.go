package drives

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/img"
	"files/pkg/preview"
	"files/pkg/redisutils"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/afero"
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

type GoogleDriveListParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type GoogleDriveListResponse struct {
	StatusCode string                             `json:"status_code"`
	FailReason *string                            `json:"fail_reason,omitempty"`
	Data       []*GoogleDriveListResponseFileData `json:"data,omitempty"`
	sync.Mutex
}

type GoogleDriveListResponseFileData struct {
	Path         string                           `json:"path"`
	Name         string                           `json:"name"`
	Size         int64                            `json:"size"`
	FileSize     int64                            `json:"fileSize"`
	Extension    string                           `json:"extension"`
	Modified     time.Time                        `json:"modified"`
	Mode         string                           `json:"mode"`
	IsDir        bool                             `json:"isDir"`
	IsSymlink    bool                             `json:"isSymlink"`
	Type         string                           `json:"type"`
	Meta         *GoogleDriveListResponseFileMeta `json:"meta,omitempty"`
	CanDownload  bool                             `json:"canDownload"`
	CanExport    bool                             `json:"canExport"`
	ExportSuffix string                           `json:"exportSuffix"`
	IdPath       string                           `json:"id_path,omitempty"`
}

type GoogleDriveListResponseFileMeta struct {
	Capabilities                 *GoogleDriveListResponseCapabilities      `json:"capabilities,omitempty"`
	CopyRequiresWriterPermission bool                                      `json:"copyRequiresWriterPermission"`
	CreatedTime                  time.Time                                 `json:"createdTime"`
	ExplicitlyTrashed            bool                                      `json:"explicitlyTrashed"`
	FileExtension                *string                                   `json:"fileExtension,omitempty"`
	FullFileExtension            *string                                   `json:"fullFileExtension,omitempty"`
	HasThumbnail                 bool                                      `json:"hasThumbnail"`
	HeadRevisionId               *string                                   `json:"headRevisionId,omitempty"`
	IconLink                     string                                    `json:"iconLink"`
	ID                           string                                    `json:"id"`
	IsAppAuthorized              bool                                      `json:"isAppAuthorized"`
	Kind                         string                                    `json:"kind"`
	LastModifyingUser            *GoogleDriveListResponseUser              `json:"lastModifyingUser,omitempty"`
	LinkShareMetadata            *GoogleDriveListResponseLinkShareMetadata `json:"linkShareMetadata,omitempty"`
	MD5Checksum                  *string                                   `json:"md5Checksum,omitempty"`
	MimeType                     string                                    `json:"mimeType"`
	ModifiedByMe                 bool                                      `json:"modifiedByMe"`
	ModifiedTime                 time.Time                                 `json:"modifiedTime"`
	Name                         string                                    `json:"name"`
	OriginalFilename             *string                                   `json:"originalFilename,omitempty"`
	OwnedByMe                    bool                                      `json:"ownedByMe"`
	Owners                       []*GoogleDriveListResponseUser            `json:"owners,omitempty"`
	QuotaBytesUsed               string                                    `json:"quotaBytesUsed"`
	SHA1Checksum                 *string                                   `json:"sha1Checksum,omitempty"`
	SHA256Checksum               *string                                   `json:"sha256Checksum,omitempty"`
	Shared                       bool                                      `json:"shared"`
	SharedWithMeTime             *time.Time                                `json:"sharedWithMeTime,omitempty"`
	Size                         *string                                   `json:"size,omitempty"`
	Spaces                       []string                                  `json:"spaces"`
	Starred                      bool                                      `json:"starred"`
	ThumbnailLink                *string                                   `json:"thumbnailLink,omitempty"`
	ThumbnailVersion             string                                    `json:"thumbnailVersion"`
	Title                        *string                                   `json:"title,omitempty"`
	Trashed                      bool                                      `json:"trashed"`
	Version                      string                                    `json:"version"`
	ViewedByMe                   bool                                      `json:"viewedByMe"`
	ViewedByMeTime               time.Time                                 `json:"viewedByMeTime"`
	ViewersCanCopyContent        bool                                      `json:"viewersCanCopyContent"`
	WebContentLink               *string                                   `json:"webContentLink,omitempty"`
	WebViewLink                  string                                    `json:"webViewLink"`
	WritersCanShare              bool                                      `json:"writersCanShare"`
}

type GoogleDriveListResponseCapabilities struct {
	CanAcceptOwnership                    bool `json:"canAcceptOwnership"`
	CanAddChildren                        bool `json:"canAddChildren"`
	CanAddMyDriveParent                   bool `json:"canAddMyDriveParent"`
	CanChangeCopyRequiresWriterPermission bool `json:"canChangeCopyRequiresWriterPermission"`
	CanChangeSecurityUpdateEnabled        bool `json:"canChangeSecurityUpdateEnabled"`
	CanChangeViewersCanCopyContent        bool `json:"canChangeViewersCanCopyContent"`
	CanComment                            bool `json:"canComment"`
	CanCopy                               bool `json:"canCopy"`
	CanDelete                             bool `json:"canDelete"`
	CanDownload                           bool `json:"canDownload"`
	CanEdit                               bool `json:"canEdit"`
	CanListChildren                       bool `json:"canListChildren"`
	CanModifyContent                      bool `json:"canModifyContent"`
	CanModifyContentRestriction           bool `json:"canModifyContentRestriction"`
	CanModifyEditorContentRestriction     bool `json:"canModifyEditorContentRestriction"`
	CanModifyLabels                       bool `json:"canModifyLabels"`
	CanModifyOwnerContentRestriction      bool `json:"canModifyOwnerContentRestriction"`
	CanMoveChildrenWithinDrive            bool `json:"canMoveChildrenWithinDrive"`
	CanMoveItemIntoTeamDrive              bool `json:"canMoveItemIntoTeamDrive"`
	CanMoveItemOutOfDrive                 bool `json:"canMoveItemOutOfDrive"`
	CanMoveItemWithinDrive                bool `json:"canMoveItemWithinDrive"`
	CanReadLabels                         bool `json:"canReadLabels"`
	CanReadRevisions                      bool `json:"canReadRevisions"`
	CanRemoveChildren                     bool `json:"canRemoveChildren"`
	CanRemoveContentRestriction           bool `json:"canRemoveContentRestriction"`
	CanRemoveMyDriveParent                bool `json:"canRemoveMyDriveParent"`
	CanRename                             bool `json:"canRename"`
	CanShare                              bool `json:"canShare"`
	CanTrash                              bool `json:"canTrash"`
	CanUntrash                            bool `json:"canUntrash"`
}

type GoogleDriveListResponseUser struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Kind         string `json:"kind"`
	Me           bool   `json:"me"`
	PermissionID string `json:"permissionId"`
}

type GoogleDriveListResponseLinkShareMetadata struct {
	SecurityUpdateEligible bool `json:"securityUpdateEligible"`
	SecurityUpdateEnabled  bool `json:"securityUpdateEnabled"`
}

type GoogleDriveMetaResponse struct {
	Data       GoogleDriveMetaData `json:"data"`
	FailReason *string             `json:"fail_reason,omitempty"`
	StatusCode string              `json:"status_code"`
}

type GoogleDriveMetaData struct {
	ID           string                   `json:"id"`
	Extension    string                   `json:"extension"`
	FileSize     int64                    `json:"fileSize"`
	IsDir        bool                     `json:"isDir"`
	IsSymlink    bool                     `json:"isSymlink"`
	Meta         *GoogleDriveMetaFileMeta `json:"meta"`
	Mode         string                   `json:"mode"`
	Modified     time.Time                `json:"modified"`
	Name         string                   `json:"name"`
	Path         string                   `json:"path"`
	Size         int64                    `json:"size"`
	Type         string                   `json:"type"`
	CanDownload  bool                     `json:"canDownload"`
	CanExport    bool                     `json:"canExport"`
	ExportSuffix string                   `json:"exportSuffix"`
}

type GoogleDriveMetaFileMeta struct {
	Capabilities                 GoogleDriveMetaCapabilities      `json:"capabilities"`
	CopyRequiresWriterPermission bool                             `json:"copyRequiresWriterPermission"`
	CreatedTime                  time.Time                        `json:"createdTime"`
	ExplicitlyTrashed            bool                             `json:"explicitlyTrashed"`
	FileExtension                *string                          `json:"fileExtension,omitempty"`
	FullFileExtension            *string                          `json:"fullFileExtension,omitempty"`
	HasThumbnail                 bool                             `json:"hasThumbnail"`
	HeadRevisionId               *string                          `json:"headRevisionId,omitempty"`
	IconLink                     string                           `json:"iconLink"`
	ID                           string                           `json:"id"`
	IsAppAuthorized              bool                             `json:"isAppAuthorized"`
	Kind                         string                           `json:"kind"`
	LastModifyingUser            GoogleDriveMetaUser              `json:"lastModifyingUser"`
	LinkShareMetadata            GoogleDriveMetaLinkShareMetadata `json:"linkShareMetadata"`
	MD5Checksum                  *string                          `json:"md5Checksum,omitempty"`
	MIMEType                     string                           `json:"mimeType"`
	ModifiedByMe                 bool                             `json:"modifiedByMe"`
	ModifiedTime                 time.Time                        `json:"modifiedTime"`
	Name                         string                           `json:"name"`
	OriginalFilename             *string                          `json:"originalFilename,omitempty"`
	OwnedByMe                    bool                             `json:"ownedByMe"`
	Owners                       []GoogleDriveMetaUser            `json:"owners"`
	QuotaBytesUsed               string                           `json:"quotaBytesUsed"`
	SHA1Checksum                 *string                          `json:"sha1Checksum,omitempty"`
	SHA256Checksum               *string                          `json:"sha256Checksum,omitempty"`
	Shared                       bool                             `json:"shared"`
	SharedWithMeTime             *time.Time                       `json:"sharedWithMeTime,omitempty"`
	Size                         *string                          `json:"size,omitempty"`
	Spaces                       []string                         `json:"spaces"`
	Starred                      bool                             `json:"starred"`
	ThumbnailLink                *string                          `json:"thumbnailLink,omitempty"`
	ThumbnailVersion             string                           `json:"thumbnailVersion"`
	Title                        *string                          `json:"title,omitempty"`
	Trashed                      bool                             `json:"trashed"`
	Version                      string                           `json:"version"`
	ViewedByMe                   bool                             `json:"viewedByMe"`
	ViewedByMeTime               time.Time                        `json:"viewedByMeTime"`
	ViewersCanCopyContent        bool                             `json:"viewersCanCopyContent"`
	WebContentLink               *string                          `json:"webContentLink,omitempty"`
	WebViewLink                  string                           `json:"webViewLink"`
	WritersCanShare              bool                             `json:"writersCanShare"`
}

type GoogleDriveMetaCapabilities struct {
	CanAcceptOwnership                    bool `json:"canAcceptOwnership"`
	CanAddChildren                        bool `json:"canAddChildren"`
	CanAddMyDriveParent                   bool `json:"canAddMyDriveParent"`
	CanChangeCopyRequiresWriterPermission bool `json:"canChangeCopyRequiresWriterPermission"`
	CanChangeSecurityUpdateEnabled        bool `json:"canChangeSecurityUpdateEnabled"`
	CanChangeViewersCanCopyContent        bool `json:"canChangeViewersCanCopyContent"`
	CanComment                            bool `json:"canComment"`
	CanCopy                               bool `json:"canCopy"`
	CanDelete                             bool `json:"canDelete"`
	CanDownload                           bool `json:"canDownload"`
	CanEdit                               bool `json:"canEdit"`
	CanListChildren                       bool `json:"canListChildren"`
	CanModifyContent                      bool `json:"canModifyContent"`
	CanModifyContentRestriction           bool `json:"canModifyContentRestriction"`
	CanModifyEditorContentRestriction     bool `json:"canModifyEditorContentRestriction"`
	CanModifyLabels                       bool `json:"canModifyLabels"`
	CanModifyOwnerContentRestriction      bool `json:"canModifyOwnerContentRestriction"`
	CanMoveChildrenWithinDrive            bool `json:"canMoveChildrenWithinDrive"`
	CanMoveItemIntoTeamDrive              bool `json:"canMoveItemIntoTeamDrive"`
	CanMoveItemOutOfDrive                 bool `json:"canMoveItemOutOfDrive"`
	CanMoveItemWithinDrive                bool `json:"canMoveItemWithinDrive"`
	CanReadLabels                         bool `json:"canReadLabels"`
	CanReadRevisions                      bool `json:"canReadRevisions"`
	CanRemoveChildren                     bool `json:"canRemoveChildren"`
	CanRemoveContentRestriction           bool `json:"canRemoveContentRestriction"`
	CanRemoveMyDriveParent                bool `json:"canRemoveMyDriveParent"`
	CanRename                             bool `json:"canRename"`
	CanShare                              bool `json:"canShare"`
	CanTrash                              bool `json:"canTrash"`
	CanUntrash                            bool `json:"canUntrash"`
}

type GoogleDriveMetaUser struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Kind         string `json:"kind"`
	Me           bool   `json:"me"`
	PermissionID string `json:"permissionId"`
}

type GoogleDriveMetaLinkShareMetadata struct {
	SecurityUpdateEligible bool `json:"securityUpdateEligible"`
	SecurityUpdateEnabled  bool `json:"securityUpdateEnabled"`
}

type GoogleDrivePostParam struct {
	ParentPath string `json:"parent_path"`
	FolderName string `json:"folder_name"`
	Drive      string `json:"drive"`
	Name       string `json:"name"`
}

type GoogleDrivePostResponseFileMeta struct {
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

type GoogleDrivePostResponseFileData struct {
	Extension string                          `json:"extension"`
	FileSize  int64                           `json:"fileSize"`
	IsDir     bool                            `json:"isDir"`
	IsSymlink bool                            `json:"isSymlink"`
	Meta      GoogleDrivePostResponseFileMeta `json:"meta"`
	Mode      string                          `json:"mode"`
	Modified  string                          `json:"modified"`
	Name      string                          `json:"name"`
	Path      string                          `json:"path"`
	Size      int64                           `json:"size"`
	Type      string                          `json:"type"`
}

type GoogleDrivePostResponse struct {
	Data       GoogleDrivePostResponseFileData `json:"data"`
	FailReason *string                         `json:"fail_reason,omitempty"`
	StatusCode string                          `json:"status_code"`
}

type GoogleDrivePatchParam struct {
	Path        string `json:"path"`
	NewFileName string `json:"new_file_name"`
	Drive       string `json:"drive"`
	Name        string `json:"name"`
}

type GoogleDriveDeleteParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type GoogleDriveCopyFileParam struct {
	CloudFilePath     string `json:"cloud_file_path"`
	NewCloudDirectory string `json:"new_cloud_directory"`
	NewCloudFileName  string `json:"new_cloud_file_name"`
	Drive             string `json:"drive"`
	Name              string `json:"name"`
}

type GoogleDriveMoveFileParam struct {
	CloudFilePath     string `json:"cloud_file_path"`
	NewCloudDirectory string `json:"new_cloud_directory"`
	Drive             string `json:"drive"`
	Name              string `json:"name"`
}

type GoogleDriveDownloadFileParam struct {
	LocalFolder   string `json:"local_folder"`
	CloudFilePath string `json:"cloud_file_path"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
	LocalFileName string `json:"local_file_name,omitempty"`
}

type GoogleDriveUploadFileParam struct {
	ParentPath    string `json:"parent_path"`
	LocalFilePath string `json:"local_file_path"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
}

type GoogleDriveTaskParameter struct {
	Drive         string `json:"drive"`
	LocalFilePath string `json:"local_file_path"`
	Name          string `json:"name"`
	ParentPath    string `json:"parent_path"`
}

type GoogleDriveTaskPauseInfo struct {
	FileSize  int64  `json:"file_size"`
	Location  string `json:"location"`
	NextStart int64  `json:"next_start"`
}

type GoogleDriveTaskResultData struct {
	FileInfo                 *GoogleDriveListResponseFileData `json:"file_info,omitempty"`
	UploadFirstOperationTime int64                            `json:"upload_first_operation_time"`
}

type GoogleDriveTaskData struct {
	ID            string                     `json:"id"`
	TaskType      string                     `json:"task_type"`
	Status        string                     `json:"status"`
	Progress      float64                    `json:"progress"`
	TaskParameter GoogleDriveTaskParameter   `json:"task_parameter"`
	PauseInfo     *GoogleDriveTaskPauseInfo  `json:"pause_info"`
	ResultData    *GoogleDriveTaskResultData `json:"result_data"`
	UserName      string                     `json:"user_name"`
	DriverName    string                     `json:"driver_name"`
	FailedReason  string                     `json:"failed_reason"`
	WorkerName    string                     `json:"worker_name"`
	CreatedAt     int64                      `json:"created_at"`
	UpdatedAt     int64                      `json:"updated_at"`
}

type GoogleDriveTaskResponse struct {
	StatusCode string              `json:"status_code"`
	FailReason string              `json:"fail_reason"`
	Data       GoogleDriveTaskData `json:"data"`
}

type GoogleDriveTaskQueryParam struct {
	TaskIds []string `json:"task_ids"`
}

type GoogleDriveTaskQueryResponse struct {
	StatusCode string                `json:"status_code"`
	FailReason string                `json:"fail_reason"`
	Data       []GoogleDriveTaskData `json:"data"`
}

func GetGoogleDriveMetadata(src string, w http.ResponseWriter, r *http.Request) (*GoogleDriveMetaData, error) {
	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	param := GoogleDriveListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return nil, err
	}
	klog.Infoln("Google Drive List Params:", string(jsonBody))
	respBody, err := GoogleDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, true)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return nil, err
	}

	var bodyJson GoogleDriveMetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		klog.Error(err)
		return nil, err
	}
	return &bodyJson.Data, nil
}

type GoogleDriveIdFocusedMetaInfos struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	IsDir        bool   `json:"is_dir"`
	CanDownload  bool   `json:"canDownload"`
	CanExport    bool   `json:"canExport"`
	ExportSuffix string `json:"exportSuffix"`
}

func GetGoogleDriveIdFocusedMetaInfos(src string, w http.ResponseWriter, r *http.Request) (info *GoogleDriveIdFocusedMetaInfos, err error) {
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	info = nil
	err = nil

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)
	if strings.Index(pathId, "/") != -1 {
		err = e.New("PathId Parse Error")
		return
	}

	param := GoogleDriveListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return
	}
	klog.Infoln("Google Drive Meta Params:", string(jsonBody))
	respBody, err := GoogleDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, true)
	if err != nil {
		klog.Errorln("Error calling drive/get_file_meta_data:", err)
		return
	}

	var bodyJson GoogleDriveMetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		klog.Error(err)
		return
	}

	if bodyJson.StatusCode == "FAIL" {
		err = e.New(*bodyJson.FailReason)
		return
	}

	info = &GoogleDriveIdFocusedMetaInfos{
		ID:           pathId,
		Path:         bodyJson.Data.Path,
		Name:         bodyJson.Data.Name,
		Size:         bodyJson.Data.FileSize,
		IsDir:        bodyJson.Data.IsDir,
		CanDownload:  bodyJson.Data.CanDownload,
		CanExport:    bodyJson.Data.CanExport,
		ExportSuffix: bodyJson.Data.ExportSuffix,
	}
	if info.Path == "/My Drive" {
		info.Name = "/"
	}
	return
}

func generateGoogleDriveFilesData(body []byte, stopChan <-chan struct{}, dataChan chan<- string, w http.ResponseWriter, r *http.Request, param GoogleDriveListParam) {
	defer close(dataChan)

	var bodyJson GoogleDriveListResponse
	if err := json.Unmarshal(body, &bodyJson); err != nil {
		klog.Error(err)
		return
	}

	var A []*GoogleDriveListResponseFileData
	bodyJson.Lock()
	A = append(A, bodyJson.Data...)
	bodyJson.Unlock()

	for len(A) > 0 {
		klog.Infoln("len(A): ", len(A))
		firstItem := A[0]
		klog.Infoln("firstItem Path: ", firstItem.Path)
		klog.Infoln("firstItem Name:", firstItem.Name)

		if firstItem.IsDir {
			pathId := firstItem.Meta.ID
			firstParam := GoogleDriveListParam{
				Path:  pathId,
				Drive: param.Drive,
				Name:  param.Name,
			}
			klog.Infoln("firstParam pathId:", pathId)
			firstJsonBody, err := json.Marshal(firstParam)
			if err != nil {
				klog.Errorln("Error marshalling JSON:", err)
				return
			}
			var firstRespBody []byte
			firstRespBody, err = GoogleDriveCall("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson GoogleDriveListResponse
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

func streamGoogleDriveFiles(w http.ResponseWriter, r *http.Request, body []byte, param GoogleDriveListParam) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateGoogleDriveFilesData(body, stopChan, dataChan, w, r, param)

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

func CopyGoogleDriveSingleFile(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPathId, srcFilename := ParseGoogleDrivePath(src)
	klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		klog.Infoln("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPathId, dstFilename := ParseGoogleDrivePath(dst)
	klog.Infoln("dstDrive:", dstDrive, "dstName:", dstName, "dstPathId:", dstPathId, "dstFilename:", dstFilename)
	if dstPathId == "" || dstFilename == "" {
		klog.Infoln("Dst parse failed.")
		return nil
	}
	dstFilename = strings.TrimSuffix(dstFilename, "/")

	param := GoogleDriveCopyFileParam{
		CloudFilePath:     srcPathId,   // id of "path/to/cloud/file.txt",
		NewCloudDirectory: dstPathId,   // id of "new/cloud/directory",
		NewCloudFileName:  dstFilename, // "new_file_name.txt",
		Drive:             dstDrive,    // "my_drive",
		Name:              dstName,     // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Copy File Params:", string(jsonBody))
	_, err = GoogleDriveCall("/drive/copy_file", "POST", jsonBody, w, r, true)
	if err != nil {
		klog.Errorln("Error calling drive/copy_file:", err)
		return err
	}
	return nil
}

func CopyGoogleDriveFolder(src, dst string, w http.ResponseWriter, r *http.Request, srcPath string) error {
	srcDrive, srcName, srcPathId, srcFilename := ParseGoogleDrivePath(src)
	klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		klog.Infoln("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPathId, dstFilename := ParseGoogleDrivePath(dst)
	klog.Infoln("dstDrive:", dstDrive, "dstName:", dstName, "dstPathId:", dstPathId, "dstFilename:", dstFilename)
	if dstPathId == "" || dstFilename == "" {
		klog.Infoln("Dst parse failed.")
		return nil
	}
	dstFilename = strings.TrimSuffix(dstFilename, "/")

	param := GoogleDriveCopyFileParam{
		CloudFilePath:     srcPathId,   // id of "path/to/cloud/file.txt",
		NewCloudDirectory: dstPathId,   // id of "new/cloud/directory",
		NewCloudFileName:  dstFilename, // "new_file_name.txt",
		Drive:             dstDrive,    // "my_drive",
		Name:              dstName,     // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Copy File Params:", string(jsonBody))
	_, err = GoogleDriveCall("/drive/copy_file", "POST", jsonBody, w, r, true)
	if err != nil {
		klog.Errorln("Error calling drive/copy_file:", err)
		return err
	}
	return nil
	//srcDrive, srcName, srcPathId, srcFilename := ParseGoogleDrivePath(src)
	//klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	//if srcPathId == "" {
	//	klog.Infoln("Src parse failed.")
	//	return nil
	//}
	//dstDrive, dstName, dstPathId, dstFilename := ParseGoogleDrivePath(dst)
	//klog.Infoln("dstDrive:", dstDrive, "dstName:", dstName, "dstPathId:", dstPathId, "dstFilename:", dstFilename)
	//if dstPathId == "" || dstFilename == "" {
	//	klog.Infoln("Dst parse failed.")
	//	return nil
	//}
	//
	//var CopyTempGoogleDrivePathIdCache = make(map[string]string)
	//var recursivePath = srcPath
	//var recursivePathId = srcPathId
	//var A []*GoogleDriveListResponseFileData
	//for {
	//	klog.Infoln("len(A): ", len(A))
	//
	//	var isDir = true
	//	var firstItem *GoogleDriveListResponseFileData
	//	if len(A) > 0 {
	//		firstItem = A[0]
	//		recursivePathId = firstItem.Meta.ID
	//		recursivePath = firstItem.Path
	//		isDir = firstItem.IsDir
	//	}
	//
	//	if isDir {
	//		var parentPathId string
	//		var folderName string
	//		if srcPathId == recursivePathId {
	//			parentPathId = dstPathId
	//			folderName = dstFilename
	//		} else {
	//			parentPathId = CopyTempGoogleDrivePathIdCache[filepath.Dir(firstItem.Path)]
	//			folderName = filepath.Base(firstItem.Path)
	//		}
	//		postParam := GoogleDrivePostParam{
	//			ParentPath: parentPathId,
	//			FolderName: folderName,
	//			Drive:      srcDrive,
	//			Name:       srcName,
	//		}
	//		postJsonBody, err := json.Marshal(postParam)
	//		if err != nil {
	//			klog.Errorln("Error marshalling JSON:", err)
	//			return err
	//		}
	//		klog.Infoln("Google Drive Post Params:", string(postJsonBody))
	//		var postRespBody []byte
	//		postRespBody, err = GoogleDriveCall("/drive/create_folder", "POST", postJsonBody, w, r, true)
	//		if err != nil {
	//			klog.Errorln("Error calling drive/create_folder:", err)
	//			return err
	//		}
	//		var postBodyJson GoogleDrivePostResponse
	//		if err = json.Unmarshal(postRespBody, &postBodyJson); err != nil {
	//			klog.Error(err)
	//			return err
	//		}
	//		CopyTempGoogleDrivePathIdCache[recursivePath] = postBodyJson.Data.Meta.ID
	//
	//		// list it and get its sub folders and files
	//		firstParam := GoogleDriveListParam{
	//			Path:  recursivePathId,
	//			Drive: srcDrive,
	//			Name:  srcName,
	//		}
	//
	//		klog.Infoln("firstParam pathId:", recursivePathId)
	//		var firstJsonBody []byte
	//		firstJsonBody, err = json.Marshal(firstParam)
	//		if err != nil {
	//			klog.Errorln("Error marshalling JSON:", err)
	//			return err
	//		}
	//		var firstRespBody []byte
	//		firstRespBody, err = GoogleDriveCall("/drive/ls", "POST", firstJsonBody, w, r, true)
	//
	//		var firstBodyJson GoogleDriveListResponse
	//		if err = json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
	//			klog.Error(err)
	//			return err
	//		}
	//
	//		if len(A) == 0 {
	//			A = firstBodyJson.Data
	//		} else {
	//			A = append(firstBodyJson.Data, A[1:]...)
	//		}
	//	} else {
	//		if len(A) > 0 {
	//			klog.Infoln(CopyTempGoogleDrivePathIdCache)
	//			copyPathPrefix := "/Drive/google/" + srcName + "/"
	//			copySrc := copyPathPrefix + firstItem.Meta.ID + "/"
	//			parentPathId := CopyTempGoogleDrivePathIdCache[filepath.Dir(firstItem.Path)]
	//			copyDst := filepath.Join(copyPathPrefix + parentPathId, firstItem.Name)
	//			klog.Infoln("copySrc: ", copySrc)
	//			klog.Infoln("copyDst: ", copyDst)
	//			err := CopyGoogleDriveSingleFile(copySrc, copyDst, w, r)
	//			if err != nil {
	//				return err
	//			}
	//			A = A[1:]
	//		}
	//	}
	//	if len(A) == 0 {
	//		return nil
	//	}
	//}
}

func GoogleFileToBuffer(src, bufferFilePath, bufferFileName string, w http.ResponseWriter, r *http.Request) (string, error) {
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}
	srcDrive, srcName, srcPathId, srcFilename := ParseGoogleDrivePath(src)
	klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		klog.Infoln("Src parse failed.")
		return bufferFileName, nil
	}

	param := GoogleDriveDownloadFileParam{
		LocalFolder:   bufferFilePath,
		CloudFilePath: srcPathId,
		Drive:         srcDrive,
		Name:          srcName,
	}
	if bufferFileName != "" {
		param.LocalFileName = bufferFileName
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return bufferFileName, err
	}
	klog.Infoln("Download File Params:", string(jsonBody))

	var respBody []byte
	respBody, err = GoogleDriveCall("/drive/download_async", "POST", jsonBody, w, r, true)
	if err != nil {
		klog.Errorln("Error calling drive/download_async:", err)
		return bufferFileName, err
	}
	var respJson GoogleDriveTaskResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		klog.Error(err)
		return bufferFileName, err
	}
	taskId := respJson.Data.ID
	taskParam := GoogleDriveTaskQueryParam{
		TaskIds: []string{taskId},
	}
	taskJsonBody, err := json.Marshal(taskParam)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return bufferFileName, err
	}
	klog.Infoln("Task Params:", string(taskJsonBody))

	for {
		time.Sleep(1000 * time.Millisecond)
		var taskRespBody []byte
		taskRespBody, err = GoogleDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
		if err != nil {
			klog.Errorln("Error calling drive/download_async:", err)
			return bufferFileName, err
		}
		var taskRespJson GoogleDriveTaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			klog.Error(err)
			return bufferFileName, err
		}
		if len(taskRespJson.Data) == 0 {
			return bufferFileName, e.New("Task Info Not Found")
		}
		if taskRespJson.Data[0].Status != "Waiting" && taskRespJson.Data[0].Status != "InProgress" {
			if taskRespJson.Data[0].Status == "Completed" {
				return bufferFileName, nil
			}
			return bufferFileName, e.New(taskRespJson.Data[0].Status)
		}
	}
}

func GoogleBufferToFile(bufferFilePath, dst string, w http.ResponseWriter, r *http.Request) (int, error) {
	dstDrive, dstName, dstPathId, dstFilename := ParseGoogleDrivePath(dst)
	klog.Infoln("srcDrive:", dstDrive, "srcName:", dstName, "srcPathId:", dstPathId, "srcFilename:", dstFilename)
	if dstPathId == "" {
		klog.Infoln("Src parse failed.")
		return http.StatusBadRequest, nil
	}

	param := GoogleDriveUploadFileParam{
		ParentPath:    dstPathId,
		LocalFilePath: bufferFilePath,
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
	respBody, err = GoogleDriveCall("/drive/upload_async", "POST", jsonBody, w, r, true)
	if err != nil {
		klog.Errorln("Error calling drive/upload_async:", err)
		return common.ErrToStatus(err), err
	}
	var respJson GoogleDriveTaskResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		klog.Error(err)
		return common.ErrToStatus(err), err
	}
	taskId := respJson.Data.ID
	taskParam := GoogleDriveTaskQueryParam{
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
		taskRespBody, err = GoogleDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
		if err != nil {
			klog.Errorln("Error calling drive/download_async:", err)
			return common.ErrToStatus(err), err
		}
		var taskRespJson GoogleDriveTaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			klog.Error(err)
			return common.ErrToStatus(err), err
		}
		if len(taskRespJson.Data) == 0 {
			err = e.New("Task Info Not Found")
			return common.ErrToStatus(err), err
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

func MoveGoogleDriveFolderOrFiles(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPathId, _ := ParseGoogleDrivePath(src)
	_, _, dstPathId, _ := ParseGoogleDrivePath(dst)

	param := GoogleDriveMoveFileParam{
		CloudFilePath:     srcPathId,
		NewCloudDirectory: dstPathId,
		Drive:             srcDrive, // "my_drive",
		Name:              srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Google Drive Patch Params:", string(jsonBody))
	_, err = GoogleDriveCall("/drive/move_file", "POST", jsonBody, w, r, false)
	if err != nil {
		klog.Errorln("Error calling drive/move_file:", err)
		return err
	}
	return nil
}

func GoogleDriveCall(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return nil, os.ErrPermission
	}

	host := common.GetHost(r)
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

		req.Header = r.Header.Clone()
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err = client.Do(req)
		if err != nil {
			klog.Errorln("Error making request:", err)
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if i < maxRetries {
				time.Sleep(retryDelay)
				continue
			}

			var responseBody []byte
			if resp.Header.Get("Content-Encoding") == "gzip" {
				reader, err := gzip.NewReader(resp.Body)
				if err != nil {
					klog.Errorln("Error creating gzip reader:", err)
					return nil, err
				}
				defer reader.Close()
				responseBody, err = ioutil.ReadAll(reader)
			} else {
				responseBody, err = ioutil.ReadAll(resp.Body)
			}
			if err != nil {
				klog.Errorln("Error reading response body:", err)
				return nil, err
			}
			klog.Infof("Non-200 response status: %d, body: %s\n", resp.StatusCode, responseBody)
			return nil, fmt.Errorf("non-200 response status: %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			klog.Infoln("GoogleDrive Call BflResponse is not JSON format:", contentType)
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

func ParseGoogleDrivePath(path string) (drive, name, dir, filename string) {
	if strings.HasPrefix(path, "/Drive/google") {
		path = path[13:]
		drive = "google"
	}

	slashes := []int{}
	for i, char := range path {
		if char == '/' {
			slashes = append(slashes, i)
		}
	}

	if len(slashes) < 2 {
		klog.Infoln("Path does not contain enough slashes.")
		return drive, "", "", ""
	}

	name = path[1:slashes[1]]

	if len(slashes) == 2 {
		return drive, name, "/", path[slashes[1]+1:]
	}

	dir = path[slashes[1]+1 : slashes[2]]
	filename = strings.TrimPrefix(path[slashes[2]:], "/")

	if dir == "root" {
		dir = "/"
	}

	return drive, name, dir, filename
}

type GoogleDriveResourceService struct {
	BaseResourceService
}

func (rc *GoogleDriveResourceService) GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
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
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	param := GoogleDriveListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return common.ErrToStatus(err), err
	}
	klog.Infoln("Google Drive List Params:", string(jsonBody))
	if stream == 1 {
		var body []byte
		body, err = GoogleDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
		streamGoogleDriveFiles(w, r, body, param)
		return 0, nil
	}
	if meta == 1 {
		_, err = GoogleDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, false)
	} else {
		_, err = GoogleDriveCall("/drive/ls", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return common.ErrToStatus(err), err
	}
	return 0, nil
}

func (rc *GoogleDriveResourceService) DeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		_, status, err := ResourceDeleteGoogle(fileCache, r.URL.Path, w, r, true)
		return status, err
	}
}

func (rc *GoogleDriveResourceService) PostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	_, status, err := ResourcePostGoogle(r.URL.Path, w, r, true)
	return status, err
}

func (rc *GoogleDriveResourceService) PutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	// not public api for google drive, so it is not implemented
	return http.StatusNotImplemented, fmt.Errorf("google drive does not supoort editing files")
}

func (rc *GoogleDriveResourceService) PatchHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		return ResourcePatchGoogle(fileCache, w, r)
	}
}

func (rs *GoogleDriveResourceService) RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	src := r.URL.Path
	metaData, err := GetGoogleDriveMetadata(src, w, r)
	if err != nil {
		klog.Error(err)
		return common.ErrToStatus(err), err
	}
	if metaData.IsDir {
		return http.StatusNotImplemented, fmt.Errorf("doesn't support directory download for google drive now")
	}
	return RawFileHandlerGoogle(src, w, r, metaData, bflName)
}

func (rs *GoogleDriveResourceService) PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		vars := mux.Vars(r)

		previewSize, err := preview.ParsePreviewSize(vars["size"])
		if err != nil {
			return http.StatusBadRequest, err
		}
		path := "/" + vars["path"]

		return PreviewGetGoogle(w, r, previewSize, path, imgSvc, fileCache, enableThumbnails, resizePreview)
	}
}

func (rc *GoogleDriveResourceService) PasteSame(action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	switch action {
	case "copy":
		if !strings.HasSuffix(src, "/") {
			src += "/"
		}
		metaInfo, err := GetGoogleDriveIdFocusedMetaInfos(src, w, r)
		if err != nil {
			return err
		}

		if metaInfo.IsDir {
			return CopyGoogleDriveFolder(src, dst, w, r, metaInfo.Path)
		}
		return CopyGoogleDriveSingleFile(src, dst, w, r)
	case "rename":
		if !strings.HasSuffix(src, "/") {
			src += "/"
		}
		return MoveGoogleDriveFolderOrFiles(src, dst, w, r)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func (rs *GoogleDriveResourceService) PasteDirFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
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

	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	param := GoogleDriveListParam{
		Path:  pathId,
		Drive: srcDrive,
		Name:  srcName,
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return err
	}
	klog.Infoln("Google Drive List Params:", string(jsonBody))
	var respBody []byte
	respBody, err = GoogleDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return err
	}
	var bodyJson GoogleDriveListResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		klog.Error(err)
		return err
	}
	for _, item := range bodyJson.Data {
		fsrc := filepath.Join(filepath.Dir(strings.TrimSuffix(src, "/")), item.Meta.ID)
		fdst := filepath.Join(fdstBase, item.Name)
		klog.Infoln(fsrc, fdst)
		if item.IsDir {
			err = rs.PasteDirFrom(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			fdst += item.ExportSuffix
			err = rs.PasteFileFrom(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), item.FileSize, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *GoogleDriveResourceService) PasteDirTo(fs afero.Fs, src, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	respBody, _, err := ResourcePostGoogle(dst, w, r, true)
	var bodyJson GoogleDrivePostResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		klog.Error(err)
		return err
	}
	driveIdCache[src] = bodyJson.Data.Meta.ID
	if err != nil {
		return err
	}
	klog.Infof("~~~Temp log for Google Drive PasteDirTo: driveIdCache: %v", driveIdCache)
	return nil
}

func (rs *GoogleDriveResourceService) PasteFileFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
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

	srcInfo, err := GetGoogleDriveIdFocusedMetaInfos(src, w, r)
	bufferFilePath, err := GenerateBufferFolder(srcInfo.Path, bflName)
	if err != nil {
		return err
	}
	bufferFileName := common.RemoveSlash(srcInfo.Name) + srcInfo.ExportSuffix
	bufferPath = filepath.Join(bufferFilePath, bufferFileName)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)
	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return err
	}
	_, err = GoogleFileToBuffer(src, bufferFilePath, bufferFileName, w, r)
	if err != nil {
		return err
	}

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(bufferPath, srcType)
	}()

	// only srcType == google need this now
	rename := r.URL.Query().Get("rename") == "true"
	if rename && dstType != SrcTypeGoogle {
		dst = PasteAddVersionSuffix(dst, dstType, false, files.DefaultFs, w, r)
	}

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

func (rs *GoogleDriveResourceService) PasteFileTo(fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	klog.Infoln("Begin to paste!")
	klog.Infoln("dst: ", dst)
	status, err := GoogleBufferToFile(bufferPath, dst, w, r)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *GoogleDriveResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}

	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	metaInfo, err := GetGoogleDriveIdFocusedMetaInfos(src, w, r)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return nil, metaInfo.Size, 0755, metaInfo.IsDir, nil
}

func (rs *GoogleDriveResourceService) MoveDelete(fileCache fileutils.FileCache, src string, ctx context.Context, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	_, status, err := ResourceDeleteGoogle(fileCache, src, w, r, true)
	if status != http.StatusOK && status != 0 {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func ResourceDeleteGoogle(fileCache fileutils.FileCache, src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	klog.Infoln("src Path:", src)
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	param := GoogleDriveDeleteParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return nil, common.ErrToStatus(err), err
	}
	klog.Infoln("Google Drive List Params:", string(jsonBody))

	// delete thumbnails
	err = delThumbsGoogle(r.Context(), fileCache, src, w, r)
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	var respBody []byte = nil
	if returnResp {
		respBody, err = GoogleDriveCall("/drive/delete", "POST", jsonBody, w, r, true)
		klog.Infoln(string(respBody))
	} else {
		_, err = GoogleDriveCall("/drive/delete", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		klog.Errorln("Error calling drive/delete:", err)
		return nil, common.ErrToStatus(err), err
	}
	return respBody, 0, nil
}

func ResourcePostGoogle(src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	klog.Infoln("src Path:", src)
	src = strings.TrimSuffix(src, "/")

	srcDrive, srcName, pathId, srcNewName := ParseGoogleDrivePath(src)

	param := GoogleDrivePostParam{
		ParentPath: pathId,
		FolderName: srcNewName,
		Drive:      srcDrive, // "my_drive",
		Name:       srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return nil, common.ErrToStatus(err), err
	}
	klog.Infoln("Google Drive Post Params:", string(jsonBody))
	var respBody []byte = nil
	if returnResp {
		respBody, err = GoogleDriveCall("/drive/create_folder", "POST", jsonBody, w, r, true)
	} else {
		_, err = GoogleDriveCall("/drive/create_folder", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		klog.Errorln("Error calling drive/create_folder:", err)
		return respBody, common.ErrToStatus(err), err
	}
	return respBody, 0, nil
}

func ResourcePatchGoogle(fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) (int, error) {
	src := r.URL.Path
	dst := r.URL.Query().Get("destination")
	dst, err := common.UnescapeURLIfEscaped(dst)

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)
	_, _, _, dstFilename := ParseGoogleDrivePath(dst)

	param := GoogleDrivePatchParam{
		Path:        pathId,
		NewFileName: dstFilename,
		Drive:       srcDrive, // "my_drive",
		Name:        srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		klog.Errorln("Error marshalling JSON:", err)
		return common.ErrToStatus(err), err
	}
	klog.Infoln("Google Drive Patch Params:", string(jsonBody))

	// delete thumbnails
	err = delThumbsGoogle(r.Context(), fileCache, src, w, r)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	_, err = GoogleDriveCall("/drive/rename", "POST", jsonBody, w, r, false)
	if err != nil {
		klog.Errorln("Error calling drive/rename:", err)
		return common.ErrToStatus(err), err
	}
	return 0, nil
}

func setContentDispositionGoogle(w http.ResponseWriter, r *http.Request, fileName string) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(fileName))
	}
}

func previewCacheKeyGoogle(f *GoogleDriveMetaData, previewSize preview.PreviewSize) string {
	return fmt.Sprintf("%x%x%x", f.ID, f.Modified.Unix(), previewSize)
}

func createPreviewGoogle(w http.ResponseWriter, r *http.Request, src string, imgSvc preview.ImgService, fileCache fileutils.FileCache,
	file *GoogleDriveMetaData, previewSize preview.PreviewSize, bflName string) ([]byte, error) {
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
	bufferFileName := common.RemoveSlash(file.Name)
	bufferPath := filepath.Join(bufferFilePath, bufferFileName)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)
	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return nil, err
	}
	_, err = GoogleFileToBuffer(src, bufferFilePath, bufferFileName, w, r)
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
		cacheKey := previewCacheKeyGoogle(file, previewSize)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			klog.Errorf("failed to cache resized image: %v", err)
		}
	}()

	klog.Infoln("Begin to remove buffer")
	RemoveDiskBuffer(bufferPath, SrcTypeGoogle)
	return buf.Bytes(), nil
}

func RawFileHandlerGoogle(src string, w http.ResponseWriter, r *http.Request, file *GoogleDriveMetaData, bflName string) (int, error) {
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
	bufferFileName := common.RemoveSlash(file.Name)
	bufferPath := filepath.Join(bufferFilePath, bufferFileName)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)
	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	_, err = GoogleFileToBuffer(src, bufferFilePath, bufferFileName, w, r)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	defer fd.Close()

	setContentDispositionGoogle(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.Modified, fd)

	klog.Infoln("Begin to remove buffer")
	RemoveDiskBuffer(bufferPath, SrcTypeGoogle)
	return 0, nil
}

func handleImagePreviewGoogle(
	w http.ResponseWriter,
	r *http.Request,
	src string,
	imgSvc preview.ImgService,
	fileCache fileutils.FileCache,
	file *GoogleDriveMetaData,
	previewSize preview.PreviewSize,
	enableThumbnails, resizePreview bool,
) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return common.ErrToStatus(os.ErrPermission), os.ErrPermission
	}

	if (previewSize == preview.PreviewSizeBig && !resizePreview) ||
		(previewSize == preview.PreviewSizeThumb && !enableThumbnails) {
		return RawFileHandlerGoogle(src, w, r, file, bflName)
	}

	format, err := imgSvc.FormatFromExtension(path.Ext(strings.TrimSuffix(file.Name, "/")))
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return RawFileHandlerGoogle(src, w, r, file, bflName)
	}
	if err != nil {
		return common.ErrToStatus(err), err
	}

	cacheKey := previewCacheKeyGoogle(file, previewSize)
	klog.Infoln("cacheKey:", cacheKey)
	klog.Infoln("f.RealPath:", file.Path)
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	if !ok {
		resizedImage, err = createPreviewGoogle(w, r, src, imgSvc, fileCache, file, previewSize, bflName)
		if err != nil {
			return common.ErrToStatus(err), err
		}
	}

	err = redisutils.UpdateFileAccessTimeToRedis(redisutils.GetFileName(cacheKey))
	if err != nil {
		return common.ErrToStatus(err), err
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.Modified, bytes.NewReader(resizedImage))

	return 0, nil
}

func PreviewGetGoogle(w http.ResponseWriter, r *http.Request, previewSize preview.PreviewSize, path string,
	imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) (int, error) {
	src := path
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	metaData, err := GetGoogleDriveMetadata(src, w, r)
	if err != nil {
		klog.Error(err)
		return common.ErrToStatus(err), err
	}

	setContentDispositionGoogle(w, r, metaData.Name)

	if strings.HasPrefix(metaData.Type, "image") {
		return handleImagePreviewGoogle(w, r, src, imgSvc, fileCache, metaData, previewSize, enableThumbnails, resizePreview)
	} else {
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", metaData.Type)
	}
}

func delThumbsGoogle(ctx context.Context, fileCache fileutils.FileCache, src string, w http.ResponseWriter, r *http.Request) error {
	metaData, err := GetGoogleDriveMetadata(src, w, r)
	if err != nil {
		klog.Errorln("Error calling drive/get_file_meta_data:", err)
		return err
	}

	for _, previewSizeName := range preview.PreviewSizeNames() {
		size, _ := preview.ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKeyGoogle(metaData, size)
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
