package clouds

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"net/http"
)

type Service struct {
	Owner          string
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

func NewService(owner string, w http.ResponseWriter, r *http.Request) *Service {
	return &Service{
		Owner:          owner,
		ResponseWriter: w,
		Request:        r,
	}
}

func (s *Service) CopyFile(param *models.CopyFileParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveCopyFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// CreateFolder implements drivers.ServiceInterface.
func (s *Service) CreateFolder(param *models.PostParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveCreateFolder)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// Delete implements drivers.ServiceInterface.
func (s *Service) Delete(param *models.DeleteParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveDelete)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// DownloadAsync implements drivers.ServiceInterface.
func (s *Service) DownloadAsync(param *models.DownloadAsyncParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveDownloadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// GetFileMetaData implements drivers.ServiceInterface.
func (s *Service) GetFileMetaData(param *models.ListParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveGetFileMetaData)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// List implements drivers.ServiceInterface.
func (s *Service) List(param *models.ListParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveList)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// MoveFile implements drivers.ServiceInterface.
func (s *Service) MoveFile(param *models.MoveFileParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveMoveFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// PauseTask implements drivers.ServiceInterface.
func (s *Service) PauseTask(taskId string) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, common.UrlDrivePauseTask, taskId)

	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPatch, &header, nil)
}

// QueryAccount implements drivers.ServiceInterface.
func (s *Service) QueryAccount() ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveQueryAccount)

	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, nil)
}

// QueryTask implements drivers.ServiceInterface.
func (s *Service) QueryTask(param *models.QueryTaskParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveQueryTask)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// Rename implements drivers.ServiceInterface.
func (s *Service) Rename(param *models.PatchParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveRename)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}

// ResumeTask implements drivers.ServiceInterface.
func (s *Service) ResumeTask(taskId string) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, common.UrlDriveResumeTask, taskId)

	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPatch, &header, nil)
}

// UploadAsync implements drivers.ServiceInterface.
func (s *Service) UploadAsync(param *models.UploadAsyncParam) ([]byte, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveUploadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	return utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
}
