package base

import (
	"context"

	"files/pkg/models"
	"files/pkg/tasks"
)

type Execute interface {
	List(contextArgs *models.HttpContextArgs) ([]byte, error)

	Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error)

	Tree(contextArgs *models.HttpContextArgs, stopChan chan struct{}, dataChan chan string) error

	// DirUsage recursively sums the file count and total byte size under
	// contextArgs.FileParam. For a regular file it reports (1, size). It
	// calls emit with running totals as it goes (throttled by the driver)
	// so the caller can stream progress; emit returning an error (e.g. the
	// client disconnected) aborts the walk and is returned as-is. The final
	// totals are returned on success.
	DirUsage(ctx context.Context, contextArgs *models.HttpContextArgs, emit func(count, size int64) error) (count, size int64, err error)

	Create(contextArgs *models.HttpContextArgs) ([]byte, error)

	Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error)

	Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error)

	Rename(contextArgs *models.HttpContextArgs) ([]byte, error)

	Edit(contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error)

	Paste(pasteParam *models.PasteParam) (*tasks.Task, error)

	// Compress submits an archive-build task. The driver implementation
	// is responsible for node routing (drive/cache/external) and
	// returning an error when the storage cannot host the operation.
	Compress(pasteParam *models.PasteParam) (*tasks.Task, error)

	// Extract submits an archive-extract task.
	Extract(pasteParam *models.PasteParam) (*tasks.Task, error)

	UploadLink(fileUploadArg *models.FileUploadArgs) ([]byte, error)

	UploadedBytes(fileUploadArg *models.FileUploadArgs) ([]byte, error)

	UploadChunks(fileUploadArg *models.FileUploadArgs) ([]byte, error)

	CheckPermission(p *models.FileParam, owner string) (models.Level, error)

	CheckPathExists(p *models.FileParam) (exists, isDir bool, err error)
}
