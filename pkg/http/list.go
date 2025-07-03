package http

import (
	"files/pkg/drivers/base"
	"files/pkg/models"
	"io"
)

func listHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.List(contextArgs)
}

func rawHandlerEx(handler base.Execute, fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, map[string]string, error) {
	return handler.Raw(fileParam, queryParam)
}

func createHandler(handler base.Execute, contextArgs *models.HttpContextArgs) ([]byte, error) {
	return handler.Create(contextArgs)
}
