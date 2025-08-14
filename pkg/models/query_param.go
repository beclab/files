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
	RawInline               string          `json:"rawInline,omitempty"`
	RawMeta                 string          `json:"rawMeta,omitempty"` // return json
	Files                   string          `json:"files,omitempty"`   // like x,y,z
	FileMode                string          `json:"fileMode,omitempty"`
	RepoName                string          `json:"repoName,omitempty"`
	RepoId                  string          `json:"repoId,omitempty"`
	Destination             string          `json:"destination,omitempty"`
	ShareType               string          `json:"shareType,omitempty"`
	Header                  http.Header     `json:"-"`
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
		ShareType:               strings.TrimSpace(r.URL.Query().Get("type")), // "mine", "shared", "share_to_me"
		Header:                  r.Header,
	}
}

func (r *QueryParam) Json() string {
	d, _ := json.Marshal(r)
	return string(d)
}
