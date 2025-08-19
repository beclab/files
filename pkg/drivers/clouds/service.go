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

func (s *service) Stat(param *models.FileParam) (*operations.OperationsStat, error) {
	_, isFile := files.GetFileNameFromPath(param.Path)
	var opts = &operations.OperationsOpt{
		NoModTime:  true,
		NoMimeType: true,
		Metadata:   false,
	}
	if isFile {
		opts.FilesOnly = true
	} else {
		opts.DirsOnly = true
	}
	fsPrefix, err := s.command.GetFsPrefix(param)
	if err != nil {
		return nil, err
	}

	var fs, remote string = fsPrefix, strings.TrimPrefix(param.Path, "/")

	klog.Infof("[service] stat, param, fs:%s, remote: %s", fs, remote)

	statResp, err := s.command.GetOperation().Stat(fs, remote, opts)
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

func (s *service) CreateFolder(param *models.FileParam) ([]byte, error) {
	klog.Infof("[service] creatFolder, param: %s", common.ToJson(param))

	fsPrefix, err := s.command.GetFsPrefix(param)
	prefixPath := files.GetPrefixPath(param.Path)
	fileName, _ := files.GetFileNameFromPath(param.Path)

	if param.FileType == common.AwsS3 || param.FileType == common.TencentCos {
		var keepFilePath = common.DefaultLocalRootPath + common.DefaultKeepFileName
		if err := files.CheckKeepFile(keepFilePath); err != nil {
			return nil, err
		}

		var srcFs = fmt.Sprintf("local:%s", common.DefaultLocalRootPath)
		var srcR = common.DefaultKeepFileName
		var dstFs = fsPrefix + prefixPath
		var dstR = fileName + "/"

		err := s.command.GetOperation().Copyfile(srcFs, srcR, dstFs, dstR)
		if err != nil {
			klog.Errorf("[service] createfolder, type: %s, dstFs: %s, dstR: %s, error: %v", param.FileType, dstFs, dstR, err)
			return nil, err
		}

		klog.Infof("[service] createfolder done!")

		return nil, nil
	}

	var fs = fsPrefix + prefixPath
	var remote = fileName
	klog.Infof("[service] createFolder, fs: %s, remote: %s", fs, remote)
	if err = s.command.GetOperation().Mkdir(fs, remote); err != nil {
		klog.Errorf("[service] createFolder error: %v, fs: %s", err, fs)
	}

	return nil, nil
}

func (s *service) Delete(param *models.FileParam, dirent string) ([]byte, error) {
	fsPrefix, err := s.command.GetFsPrefix(param)
	_, isFile := files.GetFileNameFromPath(dirent)

	var fs, remote string

	if isFile {
		fs = fsPrefix + param.Path
		remote = strings.TrimPrefix(dirent, "/")
		if err = s.command.GetOperation().Deletefile(fs, remote); err != nil {
			return nil, err
		}

	} else {
		fs = fsPrefix + param.Path
		remote = strings.Trim(dirent, "/")

		if err = s.command.GetOperation().Purge(fs, remote); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *service) DownloadAsync(param *models.FileParam, localFolder, localFileName string) (int, error) {
	var srcFs, srcRemote string
	var dstFs, dstRemote string

	srcPrefixPath := files.GetPrefixPath(param.Path)
	srcFileName, _ := files.GetFileNameFromPath(param.Path)
	srcFsPrefix, err := s.command.GetFsPrefix(param)
	if err != nil {
		return 0, err
	}

	srcFs = srcFsPrefix + srcPrefixPath
	srcRemote = strings.TrimPrefix(srcFileName, "/")

	dstFs = "local:" + localFolder
	dstRemote = localFileName

	resp, err := s.command.GetOperation().CopyfileAsync(srcFs, srcRemote, dstFs, dstRemote)
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
	remote = strings.TrimPrefix(fileParam.Path, "/")

	klog.Infof("[service] file stat, fs: %s, remote: %s", fs, remote)

	var opts = &operations.OperationsOpt{
		FilesOnly:  true,
		NoMimeType: true,
		Metadata:   false,
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

func (s *service) Rename(srcParam, dstParam *models.FileParam) ([]byte, error) {
	srcFsPrefix, err := s.command.GetFsPrefix(srcParam)
	if err != nil {
		return nil, err
	}

	dstFsPrefix, err := s.command.GetFsPrefix(dstParam)
	if err != nil {
		return nil, err
	}

	srcFileName, isFile := files.GetFileNameFromPath(srcParam.Path)
	dstFileName, _ := files.GetFileNameFromPath(dstParam.Path)

	srcPrefixPath := files.GetPrefixPath(srcParam.Path)
	dstPrefixPath := files.GetPrefixPath(dstParam.Path)

	if isFile {
		var srcFs, srcRemote string
		var dstFs, dstRemote string

		srcFs = srcFsPrefix + srcPrefixPath
		srcRemote = srcFileName
		dstFs = dstFsPrefix + dstPrefixPath
		dstRemote = dstFileName

		klog.Infof("[service] rename file, srcFs: %s, srcR: %s, dstFs: %s, dstR: %s", srcFs, srcRemote, dstFs, dstRemote)

		if err := s.command.GetOperation().MoveFile(srcFs, srcRemote, dstFs, dstRemote); err != nil {
			return nil, err
		}

		klog.Infof("[service] rename file done!")

	} else {

		if err := s.command.CreateEmptyDirectories(srcParam, dstParam); err != nil {
			klog.Errorf("[service] rename, generate empty directories error: %v", err)
			return nil, err
		}

		var srcFs, dstFs string
		srcFs = srcFsPrefix + srcParam.Path
		dstFs = dstFsPrefix + dstParam.Path

		klog.Infof("[service] rename dir, srcFs: %s, dstFs: %s", srcFs, dstFs)

		if err := s.command.GetOperation().Move(srcFs, dstFs); err != nil {
			klog.Errorf("[service] rename dir failed, srcFs: %s, dstFs: %s, error: %v", srcFs, dstFs, err)
			return nil, err
		}

		var purgeFs, purgeRemote string
		purgeFs = srcFsPrefix + srcPrefixPath
		purgeRemote = srcFileName

		if err := s.command.GetOperation().Purge(purgeFs, purgeRemote); err != nil {
			klog.Errorf("[service] rename dir and purge failed, fs: %s, remote: %s, error: %v", purgeFs, purgeRemote, err)
			return nil, err
		}

		klog.Infof("[service] rename dir done!")
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
	var fsPrefix, err = s.command.GetFsPrefix(fileParam)
	if err != nil {
		return nil, err
	}
	var fs string = fsPrefix + fileParam.Path
	var opts = &operations.OperationsOpt{
		NoMimeType: true,
		NoModTime:  true,
		Metadata:   false,
	}

	klog.Infof("[service] list, fs: %s, param: %s", fs, common.ToJson(fileParam))

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
