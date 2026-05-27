package precheck

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/global"
	"files/pkg/integration"
	"files/pkg/models"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ErrShareTargetIsFile signals the path resolves to a regular file.
// Callers translate this to common.ErrorMessageFileShareNotSupport.
var ErrShareTargetIsFile = errors.New("share target is a file, not a folder")

// VerifyShareTargetIsDir verifies fp points at a directory on a
// share-able backend. Trailing-slash spoofing is defeated because we
// ignore the slash and ask the backend directly.
//
// Only drive + sync + posix-on-disk family (cache/external/internal/
// smb/usb/hdd) are share-able: lifecycle adjust funcs (Rename/Delete/
// Move) only handle those, so anything else (cloud, unknown) is
// rejected here.
func VerifyShareTargetIsDir(fp *models.FileParam) error {
	if fp == nil {
		return errors.New("file param is nil")
	}
	switch fp.FileType {
	case common.Sync:
		return verifySyncDir(fp)
	case common.Drive:
		return verifyLocalDir(fp)
	case common.Cache, common.External, common.Internal, common.Smb, common.Usb, common.Hdd:
		if fp.Extend == "" || fp.Extend == global.CurrentNodeName {
			return verifyLocalDir(fp)
		}
		return verifyRemoteDir(fp)
	default:
		return fmt.Errorf("share target type not supported: %s", fp.FileType)
	}
}

func verifyLocalDir(fp *models.FileParam) error {
	uri, err := fp.GetResourceUri()
	if err != nil {
		return fmt.Errorf("resolve uri: %w", err)
	}
	full := uri + fp.Path
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("share target not found: %s/%s%s", fp.FileType, fp.Extend, fp.Path)
		}
		return fmt.Errorf("stat share target %s: %w", full, err)
	}
	if !info.IsDir() {
		return ErrShareTargetIsFile
	}
	return nil
}

func verifySyncDir(fp *models.FileParam) error {
	if fp.Extend == "" {
		return errors.New("sync share target invalid: repo id is empty")
	}
	repo, err := seaserv.GlobalSeafileAPI.GetRepo(fp.Extend)
	if err != nil {
		return fmt.Errorf("sync repo lookup: %w", err)
	}
	if repo == nil {
		return fmt.Errorf("sync share target not found: repo %s", fp.Extend)
	}
	p := strings.TrimRight(fp.Path, "/")
	if p == "" {
		return nil
	}
	dirID, dErr := seaserv.GlobalSeafileAPI.GetDirIdByPath(fp.Extend, p, false)
	if dErr == nil && dirID != "" {
		return nil
	}
	fileID, fErr := seaserv.GlobalSeafileAPI.GetFileIdByPath(fp.Extend, p)
	if fErr == nil && fileID != "" {
		return ErrShareTargetIsFile
	}
	if dErr != nil {
		return fmt.Errorf("sync dir lookup: %w", dErr)
	}
	if fErr != nil {
		return fmt.Errorf("sync file lookup: %w", fErr)
	}
	return fmt.Errorf("sync share target not found: sync/%s%s", fp.Extend, fp.Path)
}

func verifyRemoteDir(fp *models.FileParam) error {
	pod, err := integration.IntegrationManager().GetFilesPod(fp.Extend)
	if err != nil {
		return fmt.Errorf("locate files pod for node %s: %w", fp.Extend, err)
	}
	if pod.Status.PodIP == "" {
		return fmt.Errorf("files pod for node %s has no IP yet", fp.Extend)
	}
	url := fmt.Sprintf("http://%s/api/resources/%s/%s%s",
		pod.Status.PodIP, fp.FileType, fp.Extend, fp.Path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build remote stat request: %w", err)
	}
	if fp.Owner != "" {
		req.Header.Set(common.REQUEST_HEADER_OWNER, fp.Owner)
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := remoteClient.Do(req)
	if err != nil {
		return fmt.Errorf("remote stat %s: %w", url, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote share target not found: %s/%s%s (remote status %d)",
			fp.FileType, fp.Extend, fp.Path, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return fmt.Errorf("read remote stat response: %w", err)
	}
	// isDir is the contract field on files.FileInfo (pkg/files/file.go).
	var probe struct {
		IsDir bool `json:"isDir"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return fmt.Errorf("parse remote stat response: %w", err)
	}
	if !probe.IsDir {
		return ErrShareTargetIsFile
	}
	return nil
}
