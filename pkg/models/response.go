package models

import (
	"io"
	"net/http"
	"time"
)

type PreviewHandlerResponse struct {
	FileName     string    `json:"file_name"`
	FileModified time.Time `json:"file_modified"`
	Data         []byte    `json:"-"`

	IsCloud    bool          `json:"is_cloud"`
	RespHeader http.Header   `json:"header"`
	StatusCode int           `json:"status_code"`
	ReadCloser io.ReadCloser `json:"-"`
}

type RawHandlerResponse struct {
	FileName     string        `json:"file_name"`
	FileLength   int64         `json:"file_length"`
	FileModified time.Time     `json:"file_modified"`
	IsCloud      bool          `json:"is_cloud"`
	RespHeader   http.Header   `json:"header"`
	StatusCode   int           `json:"status_code"`
	Reader       io.ReadSeeker `json:"-"`
	ReadCloser   io.ReadCloser `json:"-"`
}
