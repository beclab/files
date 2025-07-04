package models

import (
	"io"
	"time"
)

type PreviewHandlerResponse struct {
	FileName     string    `json:"file_name"`
	FileModified time.Time `json:"file_modified"`
	Data         []byte    `json:"-"`
}

type RawHandlerResponse struct {
	FileName     string        `json:"file_name"`
	FileModified time.Time     `json:"file_modified"`
	Reader       io.ReadSeeker `json:"-"`
}
