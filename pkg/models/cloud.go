package models

import (
	"encoding/json"
	"sync"
	"time"
)

type CloudResponse struct {
	StatusCode string             `json:"status_code"`
	FailReason *string            `json:"fail_reason,omitempty"`
	Data       *CloudResponseData `json:"data"`
	Message    *string            `json:"message"`
}

func (s *CloudResponse) Modified() string {
	if s.Data.Modified != nil {
		return *s.Data.Modified
	}
	return ""
}

func (c *CloudResponse) IsSuccess() bool {
	return c.StatusCode == "SUCCESS"
}

func (c *CloudResponse) FailMessage() string {
	if c.FailReason != nil {
		return *c.FailReason
	}
	return ""
}

type CloudListResponse struct {
	StatusCode string               `json:"status_code"`
	FailReason *string              `json:"fail_reason,omitempty"`
	Data       []*CloudResponseData `json:"data"`
	FileType   string               `json:"fileType"`
	FileExtend string               `json:"fileExtend"`
	FilePath   string               `json:"filePath"`
	Name       string               `json:"name"`
	sync.Mutex
}

func (c *CloudListResponse) IsSuccess() bool {
	return c.StatusCode == "SUCCESS"
}

func (c *CloudListResponse) FailMessage() string {
	if c.FailReason != nil {
		return *c.FailReason
	}
	return ""
}

type CloudResponseData struct {
	ID           string                 `json:"id,omitempty"`
	FsType       string                 `json:"fileType"`
	FsExtend     string                 `json:"fileExtend"`
	FsPath       string                 `json:"filePath"`
	IdPath       string                 `json:"id_path,omitempty"`
	Path         string                 `json:"path"`
	Name         string                 `json:"name"`
	Size         int64                  `json:"size"`
	FileSize     int64                  `json:"fileSize"`
	Extension    string                 `json:"extension"`
	Modified     *string                `json:"modified,omitempty"`
	Mode         string                 `json:"mode"`
	IsDir        bool                   `json:"isDir"`
	IsSymlink    bool                   `json:"isSymlink"`
	Type         string                 `json:"type"`
	CanDownload  bool                   `json:"canDownload,omitempty"`
	CanExport    bool                   `json:"canExport,omitempty"`
	ExportSuffix string                 `json:"exportSuffix,omitempty"`
	Meta         *CloudResponseDataMeta `json:"meta"`
}

func (s *CloudResponseData) String() string {
	res, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(res)
}

type CloudResponseDataMeta struct {
	ETag         string  `json:"e_tag,omitempty"`
	Key          string  `json:"key,omitempty"`
	LastModified *string `json:"last_modified,omitempty"`
	Owner        *string `json:"owner,omitempty"`
	StorageClass string  `json:"storage_class,omitempty"`

	Capabilities                 *CloudResponseDataMetaCapabilities `json:"capabilities,omitempty"`
	CopyRequiresWriterPermission bool                               `json:"copyRequiresWriterPermission,omitempty"`
	CreatedTime                  time.Time                          `json:"createdTime,omitempty"`
	ExplicitlyTrashed            bool                               `json:"explicitlyTrashed,omitempty"`
	FileExtension                *string                            `json:"fileExtension,omitempty"`
	FullFileExtension            *string                            `json:"fullFileExtension,omitempty"`
	HasThumbnail                 bool                               `json:"hasThumbnail,omitempty"`
	HeadRevisionId               *string                            `json:"headRevisionId,omitempty"`
	IconLink                     string                             `json:"iconLink,omitempty"`
	ID                           string                             `json:"id,omitempty"`
	IsAppAuthorized              bool                               `json:"isAppAuthorized,omitempty"`
	Kind                         string                             `json:"kind,omitempty"`
	LastModifyingUser            *CloudResponseLastModifyingUser    `json:"lastModifyingUser,omitempty"`
	LinkShareMetadata            *CloudResponseLinkShareMetadata    `json:"linkShareMetadata,omitempty"`
	MD5Checksum                  *string                            `json:"md5Checksum,omitempty"`
	MimeType                     string                             `json:"mimeType,omitempty"`
	ModifiedByMe                 bool                               `json:"modifiedByMe,omitempty"`
	ModifiedTime                 time.Time                          `json:"modifiedTime,omitempty"`
	Name                         string                             `json:"name,omitempty"`
	OriginalFilename             *string                            `json:"originalFilename,omitempty"`
	OwnedByMe                    bool                               `json:"ownedByMe,omitempty"`
	Owners                       []*CloudResponseOwners             `json:"owners,omitempty"`
	QuotaBytesUsed               string                             `json:"quotaBytesUsed,omitempty"`
	SHA1Checksum                 *string                            `json:"sha1Checksum,omitempty"`
	SHA256Checksum               *string                            `json:"sha256Checksum,omitempty"`
	Shared                       bool                               `json:"shared,omitempty"`
	SharedWithMeTime             *time.Time                         `json:"sharedWithMeTime,omitempty"`
	Size                         interface{}                        `json:"size,omitempty"`
	Spaces                       []string                           `json:"spaces,omitempty"`
	Starred                      bool                               `json:"starred,omitempty"`
	ThumbnailLink                *string                            `json:"thumbnailLink,omitempty"`
	ThumbnailVersion             string                             `json:"thumbnailVersion,omitempty"`
	Title                        *string                            `json:"title,omitempty"`
	Trashed                      bool                               `json:"trashed,omitempty"`
	Version                      string                             `json:"version,omitempty"`
	ViewedByMe                   bool                               `json:"viewedByMe,omitempty"`
	ViewedByMeTime               time.Time                          `json:"viewedByMeTime,omitempty"`
	ViewersCanCopyContent        bool                               `json:"viewersCanCopyContent,omitempty"`
	WebContentLink               *string                            `json:"webContentLink,omitempty"`
	WebViewLink                  string                             `json:"webViewLink,omitempty"`
	WritersCanShare              bool                               `json:"writersCanShare,omitempty"`
}

type CloudResponseDataMetaCapabilities struct {
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

type CloudResponseLastModifyingUser struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Kind         string `json:"kind"`
	Me           bool   `json:"me"`
	PermissionID string `json:"permissionId"`
}

type CloudResponseLinkShareMetadata struct {
	SecurityUpdateEligible bool `json:"securityUpdateEligible"`
	SecurityUpdateEnabled  bool `json:"securityUpdateEnabled"`
}

type CloudResponseOwners struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Kind         string `json:"kind"`
	Me           bool   `json:"me"`
	PermissionId string `json:"permissionId"`
}
