package clouds

import (
	"errors"
	"files/pkg/models"
	"files/pkg/tasks"
)

// errArchiveNotSupported is shared by both Compress and Extract for
// cloud storages. The HTTP layer (handler/archive_service.go) is
// expected to validate the source filetype against
// common.PosixFileTypes and reject before getting here, but we still
// implement the interface so the type system enforces the contract.
var errArchiveNotSupported = errors.New("archive not supported on cloud storage")

func (s *CloudStorage) Compress(_ *models.PasteParam) (*tasks.Task, error) {
	return nil, errArchiveNotSupported
}

func (s *CloudStorage) Extract(_ *models.PasteParam) (*tasks.Task, error) {
	return nil, errArchiveNotSupported
}
