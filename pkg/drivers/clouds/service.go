package clouds

import (
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"os"
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

func (s *service) CreateFolder(owner string, param *models.PostParam) ([]byte, error) {
	klog.Infof("[service] creatFolder, param: %s", utils.ToJson(param))

	var configName = fmt.Sprintf("%s_%s_%s", owner, param.Drive, param.Name)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}

	if config.Type == "s3" {
		if err := s.generateKeepFile(); err != nil {
			return nil, err
		}

		var srcFs = "local:/root/"
		var srcR = ".keep"
		var dstFs = s.getFs(configName, config.Type, config.Bucket, param.ParentPath)
		var dstR = param.FolderName + "/" // like subfolder/

		if err := s.command.GetOperation().Copyfile(srcFs, srcR, dstFs, dstR); err != nil {
			klog.Errorf("[service] createfolder, type: %s, dstFs: %s, dstR: %s, error: %v", config.Type, dstFs, dstR, err)
			return nil, err
		}
		return nil, nil
	}

	var fs string = s.getFs(configName, config.Type, config.Bucket, param.ParentPath)
	if err = s.command.GetOperation().Mkdir(fs, param.FolderName); err != nil {
		klog.Errorf("[service] createFolder error: %v, fs: %s", err, fs)
	}

	return nil, nil
}

func (s *service) Delete(param *models.DeleteParam) ([]byte, error) {
	return nil, nil
}

func (s *service) DownloadAsync(param *models.DownloadAsyncParam) ([]byte, error) {
	return nil, nil
}

func (s *service) FileStat(owner string, param *models.ListParam) ([]byte, error) {
	klog.Infof("[service] file stat, param: %s", utils.ToJson(param))
	var configName = fmt.Sprintf("%s_%s_%s", owner, param.Drive, param.Name)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}
	var fs = s.getFs(configName, config.Type, config.Bucket, param.Path)
	var remote = param.Path

	data, err := s.command.GetOperation().Stat(fs, remote)
	if err != nil {
		return nil, err
	}

	if data == nil || data.Item == nil {
		return nil, nil
	}

	return utils.ToBytes(data), nil
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
		klog.Errorf("[service] list, get config error: %v", err)
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
		fs = fmt.Sprintf("%s:%s", configName, strings.TrimPrefix(fileParamPath, "/"))
	} else if configType == "drive" {
		if fileParamPath == "root" {
			fs = fmt.Sprintf("%s:", configName)
		} else {
			fs = fmt.Sprintf("%s:%s", configName, fileParamPath)
		}
	}

	return fs
}

func (s *service) generateKeepFile() error {
	var p = "/root/.keep"
	if f := fileutils.FilePathExists(p); f {
		return nil
	}

	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		return err
	}

	return nil
}
