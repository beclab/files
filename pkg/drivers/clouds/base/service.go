package base

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"net/http"
)

type Service struct {
	Owner          string
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Data           *common.Data
}

func NewService(w http.ResponseWriter, r *http.Request, d *common.Data) base.CloudServiceInterface { //
	return &Service{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
		Data:           d,
	}
}

func (s *Service) List(param *models.ListParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveList)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext[models.CloudListResponse](url, http.MethodPost, &header, paramBody)
}

// get_file_meta_data
func (s *Service) GetFileMetaData(param *models.ListParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveGetFileMetaData)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.CloudResponse](url, http.MethodPost, &header, paramBody)
}

// copy_file
func (s *Service) CopyFile(param *models.CopyFileParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveCopyFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.CloudResponse](url, http.MethodPost, &header, paramBody)
}

// move_file
func (s *Service) MoveFile(param *models.MoveFileParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveMoveFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.CloudResponse](url, http.MethodPost, &header, paramBody)
}

// delete
func (s *Service) Delete(param *models.DeleteParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveDelete)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.CloudResponse](url, http.MethodPost, &header, paramBody)
}

// rename
func (s *Service) Rename(param *models.PatchParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveRename)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.CloudResponse](url, http.MethodPost, &header, paramBody)
}

// create_folder
func (s *Service) CreateFolder(param *models.PostParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveCreateFolder)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.CloudResponse](url, http.MethodPost, &header, paramBody)
}

// download_async
func (s *Service) DownloadAsync(param *models.DownloadAsyncParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveDownloadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskResponse](url, http.MethodPost, &header, paramBody)
}

// upload_async
func (s *Service) UploadAsync(param *models.UploadAsyncParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveUploadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskResponse](url, http.MethodPost, &header, paramBody)
}

// task/query/task_ids
func (s *Service) QueryTask(param *models.QueryTaskParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveQueryTask)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskQueryResponse](url, http.MethodPost, &header, paramBody)
}

func (s *Service) QueryAccount() (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveQueryAccount)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.AccountResponse](url, http.MethodPost, &header, nil)
}

func (s *Service) PauseTask(taskId string) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, common.UrlDrivePauseTask, taskId)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskResponse](url, http.MethodPatch, &header, nil)
}

func (s *Service) ResumeTask(taskId string) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, common.UrlDriveResumeTask, taskId)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskResponse](url, http.MethodPatch, &header, nil)
}
