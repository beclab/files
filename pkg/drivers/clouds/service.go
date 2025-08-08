package clouds

import (
	"encoding/json"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/files"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

var (
	DefaultLocalRootPath = "/data/"
	DefaultKeepFileName  = ".keep"
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

	fmt.Println("---config---", utils.ToJson(config))
	if config.Type == "s3" {
		if err := s.generateKeepFile(); err != nil {
			return nil, err
		}

		var srcFs = fmt.Sprintf("local:%s", DefaultLocalRootPath) // "local:/data/"
		var srcR = DefaultKeepFileName
		var dstFs = s.getFs(configName, config.Type, config.Bucket, param.ParentPath)
		var dstR = param.FolderName + "/" // like subfolder/

		copyResp, err := s.command.GetOperation().Copyfile(srcFs, srcR, dstFs, dstR, nil)
		if err != nil {
			klog.Errorf("[service] createfolder, type: %s, dstFs: %s, dstR: %s, error: %v", config.Type, dstFs, dstR, err)
			return nil, err
		}

		klog.Infof("[service] createfolder success, data: %s", utils.ToJson(copyResp))

		return nil, nil
	}

	var fs string = s.getFs(configName, config.Type, config.Bucket, param.ParentPath)
	if err = s.command.GetOperation().Mkdir(fs, param.FolderName); err != nil {
		klog.Errorf("[service] createFolder error: %v, fs: %s", err, fs)
	}

	return nil, nil
}

func (s *service) Delete(owner string, parentPath string, param *models.DeleteParam) ([]byte, error) {
	klog.Infof("[service] delete, parent: %s, param: %s", parentPath, utils.ToJson(param))
	var configName = fmt.Sprintf("%s_%s_%s", owner, param.Drive, param.Name)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}

	var fs = s.getFs(configName, config.Type, config.Bucket, parentPath)
	var remote = strings.TrimPrefix(param.Path, "/")

	if err = s.command.GetOperation().Deletefile(fs, remote); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *service) DownloadAsync(owner string, param *models.DownloadAsyncParam) (int, error) {
	var srcFs, srcRmote string
	var dstFs, dstRemote string

	var configName = fmt.Sprintf("%s_%s_%s", owner, param.Drive, param.Name)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return 0, err
	}

	srcFs = s.getFs(configName, config.Type, config.Bucket, filepath.Dir(param.CloudFilePath))
	srcRmote = strings.TrimPrefix(filepath.Base(param.CloudFilePath), "/")
	dstFs = "local:" + param.LocalFolder
	dstRemote = param.LocalFileName

	var async = true
	resp, err := s.command.GetOperation().Copyfile(srcFs, srcRmote, dstFs, dstRemote, &async)
	if err != nil {
		return 0, err
	}

	if resp.JobId == nil {
		return 0, fmt.Errorf("copyfile async jobid invalid")
	}

	return *resp.JobId, nil
}

func (s *service) FileStat(owner string, param *models.ListParam) ([]byte, error) {
	var configName = fmt.Sprintf("%s_%s_%s", owner, param.Drive, param.Name)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}
	var fs = s.getFs(configName, config.Type, config.Bucket, filepath.Dir(param.Path))
	var remote = strings.TrimPrefix(param.Path, "/")

	klog.Infof("[service] file stat, param: %s, fs: %s, remote: %s", utils.ToJson(param), fs, remote)

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

func (s *service) QueryJob(jobId int) (*job.JobStatusResp, error) {

	resp, err := s.command.GetJob().Status(jobId)
	if err != nil {
		return nil, err
	}

	var data *job.JobStatusResp
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, err
	}

	return data, nil
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
		fs = fmt.Sprintf("%s:%s", configName, filepath.Join(bucket, fileParamPath))
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
	var keepfile = fmt.Sprintf("%s%s", DefaultLocalRootPath, DefaultKeepFileName)
	if f := files.FilePathExists(keepfile); f {
		return nil
	}

	if err := os.WriteFile(keepfile, []byte{}, 0o644); err != nil {
		return err
	}

	return nil
}
