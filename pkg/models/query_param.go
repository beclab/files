package models

import (
	"context"
	"encoding/json"
	"files/pkg/utils"
	"net/http"
)

type QueryParam struct {
	Ctx              context.Context `json:"-"`
	Owner            string          `json:"owner"`
	Stream           *int            `json:"stream"`
	Size             string          `json:"size"`
	Inline           string          `json:"inline"`
	EnableThumbnails bool            `json:"enableThumbnails"`
	ResizePreview    bool            `json:"resizePreview"`
	Files            string          `json:"files"` // like x,y,z
}

func CreateQueryParam(owner string, r *http.Request, enableThumbnails bool, resizePreview bool) (*QueryParam, error) {
	var queryParam = &QueryParam{
		Ctx:              r.Context(),
		Owner:            owner,
		EnableThumbnails: enableThumbnails,
		ResizePreview:    resizePreview,
	}

	var streamQuery = r.URL.Query().Get("stream")
	if streamQuery != "" {
		streamQueryInt, err := utils.ParseInt(streamQuery)
		if err != nil {
			return nil, err
		}
		queryParam.Stream = &streamQueryInt
	}

	queryParam.Size = r.URL.Query().Get("size")
	queryParam.Inline = r.URL.Query().Get("inline")
	queryParam.Files = r.URL.Query().Get("files")

	return queryParam, nil

}

func (r *QueryParam) Json() string {
	d, _ := json.Marshal(r)
	return string(d)
}
