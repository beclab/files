package storage

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drives/model"
	"files/pkg/utils"
	"fmt"
	"net/http"
)

type CloudStorage struct {
	Owner          string
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

func (s *CloudStorage) List(param *model.ListParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveList)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// get_file_meta_data
func (s *CloudStorage) GetFileMetaData(param *model.ListParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveGetFileMetaData)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// copy_file
func (s *CloudStorage) CopyFile(param *model.CopyFileParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveCopyFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// move_file
func (s *CloudStorage) MoveFile(param *model.MoveFileParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveMoveFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// delete
func (s *CloudStorage) Delete(param *model.DeleteParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveDelete)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// rename
func (s *CloudStorage) Rename(param *model.PatchParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveRename)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// create_folder
func (s *CloudStorage) CreateFolder(param *model.PostParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveCreateFolder)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// download_async
func (s *CloudStorage) DownloadAsync(param *model.DownloadAsyncParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveDownloadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// upload_async
func (s *CloudStorage) UploadAsync(param *model.UploadAsyncParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveUploadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// task/query/task_ids
func (s *CloudStorage) QueryTask(param *model.QueryTaskParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveQueryTask)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

func (s *CloudStorage) QueryAccount() (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, UrlDriveQueryAccount)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPost, &header, nil)
}

func (s *CloudStorage) PauseTask(taskId string) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, UrlDrivePauseTask, taskId)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPatch, &header, nil)
}

func (s *CloudStorage) ResumeTask(taskId string) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, UrlDriveResumeTask, taskId)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext(url, http.MethodPatch, &header, nil)
}
