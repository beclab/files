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

func NewService(param *base.HandlerParam) base.CloudServiceInterface { //
	return &Service{
		Owner:          param.Request.Header.Get("X-Bfl-User"),
		ResponseWriter: param.ResponseWriter,
		Request:        param.Request,
		Data:           param.Data,
	}
}

func (s *Service) List(param *models.ListParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveList)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.CloudListResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// get_file_meta_data
func (s *Service) GetFileMetaData(param *models.ListParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveGetFileMetaData)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.CloudResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// copy_file
func (s *Service) CopyFile(param *models.CopyFileParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveCopyFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.CloudResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// move_file
func (s *Service) MoveFile(param *models.MoveFileParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveMoveFile)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.CloudResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// delete
func (s *Service) Delete(param *models.DeleteParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveDelete)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.CloudResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// rename
func (s *Service) Rename(param *models.PatchParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveRename)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.CloudResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// create_folder
func (s *Service) CreateFolder(param *models.PostParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveCreateFolder)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.CloudResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// download_async
func (s *Service) DownloadAsync(param *models.DownloadAsyncParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveDownloadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.TaskResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// upload_async
func (s *Service) UploadAsync(param *models.UploadAsyncParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveUploadAsync)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.TaskResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// task/query/task_ids
func (s *Service) QueryTask(param *models.QueryTaskParam) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveQueryTask)

	paramBody, _ := json.Marshal(param)
	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, paramBody)
	if err != nil {
		return nil, err
	}

	var data *models.TaskQueryResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (s *Service) QueryAccount() (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s", host, common.UrlDriveQueryAccount)

	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPost, &header, nil)
	if err != nil {
		return nil, err
	}

	var data *models.AccountResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (s *Service) PauseTask(taskId string) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, common.UrlDrivePauseTask, taskId)

	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPatch, &header, nil)
	if err != nil {
		return nil, err
	}

	var data *models.TaskResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (s *Service) ResumeTask(taskId string) (any, error) {
	var host = common.GetHost(s.Owner)
	var url = fmt.Sprintf("%s/%s/%s", host, common.UrlDriveResumeTask, taskId)

	header := s.Request.Header.Clone()

	res, err := utils.RequestWithContext(url, http.MethodPatch, &header, nil)
	if err != nil {
		return nil, err
	}

	var data *models.TaskResponse
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	return data, nil
}
