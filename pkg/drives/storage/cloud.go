package storage

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drives/base"
	"files/pkg/drives/model"
	"fmt"
	"net/http"
)

var _ base.Lister = (*CloudStorage)(nil)

type CloudStorage struct {
	Owner          string
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

// ls
func (c *CloudStorage) List(param *model.ListParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveList)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()

	return RequestWithContext[model.ListResponse](url, http.MethodPost, &header, paramBody)
}

// get_file_meta_data
func (c *CloudStorage) GetFileMetaData(param *model.ListParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveGetFileMetaData)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.Response](url, http.MethodPost, &header, paramBody)
}

// copy_file
func (c *CloudStorage) CopyFile(param *model.CopyFileParam) (any, error) {
	// todo
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveCopyFile)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.Response](url, http.MethodPost, &header, paramBody)
}

// move_file
func (c *CloudStorage) MoveFile(param *model.MoveFileParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveMoveFile)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.Response](url, http.MethodPost, &header, paramBody)
}

// delete
func (c *CloudStorage) Delete(param *model.DeleteParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveDelete)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.Response](url, http.MethodPost, &header, paramBody)
}

// rename
func (c *CloudStorage) Rename(param *model.PatchParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveRename)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.Response](url, http.MethodPost, &header, paramBody)
}

// create_folder
func (c *CloudStorage) CreateFolder(param *model.PostParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveCreateFolder)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.Response](url, http.MethodPost, &header, paramBody)
}

// download_async
func (c *CloudStorage) DownloadAsync(param *model.DownloadAsyncParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveDownloadAsync)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.TaskResponse](url, http.MethodPost, &header, paramBody)
}

// upload_async
func (c *CloudStorage) UploadAsync(param *model.UploadAsyncParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveUploadAsync)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.TaskResponse](url, http.MethodPost, &header, paramBody)
}

// task/query/task_ids
func (c *CloudStorage) QueryTask(param *model.QueryTaskParam) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveQueryTask)

	paramBody, _ := json.Marshal(param)
	header := c.Request.Header.Clone()
	return RequestWithContext[model.TaskQueryResponse](url, http.MethodPost, &header, paramBody)
}

func (c *CloudStorage) QueryAccount() (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveQueryAccount)

	header := c.Request.Header.Clone()
	return RequestWithContext[model.AccountResponse](url, http.MethodPost, &header, nil)
}

func (c *CloudStorage) PauseTask(taskId string) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, UrlDrivePauseTask, taskId)

	header := c.Request.Header.Clone()
	return RequestWithContext[model.TaskResponse](url, http.MethodPatch, &header, nil)
}

func (c *CloudStorage) ResumeTask(taskId string) (any, error) {
	var host = common.GetHost(c.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, UrlDriveResumeTask, taskId)

	header := c.Request.Header.Clone()
	return RequestWithContext[model.TaskResponse](url, http.MethodPatch, &header, nil)
}
