package external

import (
	"files/pkg/drivers/base"
	"files/pkg/drivers/posix/posix"
	"files/pkg/models"
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

func (s *ExternalStorage) Tree(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	return s.posix.Tree(fileParam, stopChan, dataChan)
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

func (s *ExternalStorage) Rename(fileParam *models.FileParam) (int, error) {
	return s.posix.Rename(fileParam)
}
