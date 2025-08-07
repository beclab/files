package cache

import (
	"files/pkg/drivers/base"
	"files/pkg/drivers/posix/posix"
	"files/pkg/models"
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

func (s *CacheStorage) Tree(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	return s.posix.Tree(fileParam, stopChan, dataChan)
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
