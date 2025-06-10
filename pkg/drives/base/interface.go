package base

import "files/pkg/drives/model"

type Lister interface {
	List(param *model.ListParam) (any, error)
}

type MetadataGetter interface {
	GetFileMetaData(param *model.ListParam) (any, error)
}

type CopierMover interface {
	CopyFile(param *model.GoogleDriveCopyFileParam) (any, error)
	MoveFile(param *model.MoveFileParam) (any, error)
}

type DeleterRenamer interface {
	Delete(param *model.DeleteParam) (any, error)
	Rename(oldPath, newPath string) error
}

type FolderCreator interface {
	CreateFolder(param *model.PostParam) (any, error)
}

type Downloader interface {
	DownloadAsync(param *model.DownloadAsyncParam) (any, error)
}

type Uploader interface {
	UploadAsync(param *model.UploadAsyncParam) (any, error)
}

type Queryer interface {
	QueryTask(param *model.QueryTaskParam) (any, error)
	QueryAccount() (any, error)
}

type Task interface {
	PauseTask(taskId string) (any, error)
	ResumeTask(taskId string) (any, error)
}
