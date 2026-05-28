package sync

import (
	"errors"
	"files/pkg/models"
	"files/pkg/tasks"
)

var errArchiveNotSupported = errors.New("archive not supported on sync storage")

func (s *SyncStorage) Compress(_ *models.PasteParam) (*tasks.Task, error) {
	return nil, errArchiveNotSupported
}

func (s *SyncStorage) Extract(_ *models.PasteParam) (*tasks.Task, error) {
	return nil, errArchiveNotSupported
}
