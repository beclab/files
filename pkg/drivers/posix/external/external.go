package external

import (
	"errors"
	"files/pkg/drivers/base"
	"files/pkg/drivers/posix/posix"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
)

type ExternalStorage struct {
	posix   *posix.PosixStorage
	handler *base.HandlerParam
	paste   *models.PasteParam
}

func NewExternalStorage(handler *base.HandlerParam) *ExternalStorage {
	var posix = posix.NewPosixStorage(handler)
	return &ExternalStorage{
		posix:   posix,
		handler: handler,
	}
}

// peerHeaderOwner: see CacheStorage.peerHeaderOwner.
func (s *ExternalStorage) peerHeaderOwner(p *models.FileParam) string {
	if s.handler != nil && s.handler.Owner != "" {
		return s.handler.Owner
	}
	if p != nil {
		return p.Owner
	}
	return ""
}

func (s *ExternalStorage) List(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return s.posix.List(contextArgs)
}

func (s *ExternalStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	return s.posix.Preview(contextArgs)
}

func (s *ExternalStorage) Tree(contextArgs *models.HttpContextArgs, stopChan chan struct{}, dataChan chan string) error {
	return s.posix.Tree(contextArgs, stopChan, dataChan)
}

func (s *ExternalStorage) Create(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return s.posix.Create(contextArgs)
}

func (s *ExternalStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	return s.posix.Delete(fileDeleteArg)
}

func (s *ExternalStorage) Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {
	return s.posix.Raw(contextArgs)
}

func (s *ExternalStorage) Rename(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return s.posix.Rename(contextArgs)
}

func (s *ExternalStorage) Edit(contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error) {
	return s.posix.Edit(contextArgs)
}

func (s *ExternalStorage) UploadLink(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return s.posix.UploadLink(fileUploadArg)
}

func (s *ExternalStorage) UploadedBytes(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return s.posix.UploadedBytes(fileUploadArg)
}

func (s *ExternalStorage) UploadChunks(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	return s.posix.UploadChunks(fileUploadArg)
}

func (s *ExternalStorage) CheckPermission(p *models.FileParam, owner string) (models.Level, error) {
	return s.posix.CheckPermission(p, owner)
}

func (s *ExternalStorage) ProbeExists(p *models.FileParam) error {
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

func (s *ExternalStorage) ProbeIsDir(p *models.FileParam) (bool, error) {
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

func (s *ExternalStorage) ProbeWrite(dst *models.FileParam) error {
	if dst == nil || dst.Extend == "" || dst.Extend == global.CurrentNodeName {
		return s.posix.ProbeWrite(dst)
	}
	return posix.PeerProbeWrite(dst)
}
