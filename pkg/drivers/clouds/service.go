package clouds

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"os"
	"path/filepath"
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

func (s *service) Stat(configName, fs, remote string, isFile bool) (*operations.OperationsStat, error) {
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}

	var statFs = s.getFs(configName, config.Type, config.Bucket, fs)
	var opts = &operations.OperationsOpt{
		Metadata: true,
	}
	if isFile {
		opts.FilesOnly = true
	} else {
		opts.DirsOnly = true
	}

	klog.Infof("[service] stat, param, configName: %s, fs:%s, remote: %s, orgfs: %s", configName, statFs, remote, fs)

	statResp, err := s.command.GetOperation().Stat(statFs, remote, opts)
	if err != nil {
		return nil, err
	}

	return statResp, nil

}

func (s *service) CopyFile(fileParam *models.FileParam, prefixPath, dstR string) ([]byte, error) {
	var keepFilePath = common.DefaultLocalRootPath + common.DefaultKeepFileName
	if err := files.CheckKeepFile(keepFilePath); err != nil {
		return nil, err
	}

	fsPrefix, err := s.command.GetFsPrefix(fileParam)
	if err != nil {
		return nil, err
	}

	var srcFs = fmt.Sprintf("local:%s", common.DefaultLocalRootPath)
	var srcR = common.DefaultKeepFileName
	var dstFs = fsPrefix

	s.command.GetOperation().Copyfile(srcFs, srcR, dstFs, dstR)
	return nil, nil
}

func (s *service) CreateFolder(owner string, param *models.PostParam) ([]byte, error) {
	klog.Infof("[service] creatFolder, param: %s", common.ToJson(param))

	var configName = fmt.Sprintf("%s_%s_%s", owner, param.Drive, param.Name)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}

	if config.Type == common.RcloneTypeS3 {
		var keepFilePath = common.DefaultLocalRootPath + common.DefaultKeepFileName
		if err := files.CheckKeepFile(keepFilePath); err != nil {
			return nil, err
		}

		var srcFs = fmt.Sprintf("local:%s", common.DefaultLocalRootPath)
		var srcR = common.DefaultKeepFileName
		var dstFs = s.getFs(configName, config.Type, config.Bucket, param.ParentPath)
		var dstR = param.FolderName + "/"

		err := s.command.GetOperation().Copyfile(srcFs, srcR, dstFs, dstR)
		if err != nil {
			klog.Errorf("[service] createfolder, type: %s, dstFs: %s, dstR: %s, error: %v", config.Type, dstFs, dstR, err)
			return nil, err
		}

		klog.Infof("[service] createfolder done!")

		return nil, nil
	}

	var fs string = s.getFs(configName, config.Type, config.Bucket, param.ParentPath)
	klog.Infof("[service] createFolder, fs: %s, remote: %s", fs, param.FolderName)
	if err = s.command.GetOperation().Mkdir(fs, param.FolderName); err != nil {
		klog.Errorf("[service] createFolder error: %v, fs: %s", err, fs)
	}

	return nil, nil
}

func (s *service) Delete(owner string, parentPath string, param *models.DeleteParam) ([]byte, error) {
	klog.Infof("[service] delete, parent: %s, param: %s", parentPath, common.ToJson(param))
	var configName = fmt.Sprintf("%s_%s_%s", owner, param.Drive, param.Name)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}

	var isFile = true
	if strings.HasSuffix(param.Path, "/") {
		isFile = false
	}

	var fs, remote string

	if isFile {
		fs = s.getFs(configName, config.Type, config.Bucket, parentPath)
		remote = strings.TrimPrefix(param.Path, "/")
		if err = s.command.GetOperation().Deletefile(fs, remote); err != nil {
			return nil, err
		}

	} else {
		fs = s.getFs(configName, config.Type, config.Bucket, parentPath)
		remote = strings.Trim(param.Path, "/")

		if err = s.command.GetOperation().Purge(fs, remote); err != nil {
			return nil, err
		}
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

	resp, err := s.command.GetOperation().CopyfileAsync(srcFs, srcRmote, dstFs, dstRemote)
	if err != nil {
		return 0, err
	}

	if resp.JobId == nil {
		return 0, fmt.Errorf("copyfile async jobid invalid")
	}

	return *resp.JobId, nil
}

func (s *service) FileStat(fileParam *models.FileParam) ([]byte, error) {
	var fsPrefix, err = s.command.GetFsPrefix(fileParam)
	if err != nil {
		return nil, err
	}

	var fs, remote string
	fs = fsPrefix
	remote = fileParam.Path

	klog.Infof("[service] file stat, fs: %s, remote: %s", fs, remote)

	var opts = &operations.OperationsOpt{
		FilesOnly: true,
	}
	data, err := s.command.GetOperation().Stat(fs, remote, opts)
	if err != nil {
		return nil, err
	}

	if data == nil || data.Item == nil {
		return nil, nil
	}

	return common.ToBytes(data), nil
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

func (s *service) Rename(owner string, param *models.FileParam, srcName string, srcPrefixPath string, dstName string, isFile bool) ([]byte, error) {
	var configName = fmt.Sprintf("%s_%s_%s", owner, param.FileType, param.Extend)
	if isFile {
		var srcRemote, dstRemote string
		srcRemote = srcName
		dstRemote = dstName
		if err := s.renameFile(configName, srcPrefixPath, srcRemote, dstRemote); err != nil {
			return nil, err
		}

		return nil, nil
	}

	var dst = &models.FileParam{
		Owner:    param.Owner,
		FileType: param.FileType,
		Extend:   param.Extend,
		Path:     srcPrefixPath + dstName,
	}

	if err := s.command.CreateEmptyDirectories(param, dst); err != nil {
		klog.Errorf("[service] rename, generate empty directories error: %v", err)
		return nil, err
	}

	if err := s.renameDirectory(owner, param, srcPrefixPath, srcName, dstName); err != nil {
		return nil, err
	}

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
	var configName = fmt.Sprintf("%s_%s_%s", fileParam.Owner, fileParam.FileType, fileParam.Extend)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		klog.Errorf("[service] list, get config error: %v", err)
		return nil, err
	}

	var fs string = s.getFs(configName, config.Type, config.Bucket, fileParam.Path)
	var opts = &operations.OperationsOpt{
		NoMimeType: true,
		NoModTime:  true,
		Metadata:   false,
	}

	klog.Infof("[service] list, configBucket: %s, fs: %s, param: %s", config.Bucket, fs, common.ToJson(fileParam))

	data, err := s.command.GetOperation().List(fs, opts)
	if err != nil {
		return nil, err
	}

	var result = &models.CloudListResponse{
		StatusCode: "SUCCESS",
	}

	if data == nil || data.List == nil || len(data.List) == 0 {
		result.Data = []*models.CloudResponseData{}
		return result, nil
	}

	var files []*models.CloudResponseData
	for _, item := range data.List {
		var f = &models.CloudResponseData{
			ID:       item.ID,
			FsType:   fileParam.FileType,
			FsExtend: fileParam.Extend,
			FsPath:   fileParam.Path,
			Path:     fileParam.Path + item.Name, //item.Path,
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

	result.Data = files

	return result, nil
}

func (s *service) getFs(configName, configType string, configBucket string, fileParamPath string) string {
	var fs string
	var bucket string
	if configType == common.RcloneTypeS3 {
		bucket = configBucket
		if !strings.HasPrefix(fileParamPath, "/") {
			fileParamPath = "/" + fileParamPath
		}
		fs = fmt.Sprintf("%s:%s%s", configName, bucket, fileParamPath)
	} else if configType == common.RcloneTypeDropbox {
		if fileParamPath == "/" {
			fileParamPath = ""
		}
		fs = fmt.Sprintf("%s:%s", configName, fileParamPath)
	} else if configType == common.RcloneTypeDrive {
		fs = fmt.Sprintf("%s:%s", configName, fileParamPath)
	}

	return fs
}

func (s *service) generateKeepFile() error {
	var keepfile = fmt.Sprintf("%s%s", common.DefaultLocalRootPath, common.DefaultKeepFileName)
	if f := files.FilePathExists(keepfile); f {
		return nil
	}

	if err := os.WriteFile(keepfile, []byte{}, 0o644); err != nil {
		return err
	}

	return nil
}

func (s *service) renameFile(configName string, srcPrefixPath string, srcRemote, dstRemote string) error {
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return err
	}

	var srcFs string
	var dstFs string

	srcFs = s.getFs(configName, config.Type, config.Bucket, srcPrefixPath)
	dstFs = s.getFs(configName, config.Type, config.Bucket, srcPrefixPath)

	err = s.command.GetOperation().MoveFile(srcFs, srcRemote, dstFs, dstRemote)
	if err != nil {
		klog.Errorf("[service] rename file, configName: %s, srcFs: %s, srcR: %s, dstFs: %s, dstR: %s, error: %v", configName, srcFs, srcRemote, dstFs, dstRemote, err)
		return err
	}

	klog.Infof("[service] rename file, configName: %s, srcFs: %s, srcR: %s, dstFs: %s, dstR: %s", configName, srcFs, srcRemote, dstFs, dstRemote)

	return err
}

func (s *service) renameDirectory(owner string, param *models.FileParam, srcPrefixPath string, srcName, dstName string) error {
	var configName = fmt.Sprintf("%s_%s_%s", owner, param.FileType, param.Extend)
	var config, err = s.command.GetConfig().GetConfig(configName)
	if err != nil {
		return err
	}

	var srcFs, dstFs string

	srcFs = s.getFs(configName, config.Type, config.Bucket, param.Path)
	dstFs = s.getFs(configName, config.Type, config.Bucket, filepath.Join(srcPrefixPath, dstName))

	err = s.command.GetOperation().Move(srcFs, dstFs)
	if err != nil {
		klog.Errorf("[service] rename dir, owner: %s, srcFs: %s, dstFs: %s, error: %v", owner, srcFs, dstFs, err)
		return err
	}

	klog.Infof("[service] rename dir done! owner: %s, srcFs: %s, dstFs: %s", owner, srcFs, dstFs)

	var purgeSrcFs = s.getFs(configName, config.Type, config.Bucket, srcPrefixPath)
	if err = s.command.GetOperation().Purge(purgeSrcFs, srcName); err != nil {
		klog.Errorf("[service] rename dir, purge error: %v, srcFs: %s, srcRemote: %s", err, srcPrefixPath, srcName)
		return err
	}

	klog.Infof("[service] rename dir purge done! owner: %s, purgeSrcFs: %s, purgeSrcRemote: %s", owner, purgeSrcFs, srcName)

	return nil
}
