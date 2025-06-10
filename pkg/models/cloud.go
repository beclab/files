package models

import (
	"encoding/json"
	"sync"
)

type CloudResponse struct {
	StatusCode string             `json:"status_code"`
	FailReason *string            `json:"fail_reason"`
	Data       *CloudResponseData `json:"data"`
}

type CloudListResponse struct {
	StatusCode string               `json:"status_code"`
	FailReason *string              `json:"fail_reason,omitempty"`
	Data       []*CloudResponseData `json:"data"`
	sync.Mutex
}

type CloudResponseData struct {
	Path      string                 `json:"path"`
	Name      string                 `json:"name"`
	Size      int64                  `json:"size"`
	FileSize  int64                  `json:"fileSize"`
	Extension string                 `json:"extension"`
	Modified  *string                `json:"modified,omitempty"`
	Mode      string                 `json:"mode"`
	IsDir     bool                   `json:"isDir"`
	IsSymlink bool                   `json:"isSymlink"`
	Type      string                 `json:"type"`
	Meta      *CloudResponseDataMeta `json:"meta"`
}

func (s *CloudResponseData) String() string {
	res, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(res)
}

type CloudResponseDataMeta struct {
	ETag         string  `json:"e_tag"`
	Key          string  `json:"key"`
	LastModified *string `json:"last_modified,omitempty"`
	Owner        *string `json:"owner,omitempty"`
	Size         int     `json:"size"`
	StorageClass string  `json:"storage_class"`
}
