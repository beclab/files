package models

import (
	"encoding/json"
	"sync"
	"time"
)

type GoogleDriveResponse struct {
	StatusCode string                   `json:"status_code"`
	FailReason *string                  `json:"fail_reason,omitempty"`
	Data       *GoogleDriveResponseData `json:"data"`
	Message    *string                  `json:"message,omitempty"`
}

func (r *GoogleDriveResponse) IsSuccess() bool {
	return r.StatusCode == "SUCCESS"
}

func (r *GoogleDriveResponse) FailMessage() string {
	if r.FailReason != nil {
		return *r.FailReason
	}
	return ""
}

type GoogleDriveListResponse struct {
	StatusCode string                     `json:"status_code"`
	FailReason *string                    `json:"fail_reason,omitempty"`
	Data       []*GoogleDriveResponseData `json:"data,omitempty"`
	sync.Mutex
}

func (r *GoogleDriveListResponse) IsSuccess() bool {
	return r.StatusCode == "SUCCESS"
}

func (r *GoogleDriveListResponse) FailMessage() string {
	if r.FailReason != nil {
		return *r.FailReason
	}
	return ""
}

type GoogleDriveResponseData struct {
	ID string `json:"id,omitempty"`

	Path         string                       `json:"path"`
	IdPath       string                       `json:"id_path,omitempty"`
	Name         string                       `json:"name"`
	Size         int64                        `json:"size"`
	FileSize     int64                        `json:"fileSize"`
	Extension    string                       `json:"extension"`
	Modified     time.Time                    `json:"modified"`
	Mode         string                       `json:"mode"`
	IsDir        bool                         `json:"isDir"`
	IsSymlink    bool                         `json:"isSymlink"`
	Type         string                       `json:"type"`
	CanDownload  bool                         `json:"canDownload"`
	CanExport    bool                         `json:"canExport"`
	ExportSuffix string                       `json:"exportSuffix,omitempty"`
	Meta         *GoogleDriveResponseDataMeta `json:"meta,omitempty"`
}

func (g *GoogleDriveResponseData) String() string {
	data, err := json.Marshal(g)
	if err != nil {
		return ""
	}
	return string(data)
}

type GoogleDriveResponseDataMeta struct {
	Capabilities                 *GoogleDriveResponseDataMetaCapabilities `json:"capabilities,omitempty"`
	CopyRequiresWriterPermission bool                                     `json:"copyRequiresWriterPermission"`
	CreatedTime                  time.Time                                `json:"createdTime"`
	ExplicitlyTrashed            bool                                     `json:"explicitlyTrashed"`
	FileExtension                *string                                  `json:"fileExtension,omitempty"`
	FullFileExtension            *string                                  `json:"fullFileExtension,omitempty"`
	HasThumbnail                 bool                                     `json:"hasThumbnail"`
	HeadRevisionId               *string                                  `json:"headRevisionId,omitempty"`
	IconLink                     string                                   `json:"iconLink"`
	ID                           string                                   `json:"id"`
	IsAppAuthorized              bool                                     `json:"isAppAuthorized"`
	Kind                         string                                   `json:"kind"`
	LastModifyingUser            *GoogleDriveResponseLastModifyingUser    `json:"lastModifyingUser,omitempty"`
	LinkShareMetadata            *GoogleDriveResponseLinkShareMetadata    `json:"linkShareMetadata,omitempty"`
	MD5Checksum                  *string                                  `json:"md5Checksum,omitempty"`
	MimeType                     string                                   `json:"mimeType"`
	ModifiedByMe                 bool                                     `json:"modifiedByMe"`
	ModifiedTime                 time.Time                                `json:"modifiedTime"`
	Name                         string                                   `json:"name"`
	OriginalFilename             *string                                  `json:"originalFilename,omitempty"`
	OwnedByMe                    bool                                     `json:"ownedByMe"`
	Owners                       []*GoogleDriveResponseOwners             `json:"owners,omitempty"`
	QuotaBytesUsed               string                                   `json:"quotaBytesUsed"`
	SHA1Checksum                 *string                                  `json:"sha1Checksum,omitempty"`
	SHA256Checksum               *string                                  `json:"sha256Checksum,omitempty"`
	Shared                       bool                                     `json:"shared"`
	SharedWithMeTime             *time.Time                               `json:"sharedWithMeTime,omitempty"`
	Size                         *string                                  `json:"size,omitempty"`
	Spaces                       []string                                 `json:"spaces"`
	Starred                      bool                                     `json:"starred"`
	ThumbnailLink                *string                                  `json:"thumbnailLink,omitempty"`
	ThumbnailVersion             string                                   `json:"thumbnailVersion"`
	Title                        *string                                  `json:"title,omitempty"`
	Trashed                      bool                                     `json:"trashed"`
	Version                      string                                   `json:"version"`
	ViewedByMe                   bool                                     `json:"viewedByMe"`
	ViewedByMeTime               time.Time                                `json:"viewedByMeTime"`
	ViewersCanCopyContent        bool                                     `json:"viewersCanCopyContent"`
	WebContentLink               *string                                  `json:"webContentLink,omitempty"`
	WebViewLink                  string                                   `json:"webViewLink"`
	WritersCanShare              bool                                     `json:"writersCanShare"`
}

type GoogleDriveResponseDataMetaCapabilities struct {
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

type GoogleDriveResponseLastModifyingUser struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Kind         string `json:"kind"`
	Me           bool   `json:"me"`
	PermissionID string `json:"permissionId"`
}

type GoogleDriveResponseLinkShareMetadata struct {
	SecurityUpdateEligible bool `json:"securityUpdateEligible"`
	SecurityUpdateEnabled  bool `json:"securityUpdateEnabled"`
}

type GoogleDriveResponseOwners struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Kind         string `json:"kind"`
	Me           bool   `json:"me"`
	PermissionId string `json:"permissionId"`
}
