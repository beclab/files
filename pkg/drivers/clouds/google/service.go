package google

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

func NewService(param *base.HandlerParam) base.CloudServiceInterface { //
	return &Service{
		Owner:          param.Request.Header.Get("X-Bfl-User"),
		ResponseWriter: param.ResponseWriter,
		Request:        param.Request,
		Data:           param.Data,
	}
}

// CopyFile implements drivers.ServiceInterface.
func (s *Service) CopyFile(param *models.CopyFileParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveCopyFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.GoogleDriveResponse](url, http.MethodPost, &header, paramBody)
}

// CreateFolder implements drivers.ServiceInterface.
func (s *Service) CreateFolder(param *models.PostParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveCreateFolder)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.GoogleDriveResponse](url, http.MethodPost, &header, paramBody)
}

// Delete implements drivers.ServiceInterface.
func (s *Service) Delete(param *models.DeleteParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveDelete)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.GoogleDriveResponse](url, http.MethodPost, &header, paramBody)
}

// DownloadAsync implements drivers.ServiceInterface.
func (s *Service) DownloadAsync(param *models.DownloadAsyncParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveDownloadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskResponse](url, http.MethodPost, &header, paramBody)
}

// GetFileMetaData implements drivers.ServiceInterface.
func (s *Service) GetFileMetaData(param *models.ListParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveGetFileMetaData)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.GoogleDriveResponse](url, http.MethodPost, &header, paramBody)
}

// List implements drivers.ServiceInterface.
func (s *Service) List(param *models.ListParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveList)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext[models.GoogleDriveListResponse](url, http.MethodPost, &header, paramBody)
}

// MoveFile implements drivers.ServiceInterface.
func (s *Service) MoveFile(param *models.MoveFileParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveMoveFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.GoogleDriveResponse](url, http.MethodPost, &header, paramBody)
}

// PauseTask implements drivers.ServiceInterface.
func (s *Service) PauseTask(taskId string) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, common.UrlDrivePauseTask, taskId)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskResponse](url, http.MethodPatch, &header, nil)
}

// QueryAccount implements drivers.ServiceInterface.
func (s *Service) QueryAccount() (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveQueryAccount)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.AccountResponse](url, http.MethodPost, &header, nil)
}

// QueryTask implements drivers.ServiceInterface.
func (s *Service) QueryTask(param *models.QueryTaskParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveQueryTask)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskQueryResponse](url, http.MethodPost, &header, paramBody)
}

// Rename implements drivers.ServiceInterface.
func (s *Service) Rename(param *models.PatchParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveRename)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.GoogleDriveResponse](url, http.MethodPost, &header, paramBody)
}

// ResumeTask implements drivers.ServiceInterface.
func (s *Service) ResumeTask(taskId string) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, common.UrlDriveResumeTask, taskId)

	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskResponse](url, http.MethodPatch, &header, nil)
}

// UploadAsync implements drivers.ServiceInterface.
func (s *Service) UploadAsync(param *models.UploadAsyncParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveUploadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()
	return utils.RequestWithContext[models.TaskResponse](url, http.MethodPost, &header, paramBody)
}
