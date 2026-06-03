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
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"

	"files/pkg/archive/reader"
	"files/pkg/archive/sevenz"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	bizhandler "files/pkg/hertz/biz/handler"
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

	// Read every source, write the destination archive.
	for _, fp := range srcs {
		if !gateAccess(ctx, c, fp, models.ActionRead) {
			return
		}
		h := drivers.Adaptor.NewFileHandler(fp.FileType, &base.HandlerParam{Owner: owner})
		if exists, _, lerr := h.CheckPathExists(fp); lerr != nil || !exists {
			klog.Warningf("[archive] source not exists: owner=%s, src=%s, err=%v", owner, fp.Path, lerr)
			c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive source not exists"})
			return
		}
	}
	if !gateAccess(ctx, c, dst, models.ActionWrite) {
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

	resp := archmodel.CompressResp{Code: 0, Message: "success", TaskID: task.Id()}
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

	// out.zip -> out/
	dst.Path = extractDirPath(dst.Path)

	// Read the source archive, write the extracted tree.
	if !gateAccess(ctx, c, src, models.ActionRead) {
		return
	}
	srcHandler := drivers.Adaptor.NewFileHandler(src.FileType, &base.HandlerParam{Owner: owner})
	if exists, _, lerr := srcHandler.CheckPathExists(src); lerr != nil || !exists {
		klog.Warningf("[archive] source not exists: owner=%s, src=%s, err=%v", owner, req.Source, lerr)
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive source not exists"})
		return
	}
	if !gateAccess(ctx, c, dst, models.ActionWrite) {
		return
	}

	password := string(c.GetHeader(HeaderPassword))
	srcUri, err := src.GetResourceUri()
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	if status, body, hit := archivePasswordPreflight(ctx, srcUri+src.Path, password); hit {
		c.AbortWithStatusJSON(status, body)
		return
	}

	opt := &models.ArchiveOption{
		Format:           req.Format,
		Password:         password,
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

	resp := archmodel.ExtractResp{Code: 0, Message: "success", TaskID: task.Id()}
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
	var req archmodel.EntriesReq
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
	if !gateAccess(ctx, c, src, models.ActionRead) {
		return
	}
	srcHandler := drivers.Adaptor.NewFileHandler(src.FileType, &base.HandlerParam{Owner: owner})
	if exists, _, lerr := srcHandler.CheckPathExists(src); lerr != nil || !exists {
		klog.Warningf("[archive] source not exists: owner=%s, src=%s, err=%v", owner, req.Source, lerr)
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive source not exists"})
		return
	}
	uri, err := src.GetResourceUri()
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	absPath := uri + src.Path

	password := string(c.GetHeader(HeaderPassword))

	if status, body, hit := archivePasswordPreflight(ctx, absPath, password); hit {
		c.AbortWithStatusJSON(status, body)
		return
	}

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
	var req archmodel.EntryReq
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
	if !gateAccess(ctx, c, src, models.ActionRead) {
		return
	}
	srcHandler := drivers.Adaptor.NewFileHandler(src.FileType, &base.HandlerParam{Owner: owner})
	if exists, _, lerr := srcHandler.CheckPathExists(src); lerr != nil || !exists {
		klog.Warningf("[archive] source not exists: owner=%s, src=%s, err=%v", owner, req.Source, lerr)
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "archive source not exists"})
		return
	}
	uri, err := src.GetResourceUri()
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	absPath := uri + src.Path

	password := string(c.GetHeader(HeaderPassword))

	if status, body, hit := archivePasswordPreflight(ctx, absPath, password); hit {
		c.AbortWithStatusJSON(status, body)
		return
	}

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

// gateAccess is the authorization check for the writable archive
// endpoints. Archive is posix-only and never on the share reverse-proxy
// path, so it delegates to bizhandler.Gate with skipShare=false. Returns
// true if the request may proceed; on denial Gate writes a 403.
func gateAccess(ctx context.Context, c *app.RequestContext, fp *models.FileParam, action models.Action) bool {
	return bizhandler.Gate(ctx, c, fp, action, false, "archive")
}

// extractDirPath strips a recognised archive suffix from the extract
// destination ("/Documents/out.zip" -> "/Documents/out").
func extractDirPath(p string) string {
	base := filepath.Base(p)
	if common.ArchiveFormatFromName(base) == "" {
		return p
	}
	name, _ := common.SplitNameExt(base)
	if name == "" || name == base {
		return p
	}
	dir := filepath.Dir(p)
	switch dir {
	case ".":
		return name
	case "/":
		return "/" + name
	default:
		return dir + "/" + name
	}
}

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

// archivePasswordPreflight returns hit=true with the response when the request mismatches the archive's password requirement.
func archivePasswordPreflight(ctx context.Context, absPath, password string) (int, utils.H, bool) {
	encrypted, sample, derr := archiveDetectEncrypted(ctx, absPath)
	if derr != nil {
		return consts.StatusInternalServerError, utils.H{"error": derr.Error()}, true
	}
	switch {
	case !encrypted && password == "":
		return 0, nil, false
	case !encrypted && password != "":
		return consts.StatusBadRequest, utils.H{"code": 30003, "message": "archive does not require password"}, true
	case encrypted && password == "":
		return consts.StatusBadRequest, utils.H{"code": 30001, "message": "archive password required"}, true
	}
	if verr := archiveVerifyPassword(ctx, absPath, password, sample); verr != nil {
		if errors.Is(verr, sevenz.ErrPasswordInvalid) || errors.Is(verr, sevenz.ErrPasswordRequired) {
			return consts.StatusBadRequest, utils.H{"code": 30002, "message": "archive password is incorrect"}, true
		}
		return consts.StatusInternalServerError, utils.H{"error": verr.Error()}, true
	}
	return 0, nil, false
}

// archiveDetectEncrypted reports whether absPath needs a password and (best-effort) one encrypted entry path for verification.
func archiveDetectEncrypted(ctx context.Context, absPath string) (bool, string, error) {
	if strings.EqualFold(filepath.Ext(absPath), ".zip") {
		if zr, zerr := zip.OpenReader(absPath); zerr == nil {
			var inner string
			for _, f := range zr.File {
				if f.Flags&0x1 != 0 && !f.FileInfo().IsDir() {
					inner = f.Name
					break
				}
			}
			zr.Close()
			return inner != "", inner, nil
		}
	}
	var seen string
	werr := sevenz.Walk(ctx, sevenz.ListOpts{Src: absPath, Password: ""}, func(e sevenz.Entry) error {
		if e.Encrypted && !e.IsDir && seen == "" {
			seen = e.Path
			return io.EOF
		}
		return nil
	})
	if werr == nil || errors.Is(werr, io.EOF) {
		return seen != "", seen, nil
	}
	if errors.Is(werr, sevenz.ErrPasswordRequired) || errors.Is(werr, sevenz.ErrPasswordInvalid) {
		return true, "", nil
	}
	return false, "", werr
}

// archiveVerifyPassword runs `7z` with password to confirm decryption; sample (when non-empty) skips a full re-walk.
func archiveVerifyPassword(ctx context.Context, absPath, password, sample string) error {
	if sample == "" {
		werr := sevenz.Walk(ctx, sevenz.ListOpts{Src: absPath, Password: password}, func(e sevenz.Entry) error {
			if !e.IsDir && e.Size > 0 && sample == "" {
				sample = e.Path
				return io.EOF
			}
			return nil
		})
		if werr != nil && !errors.Is(werr, io.EOF) {
			return werr
		}
		if sample == "" {
			return nil
		}
	}
	bin, err := exec.LookPath(sevenz.Binary)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "e", "-so", "-bso0", "-bse2", "-bb0", "-y", "-p"+password, "--", absPath, sample)
	cmd.Stdout = io.Discard
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return sevenz.Classify(err, stderrBuf.String())
	}
	return nil
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
