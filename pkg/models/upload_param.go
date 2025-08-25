package models

import (
	"mime/multipart"
)

type ResumableInfo struct {
	ResumableChunkNumber      int                   `json:"resumableChunkNumber" form:"resumableChunkNumber"`
	ResumableChunkSize        int64                 `json:"resumableChunkSize" form:"resumableChunkSize"`
	ResumableCurrentChunkSize int64                 `json:"resumableCurrentChunkSize" form:"resumableCurrentChunkSize"`
	ResumableTotalSize        int64                 `json:"resumableTotalSize" form:"resumableTotalSize"`
	ResumableType             string                `json:"resumableType" form:"resumableType"`
	ResumableIdentifier       string                `json:"resumableIdentifier" form:"resumableIdentifier"`
	ResumableFilename         string                `json:"resumableFilename" form:"resumableFilename"`
	ResumableRelativePath     string                `json:"resumableRelativePath" form:"resumableRelativePath"`
	ResumableTotalChunks      int                   `json:"resumableTotalChunks" form:"resumableTotalChunks"`
	UploadToCloud             int                   `json:"uploadToCloud" form:"uploadToCloud"`                     // if cloud, val = 1
	UploadToCloudTaskId       string                `json:"uploadToCloudTaskId" form:"uploadToCloudTaskId"`         // task id from serve
	UploadToCloudTaskCancel   int                   `json:"uploadToCloudTaskCancel" form:"uploadToCloudTaskCancel"` // if canceled, val = 1
	ParentDir                 string                `json:"parent_dir" form:"parent_dir"`
	MD5                       string                `json:"md5,omitempty" form:"md5"`
	File                      *multipart.FileHeader `json:"file" form:"file" binding:"required"`
}

type FileUploadArgs struct {
	Node      string         `json:"node"` // node name
	FileParam *FileParam     `json:"fileParam"`
	FileName  string         `json:"fileName,omitempty"`
	From      string         `json:"from,omitempty"`
	UploadId  string         `json:"uploadId,omitempty"`
	Ranges    string         `json:"ranges,omitempty"`
	ChunkInfo *ResumableInfo `json:"chunkInfo,omitempty"`
}
