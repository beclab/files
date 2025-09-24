package models

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
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
	DriveId                 string          `json:"driveid"`
	Destination             string          `json:"destination,omitempty"`
	ShareType               string          `json:"shareType,omitempty"`
	Header                  http.Header     `json:"-"`
	Body                    io.ReadCloser   `json:"-"`
}

func CreateQueryParam(owner string, ctx context.Context, c *app.RequestContext, enableThumbnails bool, resizePreview bool) *QueryParam {
	// TODO: add for sync
	sizeStr := c.Query("size")
	if sizeStr == "" {
		sizeStr = c.Query("thumb")
	}
	// add end

	header := make(http.Header)
	c.Request.Header.VisitAll(func(key, value []byte) {
		headerKey := string(key)
		headerValue := string(value)
		header.Add(headerKey, headerValue)
	})

	return &QueryParam{
		Ctx:                     ctx,
		Owner:                   owner,
		PreviewSize:             sizeStr,
		PreviewEnableThumbnails: enableThumbnails, // todo
		PreviewResizePreview:    resizePreview,    // todo
		RawInline:               strings.TrimSpace(c.Query("inline")),
		RawMeta:                 strings.TrimSpace(c.Query("meta")),
		Files:                   strings.TrimSpace(c.Query("files")),
		FileMode:                strings.TrimSpace(c.Query("mode")),
		RepoName:                strings.TrimSpace(c.Query("repoName")),
		RepoId:                  strings.TrimSpace(c.Query("repoId")),
		DriveId:                 strings.TrimSpace(c.Query("driveId")),
		Destination:             strings.TrimSpace(c.Query("destination")),
		ShareType:               strings.TrimSpace(c.Query("type")), // "mine", "shared", "share_to_me"
		Header:                  header,
		Body:                    io.NopCloser(bytes.NewReader(c.Request.Body())),
	}
}

func (r *QueryParam) Json() string {
	d, _ := json.Marshal(r)
	return string(d)
}
