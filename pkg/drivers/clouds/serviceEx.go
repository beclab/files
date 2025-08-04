package clouds

import (
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

type serviceEx struct {
	command rclone.Interface
}

func NewServiceEx() *serviceEx {
	return &serviceEx{
		command: rclone.Command,
	}
}

func (s *serviceEx) List(fileParam *models.FileParam) (*models.CloudListResponse, error) {
	klog.Infof("[service] list, param: %s", utils.ToJson(fileParam))
	var configName = fmt.Sprintf("%s_%s_%s", fileParam.Owner, fileParam.FileType, fileParam.Extend)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}

	var bucket string
	if config.Type == "s3" {
		bucket = config.Bucket
	} else if config.Type == "dropbox" {
		bucket = config.Name
	}

	var fs = fmt.Sprintf("%s:%s/%s", configName, bucket, strings.TrimPrefix(fileParam.Path, "/"))
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
