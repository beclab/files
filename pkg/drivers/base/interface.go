package base

import (
	"files/pkg/models"
	"files/pkg/tasks"
)

type Execute interface {
	List(contextArgs *models.HttpContextArgs) ([]byte, error)

	Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error)

	Tree(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error

	Create(contextArgs *models.HttpContextArgs) ([]byte, error)

	Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error)

	Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error)

	Rename(contextArgs *models.HttpContextArgs) ([]byte, error)

	Edit(contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error)

	Paste(pasteParam *models.PasteParam) (*tasks.Task, error)
}
