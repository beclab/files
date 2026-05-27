// Package archive implements the /api/archive/:node/... endpoints.
//
// Two writable endpoints (compress / extract) submit jobs to the
// existing Task system and return a task_id; the FE then polls
// /api/task/:node/?task_id=... for progress.
//
// Two read endpoints (entries / entry) stream NDJSON and raw octet-
// stream respectively. They don't go through Task at all; the client
// closing the connection cancels the underlying reader and kills any
// 7z subprocess.
package archive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"files/pkg/archive/reader"
	"files/pkg/archive/sevenz"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	archmodel "files/pkg/hertz/biz/model/api/archive"
	"files/pkg/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
)

// HeaderPassword is the request header carrying the archive password.
// Always passed via header (never query or body) so it doesn't end up
// in access logs or replay buffers.
const HeaderPassword = "X-Archive-Password"

// EntriesReq and EntryReq are NOT in the thrift IDL (the preview
// endpoints stream NDJSON / octet-stream which thrift can't model
// directly), so we declare them inline here. CompressReq / ExtractReq
// live in pkg/hertz/biz/model/api/archive and are produced from
// pkg/hertz/idl/archive.thrift by `scripts/generate-hertz.sh`.

type EntriesReq struct {
	Source string `form:"source" query:"source" vd:"len($)>0"`
}

type EntryReq struct {
	Source string `form:"source" query:"source" vd:"len($)>0"`
	Path   string `form:"path" query:"path" vd:"len($)>0"`
}

// CompressMethod handles POST /api/archive/:node/compress.
//
// Body shape (JSON):
//
//	{
//	  "sources":      ["/drive/Home/folder", "/drive/Home/file.txt"],
//	  "destination":  "/drive/Home/out.zip",
//	  "format":       "zip" | "7z" | "tar.gz" | ... (optional; inferred from destination suffix when omitted),
//	  "level":        5,             // 0..9, optional
//	  "volumeSizeMB": 100,           // 0 = single file, optional
//	  "preserveSymlinks": false,
//	  "conflict":     "rename" | "overwrite" | "skip"
//	}
//
// Password (if any) is supplied via header X-Archive-Password.
func CompressMethod(ctx context.Context, c *app.RequestContext) {
	var req archmodel.CompressReq
	if err := c.BindAndValidate(&req); err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}

	owner := string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "user not found"})
		return
	}

	// Resolve every source URI to a FileParam; reject as soon as any
	// one isn't a posix-class storage.
	srcs := make([]*models.FileParam, 0, len(req.Sources))
	var srcFileType string
	for _, s := range req.Sources {
		fp, err := models.CreateFileParam(owner, s)
		if err != nil {
			c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("source param error: %v", err)})
			return
		}
		if !common.ListContains(common.PosixFileTypes, fp.FileType) {
			c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive only supported on local storages"})
			return
		}
		if srcFileType == "" {
			srcFileType = fp.FileType
		} else if srcFileType != fp.FileType {
			c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "all sources must share one storage type"})
			return
		}
		srcs = append(srcs, fp)
	}

	dst, err := models.CreateFileParam(owner, req.Destination)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("destination param error: %v", err)})
		return
	}
	if !common.ListContains(common.PosixFileTypes, dst.FileType) {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive only supported on local storages"})
		return
	}

	opt := &models.ArchiveOption{
		Format:           req.Format,
		Level:            int(req.Level),
		Password:         string(c.GetHeader(HeaderPassword)),
		VolumeSizeMB:     req.VolumeSizeMB,
		PreserveSymlinks: req.PreserveSymlinks,
		Conflict:         req.Conflict,
	}
	if err := opt.NormalizeForCompress(filepath.Base(req.Destination)); err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}

	pasteParam := &models.PasteParam{
		Owner:   owner,
		Action:  common.ActionCompress,
		Src:     srcs[0], // routing hint; per-task validation uses Srcs
		Srcs:    srcs,
		Dst:     dst,
		Archive: opt,
	}

	handler := drivers.Adaptor.NewFileHandler(srcs[0].FileType, &base.HandlerParam{})
	if handler == nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "no handler for storage"})
		return
	}

	task, err := handler.Compress(pasteParam)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}

	resp := archmodel.CompressResp{Code: 0, Msg: "success", TaskId: task.Id()}
	c.JSON(consts.StatusOK, resp)
}

// ExtractMethod handles POST /api/archive/:node/extract.
func ExtractMethod(ctx context.Context, c *app.RequestContext) {
	var req archmodel.ExtractReq
	if err := c.BindAndValidate(&req); err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}
	owner := string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "user not found"})
		return
	}

	src, err := models.CreateFileParam(owner, req.Source)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("source param error: %v", err)})
		return
	}
	if !common.ListContains(common.PosixFileTypes, src.FileType) {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive only supported on local storages"})
		return
	}
	dst, err := models.CreateFileParam(owner, req.Destination)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("destination param error: %v", err)})
		return
	}
	if !common.ListContains(common.PosixFileTypes, dst.FileType) {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive only supported on local storages"})
		return
	}

	opt := &models.ArchiveOption{
		Format:           req.Format,
		Password:         string(c.GetHeader(HeaderPassword)),
		PreserveSymlinks: req.PreserveSymlinks,
		Conflict:         req.Conflict,
	}
	if err := opt.NormalizeForExtract(filepath.Base(req.Source)); err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}

	pasteParam := &models.PasteParam{
		Owner:   owner,
		Action:  common.ActionExtract,
		Src:     src,
		Dst:     dst,
		Archive: opt,
	}

	handler := drivers.Adaptor.NewFileHandler(src.FileType, &base.HandlerParam{})
	if handler == nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "no handler for storage"})
		return
	}

	task, err := handler.Extract(pasteParam)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}

	resp := archmodel.ExtractResp{Code: 0, Msg: "success", TaskId: task.Id()}
	c.JSON(consts.StatusOK, resp)
}

// ----------------------------------------------------------------------
// Streaming preview
// ----------------------------------------------------------------------

// EntriesMethod handles GET /api/archive/:node/entries?source=<uri>.
//
// Response is NDJSON: one Entry object per line, then a final
// {"_done":true,"total":N} line. Errors mid-stream are surfaced as
// {"_error":"...","code":"<code>"} on a line of their own.
func EntriesMethod(ctx context.Context, c *app.RequestContext) {
	var req EntriesReq
	if err := c.BindAndValidate(&req); err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}
	owner := string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "user not found"})
		return
	}

	src, err := models.CreateFileParam(owner, req.Source)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("source param error: %v", err)})
		return
	}
	if !common.ListContains(common.PosixFileTypes, src.FileType) {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive only supported on local storages"})
		return
	}
	uri, err := src.GetResourceUri()
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	absPath := uri + src.Path

	password := string(c.GetHeader(HeaderPassword))

	rd, err := reader.Open(absPath, password)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	defer rd.Close()

	c.SetContentType("application/x-ndjson; charset=utf-8")
	c.Response.Header.Set("Cache-Control", "no-store")
	c.Response.Header.Set("X-Content-Type-Options", "nosniff")
	c.SetStatusCode(http.StatusOK)

	w := c.Response.BodyWriter()
	enc := json.NewEncoder(w)
	var total int64
	streamErr := rd.Walk(ctx, func(e reader.Entry) error {
		if err := enc.Encode(e); err != nil {
			return err
		}
		total++
		c.Flush()
		return nil
	})

	if streamErr != nil {
		code := classifyStreamError(streamErr)
		_ = enc.Encode(map[string]any{"_error": streamErr.Error(), "code": code})
		c.Flush()
		klog.V(2).Infof("[archive] entries stream error after %d entries: %v", total, streamErr)
		return
	}
	_ = enc.Encode(map[string]any{"_done": true, "total": total})
	c.Flush()
}

// EntryMethod handles GET /api/archive/:node/entry?source=<uri>&path=<inner>.
// Streams the single archived entry as application/octet-stream.
func EntryMethod(ctx context.Context, c *app.RequestContext) {
	var req EntryReq
	if err := c.BindAndValidate(&req); err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}
	owner := string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "user not found"})
		return
	}

	src, err := models.CreateFileParam(owner, req.Source)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": fmt.Sprintf("source param error: %v", err)})
		return
	}
	if !common.ListContains(common.PosixFileTypes, src.FileType) {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive only supported on local storages"})
		return
	}
	uri, err := src.GetResourceUri()
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	absPath := uri + src.Path

	password := string(c.GetHeader(HeaderPassword))

	rd, err := reader.Open(absPath, password)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	defer rd.Close()

	rc, err := rd.Open(req.Path)
	if err != nil {
		code, status := classifyEntryError(err)
		c.AbortWithStatusJSON(status, utils.H{"error": err.Error(), "code": code})
		return
	}
	defer rc.Close()

	c.SetContentType("application/octet-stream")
	base := filepath.Base(req.Path)
	c.Response.Header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, base))
	c.Response.Header.Set("Cache-Control", "no-store")
	c.SetStatusCode(http.StatusOK)

	if _, copyErr := io.Copy(c.Response.BodyWriter(), rc); copyErr != nil {
		klog.V(2).Infof("[archive] entry stream copy error for %s/%s: %v", absPath, req.Path, copyErr)
	}
}

// ----------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------

// classifyStreamError maps sevenz typed errors to FE-stable codes.
func classifyStreamError(err error) string {
	switch {
	case errors.Is(err, sevenz.ErrPasswordInvalid):
		return "password_invalid"
	case errors.Is(err, sevenz.ErrPasswordRequired):
		return "password_required"
	case errors.Is(err, sevenz.ErrCorrupt):
		return "archive_corrupt"
	case errors.Is(err, sevenz.ErrVolumeMissing):
		return "volume_missing"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}
	if strings.Contains(err.Error(), "no such file") {
		return "not_found"
	}
	return "internal"
}

// classifyEntryError returns a stable error code + the HTTP status to
// surface for a /entry request failure.
func classifyEntryError(err error) (string, int) {
	switch {
	case errors.Is(err, reader.ErrEncryptedEntry):
		return "password_required", http.StatusUnauthorized
	case errors.Is(err, reader.ErrEntryTooLarge):
		return "entry_too_large", http.StatusRequestEntityTooLarge
	case errors.Is(err, sevenz.ErrPasswordInvalid):
		return "password_invalid", http.StatusUnauthorized
	case errors.Is(err, sevenz.ErrPasswordRequired):
		return "password_required", http.StatusUnauthorized
	case errors.Is(err, sevenz.ErrCorrupt):
		return "archive_corrupt", http.StatusBadRequest
	case errors.Is(err, sevenz.ErrVolumeMissing):
		return "volume_missing", http.StatusBadRequest
	}
	if strings.Contains(err.Error(), "entry not found") {
		return "not_found", http.StatusNotFound
	}
	return "internal", http.StatusInternalServerError
}
