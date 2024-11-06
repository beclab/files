package http

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	e "errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type GoogleDriveListParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

// GoogleDriveListResponse 定义了API响应的结构
type GoogleDriveListResponse struct {
	StatusCode string                             `json:"status_code"`
	FailReason *string                            `json:"fail_reason,omitempty"`
	Data       []*GoogleDriveListResponseFileData `json:"data,omitempty"`
	sync.Mutex
}

// GoogleDriveListResponseFileData 定义了文件或文件夹数据的结构
type GoogleDriveListResponseFileData struct {
	Path      string                           `json:"path"`
	Name      string                           `json:"name"`
	Size      int64                            `json:"size"`
	FileSize  int64                            `json:"fileSize"`
	Extension string                           `json:"extension"`
	Modified  time.Time                        `json:"modified"`
	Mode      string                           `json:"mode"`
	IsDir     bool                             `json:"isDir"`
	IsSymlink bool                             `json:"isSymlink"`
	Type      string                           `json:"type"`
	Meta      *GoogleDriveListResponseFileMeta `json:"meta,omitempty"`
}

// GoogleDriveListResponseFileMeta 定义了文件或文件夹的元数据结构
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

// GoogleDriveListResponseCapabilities 定义了文件或文件夹的功能权限
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

// GoogleDriveListResponseUser 定义了用户信息的结构
type GoogleDriveListResponseUser struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Kind         string `json:"kind"`
	Me           bool   `json:"me"`
	PermissionID string `json:"permissionId"`
}

// GoogleDriveListResponseLinkShareMetadata 定义了链接共享元数据的结构
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
	Extension string                  `json:"extension"`
	FileSize  int64                   `json:"fileSize"`
	IsDir     bool                    `json:"isDir"`
	IsSymlink bool                    `json:"isSymlink"`
	Meta      GoogleDriveMetaFileMeta `json:"meta"`
	Mode      string                  `json:"mode"`
	Modified  time.Time               `json:"modified"`
	Name      string                  `json:"name"`
	Path      string                  `json:"path"`
	Size      int64                   `json:"size"`
	Type      string                  `json:"type"`
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

var GoogleDrivePathIdCache = make(map[string]string)

func getHost(w http.ResponseWriter, r *http.Request) string {
	referer := r.Header.Get("Referer")
	if referer == "" {
		//fmt.Fprintf(w, "No Referer header found\n")
		return ""
	}

	slashCount := 0
	for i, char := range referer {
		if char == '/' {
			slashCount++
			if slashCount == 3 {
				basePart := referer[:i]
				//fmt.Fprintf(w, "Base part of Referer: %s\n", basePart)
				return basePart
			}
		}
	}

	//fmt.Fprintf(w, "Less than three slashes in Referer, using entire value: %s\n", referer)
	return ""
}

// Deprecated
// isDir == true if and only if it is definitely dir
func GoogleDrivePathToId(src string, w http.ResponseWriter, r *http.Request, isDir bool) (string, string, string, string, string, error) {
	srcDrive, srcName, srcDir, srcFilename := parseGoogleDrivePath(src)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcDir:", srcDir, "srcFilename:", srcFilename)

	if srcDir == "/" {
		return "/", srcDrive, srcName, "/", "", nil
	}

	var cacheKey string
	if srcFilename == "" || isDir {
		cacheKey = srcName + srcDir
	} else {
		cacheKey = srcName + srcDir + "/" + srcFilename
	}
	fmt.Println("cacheKey:", cacheKey)
	if len(GoogleDrivePathIdCache) == 0 {
		fmt.Println("GoogleDrivepathIdCache: empty!")
	}
	for key, value := range GoogleDrivePathIdCache {
		fmt.Printf("Key: %s, Value: %s\n", key, value)
	}
	if cachedPathId, ok := GoogleDrivePathIdCache[cacheKey]; ok {
		fmt.Println("Using cached pathId for", cacheKey, ":", cachedPathId)
		return cachedPathId, srcDrive, srcName, srcDir, srcFilename, nil
	}

	var pathId = "/"
	parts := strings.Split(srcDir, "/")
	if srcFilename != "" && !isDir {
		parts = append(parts, srcFilename)
	}
	var currentPath = ""

	for _, part := range parts {
		if part == "" {
			continue
		}
		currentPath += "/" + part
		subCacheKey := srcName + currentPath
		fmt.Println("subCacheKey:", subCacheKey)
		if len(GoogleDrivePathIdCache) == 0 {
			fmt.Println("GoogleDrivepathIdCache: empty!")
		}
		for key, value := range GoogleDrivePathIdCache {
			fmt.Printf("Key: %s, Value: %s\n", key, value)
		}
		if subCachePathId, ok := GoogleDrivePathIdCache[subCacheKey]; ok {
			fmt.Println("Using cached pathId for", subCacheKey, ":", subCachePathId)
			pathId = subCachePathId
			continue
		}

		param := GoogleDriveListParam{
			Path:  pathId,
			Drive: srcDrive, // "my_drive",
			Name:  srcName,  // "file_name",
		}
		// 将数据序列化为 JSON
		jsonBody, err := json.Marshal(param)
		if err != nil {
			fmt.Println("Error marshalling JSON:", err)
			return "", srcDrive, srcName, srcDir, srcFilename, err
		}
		fmt.Println("Google Drive List Params:", string(jsonBody))
		responseStr, err := GoogleDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/copy_file:", err)
			return "", srcDrive, srcName, srcDir, srcFilename, err
		}

		var response GoogleDriveListResponse
		err = json.Unmarshal([]byte(responseStr), &response)
		if err != nil {
			return "", srcDrive, srcName, srcDir, srcFilename, err
		}

		// Find the ID for the current path
		for _, item := range response.Data {
			fmt.Println("item.Path:", item.Path, " , currentPath:", currentPath)
			if item.Path == "/My Drive"+currentPath {
				pathId = item.Meta.ID
				GoogleDrivePathIdCache[subCacheKey] = pathId
				fmt.Println("Cached pathId for", subCacheKey, ":", pathId)
				fmt.Println("subCacheKey:", subCacheKey)
				if len(GoogleDrivePathIdCache) == 0 {
					fmt.Println("GoogleDrivepathIdCache: empty!")
				}
				for key, value := range GoogleDrivePathIdCache {
					fmt.Printf("Key: %s, Value: %s\n", key, value)
				}
				break
			}
			pathId = ""
		}

		if pathId == "" {
			return "", srcDrive, srcName, srcDir, srcFilename, fmt.Errorf("ID not found for path: %s", currentPath)
		}
	}

	// 在找到完整的 pathId 后，更新缓存
	//GoogleDrivePathIdCache[cacheKey] = pathId
	//fmt.Println("Cached pathId for", cacheKey, ":", pathId)

	return pathId, srcDrive, srcName, srcDir, srcFilename, nil
}

// deprecated
// only used when copy empty folder
func GoogleDriveIdToPath(srcDrive string, srcName string, pathId string, w http.ResponseWriter, r *http.Request) (path string, name string, err error) {
	err = nil
	if pathId == "/" {
		path = "/"
		name = ""
		return
	}

	// No cache reasonable, because renaming may not happen in our system
	//var cacheKey string
	//cacheKey = srcName + "/" + pathId
	//fmt.Println("cacheKey:", cacheKey)
	//if len(GoogleDrivePathIdCache) == 0 {
	//	fmt.Println("GoogleDrivepathIdCache: empty!")
	//}
	//for key, value := range GoogleDrivePathIdCache {
	//	fmt.Printf("Key: %s, Value: %s\n", key, value)
	//}
	//if cachedPath, ok := GoogleDrivePathIdCache[cacheKey]; ok {
	//	fmt.Println("Using cached pathId for", cacheKey, ":", cachedPath)
	//	path = cachedPath
	//	name = filepath.Base(path)
	//	return
	//}

	var recursivePathId = "/"
	var A []*GoogleDriveListResponseFileData
	for {
		fmt.Println("len(A): ", len(A))

		var isDir = true
		if len(A) > 0 {
			firstItem := A[0]
			if recursivePathId == pathId {
				path = firstItem.Path
				name = firstItem.Name
				//GoogleDrivePathIdCache[srcName+"/"+pathId] = path
				return
			}

			recursivePathId = firstItem.Meta.ID
			isDir = firstItem.IsDir
		}

		if isDir {
			firstParam := GoogleDriveListParam{
				Path:  recursivePathId,
				Drive: srcDrive,
				Name:  srcName,
			}
			fmt.Println("firstParam pathId:", pathId)
			var firstJsonBody []byte
			firstJsonBody, err = json.Marshal(firstParam)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				return
			}
			var firstRespBody []byte
			firstRespBody, err = GoogleDriveCall("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson GoogleDriveListResponse
			if err = json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
				fmt.Println(err)
				return
			}

			if len(A) == 0 {
				A = firstBodyJson.Data
			} else {
				A = append(firstBodyJson.Data, A[1:]...)
			}
		} else {
			if len(A) > 0 {
				A = A[1:]
			}
		}
		if len(A) == 0 {
			err = e.New("no result")
			return
		}
	}
}

// deprecated
func getGoogleDriveIdInfos(src string, w http.ResponseWriter, r *http.Request) (path string, name string, isDir bool, err error) {
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	path = ""
	name = ""
	isDir = true
	err = nil

	srcDrive, srcName, pathId, _ := parseGoogleDrivePath(src)
	if strings.Index(pathId, "/") != -1 {
		err = e.New("PathId Parse Error")
		return
	}

	param := GoogleDriveListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	fmt.Println("Google Drive List Params:", string(jsonBody))
	respBody, err := GoogleDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/ls:", err)
		return
	}

	var bodyJson GoogleDriveListResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		fmt.Println(err)
		return
	}

	for _, item := range bodyJson.Data {
		if item.Meta.ID == pathId {
			isDir = false
			path = item.Path
			name = item.Name
			return
		}
	}

	if len(bodyJson.Data) > 0 {
		item := bodyJson.Data[0]
		name = item.Name
		path = strings.TrimSuffix(item.Path, "/"+name)
		if path == "/My Drive" {
			name = "/"
		} else {
			name = filepath.Base(path)
		}
		return
	}

	path, name, err = GoogleDriveIdToPath(srcDrive, srcName, pathId, w, r)
	return
}

type GoogleDriveIdFocusedMetaInfos struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

func getGoogleDriveIdFocusedMetaInfos(src string, w http.ResponseWriter, r *http.Request) (info *GoogleDriveIdFocusedMetaInfos, err error) {
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	info = nil
	err = nil

	srcDrive, srcName, pathId, _ := parseGoogleDrivePath(src)
	if strings.Index(pathId, "/") != -1 {
		err = e.New("PathId Parse Error")
		return
	}

	param := GoogleDriveListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	fmt.Println("Google Drive Awss3MetaResponseMeta Params:", string(jsonBody))
	respBody, err := GoogleDriveCall("/drive/get_file_meta_data", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/get_file_meta_data:", err)
		return
	}

	var bodyJson GoogleDriveMetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		fmt.Println(err)
		return
	}

	info = &GoogleDriveIdFocusedMetaInfos{
		ID:    pathId,
		Path:  bodyJson.Data.Path,
		Name:  bodyJson.Data.Name,
		Size:  bodyJson.Data.FileSize,
		IsDir: bodyJson.Data.IsDir,
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
		fmt.Println(err)
		return
	}

	var A []*GoogleDriveListResponseFileData
	bodyJson.Lock()
	A = append(A, bodyJson.Data...)
	bodyJson.Unlock()

	for len(A) > 0 {
		fmt.Println("len(A): ", len(A))
		firstItem := A[0]
		fmt.Println("firstItem Path: ", firstItem.Path)
		fmt.Println("firstItem Name:", firstItem.Name)

		if firstItem.IsDir {
			//path := firstItem.Path
			//path = strings.TrimPrefix(path, "/My Drive")
			//if path != "/" {
			//	path += "/"
			//}
			//pathId, _, _, _, _, err := GoogleDrivePathToId("/Drive/"+param.Name+path, w, r, true)
			//if err != nil {
			//	fmt.Println(err)
			//	return
			//}
			//fmt.Println("firstItem formed path: /Drive/" + param.Name + path)
			pathId := firstItem.Meta.ID
			firstParam := GoogleDriveListParam{
				Path:  pathId,
				Drive: param.Drive,
				Name:  param.Name,
			}
			fmt.Println("firstParam pathId:", pathId)
			firstJsonBody, err := json.Marshal(firstParam)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				fmt.Println(err)
				return
			}
			var firstRespBody []byte
			firstRespBody, err = GoogleDriveCall("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson GoogleDriveListResponse
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

func copyGoogleDriveSingleFile(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPathId, srcFilename := parseGoogleDrivePath(src)
	//srcPathId, srcDrive, srcName, srcDir, srcFilename, err := GoogleDrivePathToId(src, w, r, false)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		fmt.Println("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPathId, dstFilename := parseGoogleDrivePath(dst)
	//dstPathId, dstDrive, dstName, dstDir, dstFilename, err := GoogleDrivePathToId(dst, w, r, true)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPathId:", dstPathId, "dstFilename:", dstFilename)
	if dstPathId == "" || dstFilename == "" {
		fmt.Println("Dst parse failed.")
		return nil
	}
	// 填充数据
	param := GoogleDriveCopyFileParam{
		CloudFilePath:     srcPathId,   // id of "path/to/cloud/file.txt",
		NewCloudDirectory: dstPathId,   // id of "new/cloud/directory",
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
	_, err = GoogleDriveCall("/drive/copy_file", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/copy_file:", err)
		return err
	}
	return nil
}

func copyGoogleDriveFolder(src, dst string, w http.ResponseWriter, r *http.Request, srcPath, srcPathName string) error {
	srcDrive, srcName, srcPathId, srcFilename := parseGoogleDrivePath(src)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		fmt.Println("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPathId, dstFilename := parseGoogleDrivePath(dst)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPathId:", dstPathId, "dstFilename:", dstFilename)
	if dstPathId == "" || dstFilename == "" {
		fmt.Println("Dst parse failed.")
		return nil
	}

	var CopyTempGoogleDrivePathIdCache = make(map[string]string)
	var recursivePath = srcPath
	var recursivePathId = srcPathId
	var A []*GoogleDriveListResponseFileData
	for {
		fmt.Println("len(A): ", len(A))

		var isDir = true
		var firstItem *GoogleDriveListResponseFileData
		if len(A) > 0 {
			firstItem = A[0]
			recursivePathId = firstItem.Meta.ID
			recursivePath = firstItem.Path
			isDir = firstItem.IsDir
		}

		if isDir {
			// create a new folder and record its id map
			var parentPathId string
			var folderName string
			if srcPathId == recursivePathId {
				parentPathId = dstPathId
				folderName = dstFilename
			} else {
				// 需要取其父目录的ID
				parentPathId = CopyTempGoogleDrivePathIdCache[filepath.Dir(firstItem.Path)]
				folderName = filepath.Base(firstItem.Path)
			}
			postParam := GoogleDrivePostParam{
				ParentPath: parentPathId, // 占位，这个需要变的，先把src侧写对
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
			postRespBody, err = GoogleDriveCall("/drive/create_folder", "POST", postJsonBody, w, r, true)
			if err != nil {
				fmt.Println("Error calling drive/create_folder:", err)
				return err
			}
			var postBodyJson GoogleDrivePostResponse
			if err = json.Unmarshal(postRespBody, &postBodyJson); err != nil {
				fmt.Println(err)
				return err
			}
			CopyTempGoogleDrivePathIdCache[recursivePath] = postBodyJson.Data.Meta.ID

			// list it and get its sub folders and files
			firstParam := GoogleDriveListParam{
				Path:  recursivePathId,
				Drive: srcDrive,
				Name:  srcName,
			}

			fmt.Println("firstParam pathId:", recursivePathId)
			var firstJsonBody []byte
			firstJsonBody, err = json.Marshal(firstParam)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				return err
			}
			var firstRespBody []byte
			firstRespBody, err = GoogleDriveCall("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson GoogleDriveListResponse
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
				fmt.Println(CopyTempGoogleDrivePathIdCache)
				copyPathPrefix := "/Drive/google/" + srcName + "/"
				copySrc := copyPathPrefix + firstItem.Meta.ID + "/"
				parentPathId := CopyTempGoogleDrivePathIdCache[filepath.Dir(firstItem.Path)]
				copyDst := copyPathPrefix + parentPathId + "/" + firstItem.Name
				fmt.Println("copySrc: ", copySrc)
				fmt.Println("copyDst: ", copyDst)
				err := copyGoogleDriveSingleFile(copySrc, copyDst, w, r)
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

func findFirstDownloadingFile(dir string) (string, bool, error) {
	var result string
	found := false

	// Walk函数遍历目录
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 检查文件扩展名
		if !info.IsDir() && filepath.Ext(info.Name()) == ".downloading" {
			result = path // 保存找到的文件路径
			found = true  // 标记为找到
			// 找到文件后，通过返回filepath.SkipDir停止遍历
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}
	// 检查是否找到文件
	if found {
		return result, true, err
	}
	return "", false, err
}

func googleFileToBuffer(src, bufferFilePath string, w http.ResponseWriter, r *http.Request) (string, error) {
	var bufferFilename = ""
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}
	srcDrive, srcName, srcPathId, srcFilename := parseGoogleDrivePath(src)
	//srcPathId, srcDrive, srcName, srcDir, srcFilename, err := GoogleDrivePathToId(src, w, r, false)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		fmt.Println("Src parse failed.")
		return bufferFilename, nil
	}

	// 填充数据
	param := GoogleDriveDownloadFileParam{
		LocalFolder:   bufferFilePath,
		CloudFilePath: srcPathId,
		Drive:         srcDrive, // "my_drive",
		Name:          srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return bufferFilename, err
	}
	fmt.Println("Download File Params:", string(jsonBody))

	var respBody []byte
	respBody, err = GoogleDriveCall("/drive/download_async", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/download_async:", err)
		return bufferFilename, err
	}
	var respJson GoogleDriveTaskResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		fmt.Println(err)
		return bufferFilename, err
	}
	taskId := respJson.Data.ID
	taskParam := GoogleDriveTaskQueryParam{
		TaskIds: []string{taskId},
	}
	// 将数据序列化为 JSON
	taskJsonBody, err := json.Marshal(taskParam)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return bufferFilename, err
	}
	fmt.Println("Task Params:", string(taskJsonBody))

	for {
		time.Sleep(1000 * time.Millisecond)
		var taskRespBody []byte
		taskRespBody, err = GoogleDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/download_async:", err)
			return bufferFilename, err
		}
		var taskRespJson GoogleDriveTaskQueryResponse
		if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
			fmt.Println(err)
			return bufferFilename, err
		}
		if len(taskRespJson.Data) == 0 {
			return bufferFilename, e.New("Task Info Not Found")
		}
		if taskRespJson.Data[0].Status != "Waiting" && taskRespJson.Data[0].Status != "InProgress" {
			if taskRespJson.Data[0].Status == "Completed" {
				//var found bool
				//bufferFilename, found, err = findFirstDownloadingFile(bufferFilePath)
				//if err != nil {
				//	return bufferFilename, err
				//}
				//if !found || bufferFilename == "" {
				//	bufferFilename = bufferFilePath + "/" + srcFilename
				//}
				//fmt.Println("bufferFilename:", bufferFilename)
				//time.Sleep(200 * time.Millisecond)
				return bufferFilename, nil
			}
			return bufferFilename, e.New(taskRespJson.Data[0].Status)
		}
	}
}

func googleBufferToFile(bufferFilePath, dst string, w http.ResponseWriter, r *http.Request) (int, error) {
	dstDrive, dstName, dstPathId, dstFilename := parseGoogleDrivePath(dst)
	//srcPathId, srcDrive, srcName, srcDir, srcFilename, err := GoogleDrivePathToId(src, w, r, false)
	fmt.Println("srcDrive:", dstDrive, "srcName:", dstName, "srcPathId:", dstPathId, "srcFilename:", dstFilename)
	if dstPathId == "" {
		fmt.Println("Src parse failed.")
		return http.StatusBadRequest, nil
	}

	// 填充数据
	param := GoogleDriveUploadFileParam{
		ParentPath:    dstPathId,
		LocalFilePath: bufferFilePath,
		Drive:         dstDrive, // "my_drive",
		Name:          dstName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Download File Params:", string(jsonBody))

	var respBody []byte
	respBody, err = GoogleDriveCall("/drive/upload_async", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/upload_async:", err)
		return errToStatus(err), err
	}
	var respJson GoogleDriveTaskResponse
	if err = json.Unmarshal(respBody, &respJson); err != nil {
		fmt.Println(err)
		return errToStatus(err), err
	}
	taskId := respJson.Data.ID
	taskParam := GoogleDriveTaskQueryParam{
		TaskIds: []string{taskId},
	}
	// 将数据序列化为 JSON
	taskJsonBody, err := json.Marshal(taskParam)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Task Params:", string(taskJsonBody))

	for {
		time.Sleep(500 * time.Millisecond)
		var taskRespBody []byte
		taskRespBody, err = GoogleDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/download_async:", err)
			return errToStatus(err), err
		}
		var taskRespJson GoogleDriveTaskQueryResponse
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

// the hardest
func downloadGoogleDriveSingleFile(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPathId, srcFilename := parseGoogleDrivePath(src)
	//srcPathId, srcDrive, srcName, srcDir, srcFilename, err := GoogleDrivePathToId(src, w, r, false)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		fmt.Println("Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPathId, dstFilename := parseGoogleDrivePath(dst)
	//dstPathId, dstDrive, dstName, dstDir, dstFilename, err := GoogleDrivePathToId(dst, w, r, true)
	fmt.Println("dstDrive:", dstDrive, "dstName:", dstName, "dstPathId:", dstPathId, "dstFilename:", dstFilename)
	if dstPathId == "" || dstFilename == "" {
		fmt.Println("Dst parse failed.")
		return nil
	}
	// 填充数据
	param := GoogleDriveCopyFileParam{
		CloudFilePath:     srcPathId,   // id of "path/to/cloud/file.txt",
		NewCloudDirectory: dstPathId,   // id of "new/cloud/directory",
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
	_, err = GoogleDriveCall("/drive/copy_file", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/copy_file:", err)
		return err
	}
	return nil
}

func downloadGoogleDriveFolder(src string, w http.ResponseWriter, r *http.Request, srcPath, srcPathName string) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}
	var err error

	srcDrive, srcName, srcPathId, srcFilename := parseGoogleDrivePath(src)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		fmt.Println("Src parse failed.")
		return nil
	}

	var CopyTempGoogleDrivePathIdCache = make(map[string]string)
	var recursivePath = srcPath
	var recursivePathId = srcPathId
	var A []*GoogleDriveListResponseFileData
	for {
		fmt.Println("len(A): ", len(A))

		var isDir = true
		var firstItem *GoogleDriveListResponseFileData
		if len(A) > 0 {
			firstItem = A[0]
			recursivePathId = firstItem.Meta.ID
			recursivePath = firstItem.Path
			isDir = firstItem.IsDir
		}

		if isDir {
			// create a new folder to temp directory in terminus
			var tempRootPath string
			if srcPathId == recursivePathId {
				tempRootPath, err = generateBufferFolder(srcPath, bflName)
				if err != nil {
					fmt.Println("Error generating temp folder:", err)
					return err
				}
			}
			createPath := tempRootPath + "/" + strings.TrimPrefix(recursivePath, "/My Drive")
			err = os.MkdirAll(createPath, 0755)
			if err != nil {
				return err
			}

			// list it and get its sub folders and files
			firstParam := GoogleDriveListParam{
				Path:  recursivePathId,
				Drive: srcDrive,
				Name:  srcName,
			}

			fmt.Println("firstParam pathId:", recursivePathId)
			var firstJsonBody []byte
			firstJsonBody, err = json.Marshal(firstParam)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				return err
			}
			var firstRespBody []byte
			firstRespBody, err = GoogleDriveCall("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson GoogleDriveListResponse
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
				fmt.Println(CopyTempGoogleDrivePathIdCache)
				copyPathPrefix := "/Drive/google/" + srcName + "/"
				copySrc := copyPathPrefix + firstItem.Meta.ID + "/"
				parentPathId := CopyTempGoogleDrivePathIdCache[filepath.Dir(firstItem.Path)]
				copyDst := copyPathPrefix + parentPathId + "/" + firstItem.Name
				fmt.Println("copySrc: ", copySrc)
				fmt.Println("copyDst: ", copyDst)
				err := copyGoogleDriveSingleFile(copySrc, copyDst, w, r)
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

func moveGoogleDriveFolderOrFiles(src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPathId, _ := parseGoogleDrivePath(src)
	_, _, dstPathId, _ := parseGoogleDrivePath(dst)

	param := GoogleDriveMoveFileParam{
		CloudFilePath:     srcPathId,
		NewCloudDirectory: dstPathId,
		Drive:             srcDrive, // "my_drive",
		Name:              srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println("Google Drive Patch Params:", string(jsonBody))
	_, err = GoogleDriveCall("/drive/move_file", "POST", jsonBody, w, r, false)
	if err != nil {
		fmt.Println("Error calling drive/move_file:", err)
		return err
	}
	return nil
}

func testDriveLs(w http.ResponseWriter, r *http.Request) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	origin := r.Header.Get("Origin")
	dstUrl := origin + "/api/resources%2FHome%2FDocuments%2F"
	fmt.Println("dstUrl:", dstUrl)

	req, err := http.NewRequest("GET", dstUrl, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return err
	}

	// 设置请求头
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")

	// 遍历并打印所有的 header 字段和值
	for name, values := range req.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	defer resp.Body.Close()

	//// 读取响应体
	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	http.Error(w, "Error reading response body: "+err.Error(), http.StatusInternalServerError)
	//	return err
	//}
	//
	//// 假设响应是UTF-8编码的文本（根据实际情况调整）
	//// 如果Content-Type指明了其他编码，请相应地解码
	//responseText := string(body) // 这里默认按UTF-8处理
	//
	//// 将响应文本写入ResponseWriter（确保设置了正确的Content-Type头）
	//w.Header().Set("Content-Type", "text/plain; charset=utf-8") // 或根据需要设置为其他MIME类型
	//w.Write([]byte(responseText))
	// 读取响应体
	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	http.Error(w, "Error reading response body: "+err.Error(), http.StatusInternalServerError)
	//	return err
	//}
	//
	//// 解析JSON响应体
	//var jsonResponse map[string]interface{}
	//err = json.Unmarshal(body, &jsonResponse)
	//if err != nil {
	//	http.Error(w, "Error unmarshaling JSON response: "+err.Error(), http.StatusInternalServerError)
	//	return err
	//}
	//
	//// 将解析后的JSON响应体转换为字符串（格式化输出）
	//responseText, err := json.MarshalIndent(jsonResponse, "", "  ")
	//if err != nil {
	//	http.Error(w, "Error marshaling JSON response to text: "+err.Error(), http.StatusInternalServerError)
	//	return err
	//}
	//
	//// 设置响应头并写入响应体
	//w.Header().Set("Content-Type", "application/json; charset=utf-8")
	//w.Write([]byte(responseText))

	// 遍历并打印所有的 header 字段和值
	fmt.Printf("GoogleDriveListResponse Hedears:\n")
	for name, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}
	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		fmt.Println("GoogleDriveListResponse is not JSON format:", contentType)
	}

	// 读取响应体
	var body []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		// 如果响应体被gzip压缩
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			return err
		}
		defer reader.Close()

		body, err = ioutil.ReadAll(reader)
		if err != nil {
			fmt.Println("Error reading gzipped response body:", err)
			return err
		}
	} else {
		// 如果响应体没有被压缩
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return err
		}
	}

	// 解析JSON
	var datas map[string]interface{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		fmt.Println("Error unmarshaling JSON response:", err)
		return err
	}

	// 打印解析后的数据
	fmt.Println("Parsed JSON response:", datas)
	// 将解析后的JSON响应体转换为字符串（格式化输出）
	responseText, err := json.MarshalIndent(datas, "", "  ")
	if err != nil {
		http.Error(w, "Error marshaling JSON response to text: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	// 设置响应头并写入响应体
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(responseText))
	return nil
}

func testDriveLs2(w http.ResponseWriter, r *http.Request) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	// dstUrl := "http://files-service.user-space-" + bflName + ":8181/ls"
	origin := r.Header.Get("Origin")
	dstUrl := origin + "/drive/ls"
	fmt.Println("dstUrl:", dstUrl)

	//payload := []byte(`{"path":"/","name":"wangrongxiang@bytetrade.io","drive":"google"}`)
	type RequestPayload struct {
		Path  string `json:"path"`
		Name  string `json:"name"`
		Drive string `json:"drive"`
	}
	payload := RequestPayload{
		Path:  "/",
		Name:  "wangrongxiang@bytetrade.io",
		Drive: "google",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return err
	}

	req, err := http.NewRequest("POST", dstUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return err
	}

	// 设置请求头
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")

	// 遍历并打印所有的 header 字段和值
	for name, values := range req.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response body: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	// 设置Content-Type头（根据实际情况调整）
	w.Header().Set("Content-Type", "application/octet-stream") // 如果是文本，请使用"text/plain; charset=utf-8"或其他适当的MIME类型和字符集

	// 将响应体写入ResponseWriter
	_, err = w.Write(body)
	if err != nil {
		http.Error(w, "Error writing response body: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	fmt.Println("Error reading response body:", err)
	//	return err
	//}

	// Copy the response body directly to the http.ResponseWriter
	//_, err = io.Copy(w, resp.Body)
	//if err != nil {
	//	http.Error(w, "Error copying response body", http.StatusInternalServerError)
	//	return err
	//}

	//fmt.Println(string(body))
	// Write the response body to the http.ResponseWriter
	//w.Header().Set("Content-Type", "application/json")
	//w.Write(body)
	// Convert the response body to UTF-8 encoding
	//bodyString := string(body)
	//utf8Body := []byte(bodyString)
	//
	//// Set the Content-Type header to indicate JSON data
	//w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//w.Write(utf8Body)

	return nil
}

func GoogleDriveCall(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return nil, os.ErrPermission
	}

	authority := r.Header.Get("Authority")
	fmt.Println("*****Google Drive Call URL authority:", authority)
	host := r.Header.Get("Origin")
	if host == "" {
		host = getHost(w, r) // r.Header.Get("Origin")
	}
	fmt.Println("*****Google Drive Call URL host:", host)
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

	// 遍历并打印所有的 header 字段和值
	//for name, values := range req.Header {
	//	for _, value := range values {
	//		fmt.Printf("%s: %s\n", name, value)
	//	}
	//}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	defer resp.Body.Close()

	// 遍历并打印所有的 header 字段和值
	//fmt.Printf("GoogleDriveListResponse Hedears:\n")
	//for name, values := range resp.Header {
	//	for _, value := range values {
	//		fmt.Printf("%s: %s\n", name, value)
	//	}
	//}
	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		fmt.Println("GoogleDrive Call Response is not JSON format:", contentType)
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

func splitGoogleDrivePath(path string) (directory, fileName string) {
	// 找到最后一个 '/' 的索引
	lastIndex := strings.LastIndex(path, "/")

	// 检查是否找到了 '/'
	if lastIndex == -1 {
		// 如果没有找到 '/'，则整个字符串作为文件名，目录为空
		return "", path
	}

	// 提取目录和文件名
	directory = path[:lastIndex]
	fileName = path[lastIndex+1:]

	return directory, fileName
}

func parseGoogleDrivePath(path string) (drive, name, dir, filename string) {
	// 去掉开头的 "/Drive"
	if strings.HasPrefix(path, "/Drive/google") {
		path = path[13:]
		drive = "google"
	}

	// 查找每个 '/' 的位置
	slashes := []int{}
	for i, char := range path {
		if char == '/' {
			slashes = append(slashes, i)
		}
	}

	// 检查是否有足够的 '/' 来提取所需的部分
	if len(slashes) < 2 {
		fmt.Println("Path does not contain enough slashes.")
		return drive, "", "", ""
	}

	// 提取 drive 和 name
	name = path[1:slashes[1]]
	//name = path[slashes[1]+1 : slashes[2]]

	if len(slashes) == 2 {
		return drive, name, "/", path[slashes[1]+1:]
	}

	// 提取 dir 和 filename
	// len(slashes) >= 3
	if slashes[len(slashes)-1] == len(path)-1 {
		// 路径以 '/' 结尾，视为文件夹
		dir = path[slashes[1]+1 : len(path)-1]
		filename = ""
	} else {
		// 路径不以 '/' 结尾，视为文件
		dir = path[slashes[1]+1 : slashes[len(slashes)-1]]
		filename = path[slashes[len(slashes)-1]+1:]
	}

	return drive, name, dir, filename
}

func resourceGetGoogle(w http.ResponseWriter, r *http.Request, stream int, meta int) (int, error) {
	// src is like [repo-id]/path/filename
	src := r.URL.Path
	fmt.Println("src Path:", src)
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	//src = strings.Trim(src, "/") + "/"

	srcDrive, srcName, pathId, _ := parseGoogleDrivePath(src)

	//pathId, srcDrive, srcName, _, _, err := GoogleDrivePathToId(src, w, r, false)
	//if err != nil {
	//	return errToStatus(err), err
	//}

	param := GoogleDriveListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Google Drive List Params:", string(jsonBody))
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
		fmt.Println("Error calling drive/ls:", err)
		return errToStatus(err), err
	}
	return 0, nil
}

func resourcePostGoogle(src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	fmt.Println("src Path:", src)
	src = strings.TrimSuffix(src, "/")

	srcDrive, srcName, pathId, srcNewName := parseGoogleDrivePath(src)

	param := GoogleDrivePostParam{
		ParentPath: pathId,
		FolderName: srcNewName,
		Drive:      srcDrive, // "my_drive",
		Name:       srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil, errToStatus(err), err
	}
	fmt.Println("Google Drive Post Params:", string(jsonBody))
	var respBody []byte = nil
	if returnResp {
		respBody, err = GoogleDriveCall("/drive/create_folder", "POST", jsonBody, w, r, true)
	} else {
		_, err = GoogleDriveCall("/drive/create_folder", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/create_folder:", err)
		return respBody, errToStatus(err), err
	}
	return respBody, 0, nil
}

func resourcePatchGoogle(w http.ResponseWriter, r *http.Request) (int, error) {
	src := r.URL.Path
	dst := r.URL.Query().Get("destination")
	//action := r.URL.Query().Get("action")
	dst, err := url.QueryUnescape(dst)

	srcDrive, srcName, pathId, _ := parseGoogleDrivePath(src)
	_, _, _, dstFilename := parseGoogleDrivePath(dst)

	param := GoogleDrivePatchParam{
		Path:        pathId,
		NewFileName: dstFilename,
		Drive:       srcDrive, // "my_drive",
		Name:        srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Google Drive Patch Params:", string(jsonBody))
	_, err = GoogleDriveCall("/drive/rename", "POST", jsonBody, w, r, false)
	if err != nil {
		fmt.Println("Error calling drive/rename:", err)
		return errToStatus(err), err
	}
	return 0, nil
}

func resourceDeleteGoogle(src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	// src is like [repo-id]/path/filename
	if src == "" {
		src = r.URL.Path
	}
	fmt.Println("src Path:", src)
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	//src = strings.Trim(src, "/") + "/"

	srcDrive, srcName, pathId, _ := parseGoogleDrivePath(src)

	//pathId, srcDrive, srcName, _, _, err := GoogleDrivePathToId(src, w, r, false)
	//if err != nil {
	//	return errToStatus(err), err
	//}

	param := GoogleDriveDeleteParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil, errToStatus(err), err
	}
	fmt.Println("Google Drive List Params:", string(jsonBody))
	//if stream == 1 {
	//	var body []byte
	//	body, err = GoogleDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
	//	streamGoogleDriveFiles(w, r, body, param)
	//	return 0, nil
	//}

	var respBody []byte = nil
	if returnResp {
		respBody, err = GoogleDriveCall("/drive/delete", "POST", jsonBody, w, r, true)
		fmt.Println(string(respBody))
	} else {
		_, err = GoogleDriveCall("/drive/delete", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/delete:", err)
		return nil, errToStatus(err), err
	}
	return respBody, 0, nil
}
