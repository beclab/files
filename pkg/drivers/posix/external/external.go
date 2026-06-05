package external

import (
	"context"
	"errors"
	"files/pkg/drivers/base"
	"files/pkg/drivers/posix/posix"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
)

type ExternalStorage struct {
	posix *posix.PosixStorage
	paste *models.PasteParam
}

func NewExternalStorage(handler *base.HandlerParam) *ExternalStorage {
	var posix = posix.NewPosixStorage(handler)
	return &ExternalStorage{
		posix: posix,
	}
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

func (s *ExternalStorage) DirUsage(ctx context.Context, contextArgs *models.HttpContextArgs, emit func(count, size int64) error) (int64, int64, error) {
	return s.posix.DirUsage(ctx, contextArgs, emit)
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

func (s *ExternalStorage) CheckPathExists(p *models.FileParam) (exists, isDir bool, err error) {
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
