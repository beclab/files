package model

import (
	"sync"
	"time"
)

type Response struct {
	StatusCode string        `json:"status_code"`
	FailReason *string       `json:"fail_reason,omitempty"`
	Data       *ResponseData `json:"data"`
	Message    *string       `json:"message,omitempty"`
}

func (r *Response) IsSuccess() bool {
	return r.StatusCode == "SUCCESS"
}

func (r *Response) FailMessage() string {
	if r.FailReason != nil {
		return *r.FailReason
	}
	return ""
}

type ListResponse struct {
	StatusCode string          `json:"status_code"`
	FailReason *string         `json:"fail_reason,omitempty"`
	Data       []*ResponseData `json:"data,omitempty"`
	sync.Mutex
}

func (r *ListResponse) IsSuccess() bool {
	return r.StatusCode == "SUCCESS"
}

func (r *ListResponse) FailMessage() string {
	if r.FailReason != nil {
		return *r.FailReason
	}
	return ""
}

type ResponseData struct {
	ID           string            `json:"id"`
	Path         string            `json:"path"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	FileSize     int64             `json:"fileSize"`
	Extension    string            `json:"extension"`
	Modified     time.Time         `json:"modified"`
	Mode         string            `json:"mode"`
	IsDir        bool              `json:"isDir"`
	IsSymlink    bool              `json:"isSymlink"`
	Type         string            `json:"type"`
	Meta         *ResponseDataMeta `json:"meta,omitempty"`
	CanDownload  bool              `json:"canDownload"`
	CanExport    bool              `json:"canExport"`
	ExportSuffix string            `json:"exportSuffix"`
	IdPath       string            `json:"id_path,omitempty"`
}

type ResponseDataMeta struct {
	Capabilities                 *ResponseDataMetaCapabilities `json:"capabilities,omitempty"`
	CopyRequiresWriterPermission bool                          `json:"copyRequiresWriterPermission"`
	CreatedTime                  time.Time                     `json:"createdTime"`
	ExplicitlyTrashed            bool                          `json:"explicitlyTrashed"`
	FileExtension                *string                       `json:"fileExtension,omitempty"`
	FullFileExtension            *string                       `json:"fullFileExtension,omitempty"`
	HasThumbnail                 bool                          `json:"hasThumbnail"`
	HeadRevisionId               *string                       `json:"headRevisionId,omitempty"`
	IconLink                     string                        `json:"iconLink"`
	ID                           string                        `json:"id"`
	IsAppAuthorized              bool                          `json:"isAppAuthorized"`
	Kind                         string                        `json:"kind"`
	LastModifyingUser            *ResponseLastModifyingUser    `json:"lastModifyingUser,omitempty"`
	LinkShareMetadata            *ResponseLinkShareMetadata    `json:"linkShareMetadata,omitempty"`
	MD5Checksum                  *string                       `json:"md5Checksum,omitempty"`
	MimeType                     string                        `json:"mimeType"`
	ModifiedByMe                 bool                          `json:"modifiedByMe"`
	ModifiedTime                 time.Time                     `json:"modifiedTime"`
	Name                         string                        `json:"name"`
	OriginalFilename             *string                       `json:"originalFilename,omitempty"`
	OwnedByMe                    bool                          `json:"ownedByMe"`
	Owners                       []*ResponseOwners             `json:"owners,omitempty"`
	QuotaBytesUsed               string                        `json:"quotaBytesUsed"`
	SHA1Checksum                 *string                       `json:"sha1Checksum,omitempty"`
	SHA256Checksum               *string                       `json:"sha256Checksum,omitempty"`
	Shared                       bool                          `json:"shared"`
	SharedWithMeTime             *time.Time                    `json:"sharedWithMeTime,omitempty"`
	Size                         *string                       `json:"size,omitempty"`
	Spaces                       []string                      `json:"spaces"`
	Starred                      bool                          `json:"starred"`
	ThumbnailLink                *string                       `json:"thumbnailLink,omitempty"`
	ThumbnailVersion             string                        `json:"thumbnailVersion"`
	Title                        *string                       `json:"title,omitempty"`
	Trashed                      bool                          `json:"trashed"`
	Version                      string                        `json:"version"`
	ViewedByMe                   bool                          `json:"viewedByMe"`
	ViewedByMeTime               time.Time                     `json:"viewedByMeTime"`
	ViewersCanCopyContent        bool                          `json:"viewersCanCopyContent"`
	WebContentLink               *string                       `json:"webContentLink,omitempty"`
	WebViewLink                  string                        `json:"webViewLink"`
	WritersCanShare              bool                          `json:"writersCanShare"`
}

type ResponseDataMetaCapabilities struct {
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

type ResponseLastModifyingUser struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Kind         string `json:"kind"`
	Me           bool   `json:"me"`
	PermissionID string `json:"permissionId"`
}

type ResponseLinkShareMetadata struct {
	SecurityUpdateEligible bool `json:"securityUpdateEligible"`
	SecurityUpdateEnabled  bool `json:"securityUpdateEnabled"`
}

type ResponseOwners struct {
	StatusCode string               `json:"status_code"`
	FailReason *string              `json:"fail_reason,omitempty"`
	Data       *GoogleDriveMetaData `json:"data"`
	Message    *string              `json:"message,omitempty"`
}

type GoogleDriveRenameResponse struct {
	StatusCode string               `json:"status_code"`
	FailReason *string              `json:"fail_reason,omitempty"`
	Data       *GoogleDriveMetaData `json:"data"`
	Message    *string              `json:"message,omitempty"`
}

type GoogleDriveDeleteResponse struct {
	StatusCode string               `json:"status_code"`
	FailReason *string              `json:"fail_reason,omitempty"`
	Data       *GoogleDriveMetaData `json:"data"`
	Message    *string              `json:"message,omitempty"`
}

type GoogleDriveMetaResponse struct {
	StatusCode string               `json:"status_code"`
	FailReason *string              `json:"fail_reason,omitempty"`
	Data       *GoogleDriveMetaData `json:"data"`
	Message    *string              `json:"message,omitempty"`
}

func (gdmr *GoogleDriveMetaResponse) IsSuccess() bool {
	return gdmr.StatusCode == "SUCCESS"
}

func (gdmr *GoogleDriveMetaResponse) FailMessage() string {
	if gdmr.FailReason != nil {
		return *gdmr.FailReason
	}
	return ""
}

type GoogleDriveDownloadAsyncResponse struct {
	StatusCode string               `json:"status_code"`
	FailReason *string              `json:"fail_reason,omitempty"`
	Data       *GoogleDriveMetaData `json:"data"`
	Message    *string              `json:"message,omitempty"`
}

type GoogleDriveUploadAsyncResponse struct {
	StatusCode string               `json:"status_code"`
	FailReason *string              `json:"fail_reason,omitempty"`
	Data       *GoogleDriveMetaData `json:"data"`
	Message    *string              `json:"message,omitempty"`
}

type GoogleDriveMetaData struct {
	Path         string                   `json:"path"`
	IdPath       string                   `json:"id_path"`
	Name         string                   `json:"name"`
	Size         int64                    `json:"size"`
	FileSize     int64                    `json:"fileSize"`
	Extension    string                   `json:"extension"`
	Modified     time.Time                `json:"modified"`
	Mode         string                   `json:"mode"`
	IsDir        bool                     `json:"isDir"`
	IsSymlink    bool                     `json:"isSymlink"`
	Type         string                   `json:"type"`
	CanDownload  bool                     `json:"canDownload"`
	CanExport    bool                     `json:"canExport"`
	ExportSuffix string                   `json:"exportSuffix"`
	Meta         *GoogleDriveMetaFileMeta `json:"meta"`
	ID           string                   `json:"id"` // ??
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
