package models

import (
	"context"
	"encoding/json"
	"net/http"
)

type QueryParam struct {
	Ctx                     context.Context `json:"-"`
	Owner                   string          `json:"owner"`
	PreviewSize             string          `json:"previewSize"`
	PreviewEnableThumbnails bool            `json:"previewEnableThumbnails"`
	PreviewResizePreview    bool            `json:"previewResizePreview"`
	RawInline               string          `json:"rawInline"`
	Files                   string          `json:"files"` // like x,y,z
	FileMode                string          `json:"fileMode"`
}

func CreateQueryParam(owner string, r *http.Request, enableThumbnails bool, resizePreview bool) *QueryParam {
	return &QueryParam{
		Ctx:                     r.Context(),
		Owner:                   owner,
		PreviewSize:             r.URL.Query().Get("size"),
		PreviewEnableThumbnails: enableThumbnails, // todo
		PreviewResizePreview:    resizePreview,    // todo
		RawInline:               r.URL.Query().Get("inline"),
		Files:                   r.URL.Query().Get("files"),
		FileMode:                r.URL.Query().Get("mode"),
	}
}

func (r *QueryParam) Json() string {
	d, _ := json.Marshal(r)
	return string(d)
}
