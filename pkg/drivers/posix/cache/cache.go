package cache

import (
	"errors"
	"files/pkg/drivers/base"
	"files/pkg/drivers/posix/posix"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
)

type CacheStorage struct {
	posix   *posix.PosixStorage
	handler *base.HandlerParam
	paste   *models.PasteParam
}

func NewCacheStorage(handler *base.HandlerParam) *CacheStorage {
	var posix = posix.NewPosixStorage(handler)
	return &CacheStorage{
		posix:   posix,
		handler: handler,
	}
}

// peerHeaderOwner picks the X-Bfl-User for a cross-node probe:
// handler.Owner (set by the caller, e.g. share grantor) wins over
// p.Owner so the request FileParam stays untouched.
func (s *CacheStorage) peerHeaderOwner(p *models.FileParam) string {
	if s.handler != nil && s.handler.Owner != "" {
		return s.handler.Owner
	}
	if p != nil {
		return p.Owner
	}
	return ""
}

func (s *CacheStorage) List(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return s.posix.List(contextArgs)
}

func (s *CacheStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	return s.posix.Preview(contextArgs)
}

func (s *CacheStorage) Tree(contextArgs *models.HttpContextArgs, stopChan chan struct{}, dataChan chan string) error {
	return s.posix.Tree(contextArgs, stopChan, dataChan)
}

func (s *CacheStorage) Create(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return s.posix.Create(contextArgs)
}

func (s *CacheStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	return s.posix.Delete(fileDeleteArg)
}

func (s *CacheStorage) Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {
	return s.posix.Raw(contextArgs)
}

func (s *CacheStorage) Rename(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return s.posix.Rename(contextArgs)
}

func (s *CacheStorage) Edit(contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error) {
	return s.posix.Edit(contextArgs)
}

func (s *CacheStorage) UploadLink(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return s.posix.UploadLink(fileUploadArg)
}

func (s *CacheStorage) UploadedBytes(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return s.posix.UploadedBytes(fileUploadArg)
}

func (s *CacheStorage) UploadChunks(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return s.posix.UploadChunks(fileUploadArg)
}

func (s *CacheStorage) CheckPermission(p *models.FileParam, owner string) (models.Level, error) {
	return s.posix.CheckPermission(p, owner)
}

func (s *CacheStorage) ProbeExists(p *models.FileParam) error {
	if p == nil || p.Extend == "" || p.Extend == global.CurrentNodeName {
		return s.posix.ProbeExists(p)
	}
	_, err := posix.PeerStat(p, s.peerHeaderOwner(p), false)
	if err == nil {
		return nil
	}
	var statusErr *posix.PeerStatusError
	if errors.As(err, &statusErr) {
		return fmt.Errorf("remote source not found: %s/%s%s (remote status %d)",
			p.FileType, p.Extend, p.Path, statusErr.Code)
	}
	return err
}

func (s *CacheStorage) ProbeIsDir(p *models.FileParam) (bool, error) {
	if p == nil || p.Extend == "" || p.Extend == global.CurrentNodeName {
		return s.posix.ProbeIsDir(p)
	}
	isDir, err := posix.PeerStat(p, s.peerHeaderOwner(p), true)
	if err == nil {
		return isDir, nil
	}
	var statusErr *posix.PeerStatusError
	if errors.As(err, &statusErr) {
		return false, fmt.Errorf("remote share target not found: %s/%s%s (remote status %d)",
			p.FileType, p.Extend, p.Path, statusErr.Code)
	}
	return false, err
}

func (s *CacheStorage) ProbeWrite(dst *models.FileParam) error {
	if dst == nil || dst.Extend == "" || dst.Extend == global.CurrentNodeName {
		return s.posix.ProbeWrite(dst)
	}
	return posix.PeerProbeWrite(dst)
}
