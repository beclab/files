package cache

import (
	"context"
	"errors"
	"files/pkg/drivers/base"
	"files/pkg/drivers/posix/posix"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
)

type CacheStorage struct {
	posix *posix.PosixStorage
	paste *models.PasteParam
}

func NewCacheStorage(handler *base.HandlerParam) *CacheStorage {
	var posix = posix.NewPosixStorage(handler)
	return &CacheStorage{
		posix: posix,
	}
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

func (s *CacheStorage) DirUsage(ctx context.Context, contextArgs *models.HttpContextArgs, emit func(count, size int64) error) (int64, int64, error) {
	return s.posix.DirUsage(ctx, contextArgs, emit)
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

func (s *CacheStorage) CheckPathExists(p *models.FileParam) (exists, isDir bool, err error) {
	if p == nil || p.Extend == "" || p.Extend == global.CurrentNodeName {
		return s.posix.CheckPathExists(p)
	}
	e, d, rerr := posix.RemotePathExists(p, p.Owner)
	if rerr == nil {
		return e, d, nil
	}
	var statusErr *posix.RemoteStatusError
	if errors.As(rerr, &statusErr) {
		return false, false, fmt.Errorf("remote source not found: %s/%s%s (remote status %d)",
			p.FileType, p.Extend, p.Path, statusErr.Code)
	}
	return false, false, rerr
}
