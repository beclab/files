package upload

import (
	"mime/multipart"
	"time"
)

type UploadChunkFile struct {
	ChunkFile  *multipart.FileHeader `json:"chunk_file" form:"chunk_file" binding:"required"`
	ChunkIndex string                `json:"chunk_index" form:"chunk_index" binding:"required"`
	ChunkHash  string                `json:"chunk_hash" form:"chunk_hash" binding:"required"`
	ChunkSum   int                   `json:"chunk_sum" form:"chunk_sum" binding:"required"`
	FileHash   string                `json:"file_hash" form:"file_hash" binding:"required"`
}

type MergeChunkFile struct {
	FileHash    string `json:"file_hash" form:"file_hash" binding:"required"`
	ChunkSum    int    `json:"chunk_sum" form:"chunk_sum" binding:"required"`
	StoragePath string `json:"storage_path" form:"storage_path"`
	FileName    string `json:"file_name" form:"file_name" binding:"required"`
}

type FileMetaData struct {
	FileRelativePath string `json:"file_relative_path" form:"file_relative_path" binding:"required"`
	FileType         string `json:"file_type" form:"file_type" binding:"required"`
	FileSize         int64  `json:"file_size" form:"file_size" binding:"required"`
	StoragePath      string `json:"storage_path" form:"storage_path" binding:"required"`
	FullPath         string `json:"full_path"` // storage_path(must exist) + file_relative_path
}

type FileInfo struct {
	ID             string    `json:"id"`
	Offset         int64     `json:"offset"`
	LastUpdateTime time.Time `json:"-"`
	FileMetaData
}

type FilePatchInfo struct {
	File         *multipart.FileHeader `json:"file" form:"file" binding:"required"`
	UploadOffset int64                 `json:"upload_offset" form:"upload_offset" binding:"required"`
}

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
	ParentDir                 string                `json:"parent_dir" form:"parent_dir"`
	MD5                       string                `json:"md5,omitempty" form:"md5"`
	File                      *multipart.FileHeader `json:"file" form:"file" binding:"required"`
}
