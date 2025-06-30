package base

import (
	"files/pkg/models"
	"io"
)

type Execute interface {
	List(fileParam *models.FileParam) ([]byte, error)
	Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error)
	Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, error)

	// CreateFolder(fileParam *models.FileParam) (int, error)
	// Rename(fileParam *models.FileParam) (int, error)

}

type CloudServiceInterface interface {
	Lister
	MetadataGetter
	CopierMover
	DeleterRenamer
	FolderCreator
	Downloader
	Uploader
	Queryer
	Task
}

type Lister interface {
	List(param *models.ListParam) ([]byte, error)
}

type MetadataGetter interface {
	GetFileMetaData(param *models.ListParam) ([]byte, error)
}

type CopierMover interface {
	CopyFile(param *models.CopyFileParam) ([]byte, error)
	MoveFile(param *models.MoveFileParam) ([]byte, error)
}

type DeleterRenamer interface {
	Delete(param *models.DeleteParam) ([]byte, error)
	Rename(param *models.PatchParam) ([]byte, error)
}

type FolderCreator interface {
	CreateFolder(param *models.PostParam) ([]byte, error)
}

type Downloader interface {
	DownloadAsync(param *models.DownloadAsyncParam) ([]byte, error)
}

type Uploader interface {
	UploadAsync(param *models.UploadAsyncParam) ([]byte, error)
}

type Queryer interface {
	QueryTask(param *models.QueryTaskParam) ([]byte, error)
	QueryAccount() ([]byte, error)
}

type Task interface {
	PauseTask(taskId string) ([]byte, error)
	ResumeTask(taskId string) ([]byte, error)
}
