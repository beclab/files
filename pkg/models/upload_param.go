package models

import (
	"mime/multipart"

	"github.com/cloudwego/hertz/pkg/app"
)

type ResumableInfo struct {
	ResumableChunkNumber      int                   `json:"resumableChunkNumber" form:"resumableChunkNumber"`
	ResumableChunkSize        int64                 `json:"resumableChunkSize" form:"resumableChunkSize"`
	ResumableCurrentChunkSize int64                 `json:"resumableCurrentChunkSize" form:"resumableCurrentChunkSize"`
	ResumableTotalSize        int64                 `json:"resumableTotalSize" form:"resumableTotalSize"`
	ResumableType             string                `json:"resumableType" form:"resumableType"`
	ResumableIdentifier       string                `json:"resumableIdentifier" form:"resumableIdentifier"`
	ResumableIdenty           string                `json:"resumableIdenty" form:"resumableIdenty"`
	ResumableFilename         string                `json:"resumableFilename" form:"resumableFilename"`
	ResumableRelativePath     string                `json:"resumableRelativePath" form:"resumableRelativePath"`
	ResumableTotalChunks      int                   `json:"resumableTotalChunks" form:"resumableTotalChunks"`
	UploadToCloud             int                   `json:"uploadToCloud" form:"uploadToCloud"`                     // if cloud, val = 1
	UploadToCloudTaskId       string                `json:"uploadToCloudTaskId" form:"uploadToCloudTaskId"`         // task id from serve
	UploadToCloudTaskCancel   int                   `json:"uploadToCloudTaskCancel" form:"uploadToCloudTaskCancel"` // if canceled, val = 1
	ParentDir                 string                `json:"parent_dir" form:"parent_dir"`
	MD5                       string                `json:"md5,omitempty" form:"md5"`
	File                      *multipart.FileHeader `json:"file" form:"file" binding:"required"`
	Share                     string                `json:"share" form:"share"` // val = 1
	ShareType                 string                `json:"sharetype" form:"sharetype"`
	Shareby                   string                `json:"shareby" form:"shareby"`
	SharebyPath               string                `json:"sharebyPath" form:"sharebyPath"`
}

type FileUploadArgs struct {
	Node           string         `json:"node"` // node name
	FileParam      *FileParam     `json:"fileParam"`
	FileName       string         `json:"fileName,omitempty"`
	From           string         `json:"from,omitempty"`
	Identy         string         `json:"identy"`
	Share          string         `json:"share"`
	ShareType      string         `json:"sharetype"`
	ShareBy        string         `json:"shareby"`
	UploadId       string         `json:"uploadId,omitempty"`
	Ranges         string         `json:"ranges,omitempty"`
	UserAgentHash  string         `json:"userAgentHash"`
	ChunkInfo      *ResumableInfo `json:"chunkInfo,omitempty"`
	RequestContext *app.RequestContext
}
