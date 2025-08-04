package clouds

import (
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

type service struct {
	command rclone.Interface
}

func NewService() *service {
	return &service{
		command: rclone.Command,
	}
}

func (s *service) CopyFile(param *models.CopyFileParam) ([]byte, error) {
	return nil, nil
}

func (s *service) CreateFolder(param *models.PostParam) ([]byte, error) {
	return nil, nil
}

func (s *service) Delete(param *models.DeleteParam) ([]byte, error) {
	return nil, nil
}

func (s *service) DownloadAsync(param *models.DownloadAsyncParam) ([]byte, error) {
	return nil, nil
}

func (s *service) GetFileMetaData(param *models.ListParam) ([]byte, error) {
	return nil, nil
}

func (s *service) MoveFile(param *models.MoveFileParam) ([]byte, error) {
	return nil, nil
}

func (s *service) PauseTask(taskId string) ([]byte, error) {
	return nil, nil
}

func (s *service) QueryAccount() ([]byte, error) {
	return nil, nil
}

func (s *service) Rename(param *models.PatchParam) ([]byte, error) {
	return nil, nil
}

func (s *service) UploadAsync(param *models.UploadAsyncParam) ([]byte, error) {
	return nil, nil
}

func (s *service) QueryTask(param *models.QueryTaskParam) ([]byte, error) {
	return nil, nil
}

func (s *service) List(fileParam *models.FileParam) (*models.CloudListResponse, error) {
	klog.Infof("[service] list, param: %s", utils.ToJson(fileParam))
	var configName = fmt.Sprintf("%s_%s_%s", fileParam.Owner, fileParam.FileType, fileParam.Extend)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}

	var fs string = s.getFs(configName, config.Type, config.Bucket, fileParam.Path)
	data, err := s.command.GetOperation().List(fs)
	if err != nil {
		return nil, err
	}

	if data == nil || data.List == nil || len(data.List) == 0 {
		return nil, nil
	}

	var files []*models.CloudResponseData
	for _, item := range data.List {
		var f = &models.CloudResponseData{
			ID:       item.ID,
			FsType:   fileParam.FileType,
			FsExtend: fileParam.Extend,
			FsPath:   fileParam.Path,
			Path:     item.Path,
			Name:     item.Name,
			Size:     item.Size,
			FileSize: item.Size,
			Modified: &item.ModTime,
			IsDir:    item.IsDir,
			Meta: &models.CloudResponseDataMeta{
				ID:           item.Path,
				LastModified: &item.ModTime,
				Key:          item.Name,
				Size:         item.Size,
			},
		}
		files = append(files, f)
	}

	var result = &models.CloudListResponse{
		StatusCode: "SUCCESS",
		Data:       files,
	}

	return result, nil
}

func (s *service) getFs(configName, configType string, configBucket string, fileParamPath string) string {
	var fs string
	var bucket string
	if configType == "s3" {
		bucket = configBucket
		fs = fmt.Sprintf("%s:%s/%s", configName, bucket, strings.TrimPrefix(fileParamPath, "/"))
	} else if configType == "dropbox" {
		bucket = ""
		fs = fmt.Sprintf("%s:", configName)
	} else if configType == "drive" {
		bucket = ""
		if fileParamPath == "root" {
			fs = fmt.Sprintf("%s:", configName)
		} else {
			fs = fmt.Sprintf("%s:%s", configName, fileParamPath)
		}
	}

	return fs
}
