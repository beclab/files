package base

import (
	"files/pkg/models"
	"files/pkg/tasks"
)

type Execute interface {
	List(contextArgs *models.HttpContextArgs) ([]byte, error)

	Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error)

	Tree(contextArgs *models.HttpContextArgs, stopChan chan struct{}, dataChan chan string) error

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
}
