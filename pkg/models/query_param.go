package models

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type QueryParam struct {
	Ctx                     context.Context `json:"-"`
	Owner                   string          `json:"owner"`
	PreviewSize             string          `json:"previewSize"`
	PreviewEnableThumbnails bool            `json:"previewEnableThumbnails"`
	PreviewResizePreview    bool            `json:"previewResizePreview"`
	RawInline               string          `json:"rawInline,omitempty"`
	RawMeta                 string          `json:"rawMeta,omitempty"` // return json
	Files                   string          `json:"files,omitempty"`   // like x,y,z
	FileMode                string          `json:"fileMode,omitempty"`
	RepoName                string          `json:"repoName,omitempty"`
	RepoId                  string          `json:"repoId,omitempty"`
	Destination             string          `json:"destination,omitempty"`
	ShareType               string          `json:"shareType,omitempty"`
	Header                  http.Header     `json:"-"`
	Body                    io.ReadCloser   `json:"-"`
}

func (r *QueryParam) Json() string {
	d, _ := json.Marshal(r)
	return string(d)
}
