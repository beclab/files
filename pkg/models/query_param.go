package models

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type QueryParam struct {
	Ctx                     context.Context `json:"-"`
	Owner                   string          `json:"owner"`
	PreviewSize             string          `json:"previewSize"`
	PreviewEnableThumbnails bool            `json:"previewEnableThumbnails"`
	PreviewResizePreview    bool            `json:"previewResizePreview"`
	RawInline               string          `json:"rawInline"`
	RawMeta                 string          `json:"rawMeta"` // return json
	Files                   string          `json:"files"`   // like x,y,z
	FileMode                string          `json:"fileMode"`
	RepoName                string          `json:"repoName"`
	RepoId                  string          `json:"repoId"`
	Destination             string          `json:"destination"`
}

func CreateQueryParam(owner string, r *http.Request, enableThumbnails bool, resizePreview bool) *QueryParam {
	// TODO: add for sync
	sizeStr := r.URL.Query().Get("size")
	if sizeStr == "" {
		sizeStr = r.URL.Query().Get("thumb")
	}
	// add end
	return &QueryParam{
		Ctx:                     r.Context(),
		Owner:                   owner,
		PreviewSize:             sizeStr,          // r.URL.Query().Get("size"),
		PreviewEnableThumbnails: enableThumbnails, // todo
		PreviewResizePreview:    resizePreview,    // todo
		RawInline:               strings.TrimSpace(r.URL.Query().Get("inline")),
		RawMeta:                 strings.TrimSpace(r.URL.Query().Get("meta")),
		Files:                   strings.TrimSpace(r.URL.Query().Get("files")),
		FileMode:                strings.TrimSpace(r.URL.Query().Get("mode")),
		RepoName:                strings.TrimSpace(r.URL.Query().Get("repoName")),
		RepoId:                  strings.TrimSpace(r.URL.Query().Get("repoId")),
		Destination:             strings.TrimSpace(r.URL.Query().Get("destination")),
	}
}

func (r *QueryParam) Json() string {
	d, _ := json.Marshal(r)
	return string(d)
}
