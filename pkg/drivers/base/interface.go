package base

import (
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/preview"
)

type Execute interface {
	List(fileParam *models.FileParam) (int, error)
	CreateFolder(fileParam *models.FileParam) (int, error)
	Rename(fileParam *models.FileParam) (int, error)

	Preview(fileParam *models.FileParam, imgSvc preview.ImgService, fileCache fileutils.FileCache) (int, error)
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
	List(param *models.ListParam) (any, error)
}

type MetadataGetter interface {
	GetFileMetaData(param *models.ListParam) (any, error)
}

type CopierMover interface {
	CopyFile(param *models.CopyFileParam) (any, error)
	MoveFile(param *models.MoveFileParam) (any, error)
}

type DeleterRenamer interface {
	Delete(param *models.DeleteParam) (any, error)
	Rename(param *models.PatchParam) (any, error)
}

type FolderCreator interface {
	CreateFolder(param *models.PostParam) (any, error)
}

type Downloader interface {
	DownloadAsync(param *models.DownloadAsyncParam) (any, error)
}

type Uploader interface {
	UploadAsync(param *models.UploadAsyncParam) (any, error)
}

type Queryer interface {
	QueryTask(param *models.QueryTaskParam) (any, error)
	QueryAccount() (any, error)
}

type Task interface {
	PauseTask(taskId string) (any, error)
	ResumeTask(taskId string) (any, error)
}
