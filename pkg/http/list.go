package http

import (
	"files/pkg/drivers/base"
	"files/pkg/models"
	"io"
)

func listHandler(handler base.Execute, fileParam *models.FileParam) ([]byte, error) {
	return handler.List(fileParam)
}

func previewHandlerEx(handler base.Execute, fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	return handler.Preview(fileParam, queryParam)
}

func rawHandlerEx(handler base.Execute, fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error) {
	return handler.Raw(fileParam, queryParam)
}
