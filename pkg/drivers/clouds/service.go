package clouds

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		Recurse:    false,
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

func GenerateCloudCachePath(fileParam *models.FileParam) string {
	timeStamp := time.Now().UnixNano()
	fileName := filepath.Base(fileParam.Path)

	var fileNamePathMapping string = fmt.Sprintf("%d_%s", timeStamp, fileName)

	cachePath := filepath.Join(common.CACHE_PREFIX, global.GlobalData.GetPvcCache(fileParam.Owner), common.DefaultLocalFileCachePath, common.CloudCache, fileNamePathMapping)

	return cachePath
}

func (s *service) CreateFile(fileParam *models.FileParam, dstR string, body io.ReadCloser) ([]byte, error) {
	var createCacheFilePath = GenerateCloudCachePath(fileParam)
	if err := files.CheckCloudCreateCacheFile(createCacheFilePath, body); err != nil {
		return nil, err
	}
	defer os.Remove(createCacheFilePath)

	fsPrefix, err := s.command.GetFsPrefix(fileParam)
	if err != nil {
		return nil, err
	}

	var srcFs = fmt.Sprintf("local:%s", filepath.Dir(createCacheFilePath))
	var srcR = filepath.Base(createCacheFilePath)
	var dstFs = filepath.Join(fsPrefix, filepath.Dir(fileParam.Path)) + "/"

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

func (s *service) Rename(srcParam, dstParam *models.FileParam, driveId string) ([]byte, error) {
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

		if srcParam.IsGoogleDrive() && driveId != "" {
			var fs = srcFsPrefix
			var args = []string{
				driveId,
				dstFs + dstRemote,
			}
			klog.Infof("[service] rename file google, fs: %s, args: %v", fs, args)

			if err := s.command.GetOperation().MoveId(fs, args); err != nil {
				return nil, err
			}

		} else {
			klog.Infof("[service] rename file, srcFs: %s, srcR: %s, dstFs: %s, dstR: %s", srcFs, srcRemote, dstFs, dstRemote)

			if err := s.command.GetOperation().MoveFile(srcFs, srcRemote, dstFs, dstRemote); err != nil {
				return nil, err
			}
		}

		klog.Infof("[service] rename file done!")

	} else {
		// ~ directory
		var srcFs, dstFs string
		srcFs = srcFsPrefix + srcParam.Path
		dstFs = dstFsPrefix + dstParam.Path

		var opts = &operations.OperationsOpt{
			Recurse:    false,
			NoModTime:  true,
			NoMimeType: true,
			Metadata:   false,
		}

		klog.Infof("[service] rename dir, list first, srcFs: %s, dstFs: %s", srcFs, dstFs)

		srcItems, _ := s.command.GetOperation().List(srcFs, opts, nil)
		if srcItems == nil || srcItems.List == nil || len(srcItems.List) == 0 {
			if err := s.command.CreateEmptyDirectory(dstParam); err != nil {
				klog.Errorf("[service] rename dir, create empty dir failed, error: %v", err)
			}
		} else {
			if dstParam.IsGoogleDrive() && driveId != "" {
				srcFs = strings.TrimRight(srcFsPrefix, ":") + ",root_folder_id=" + driveId + ":"
			}

			klog.Infof("[service] rename dir, srcFs: %s, dstFs: %s", srcFs, dstFs)

			if err := s.command.GetOperation().Move(srcFs, dstFs); err != nil {
				klog.Errorf("[service] rename dir failed, srcFs: %s, dstFs: %s, error: %v", srcFs, dstFs, err)
				return nil, err
			}

			if dstParam.IsGoogleDrive() && driveId != "" {
				if err := s.command.GetOperation().RmDirs(srcFs, ""); err != nil {
					klog.Warningf("[service] rename dir, google rmdirs failed, fs: %s, err: %v", srcFs, err)
				}
			}
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

func (s *service) QueryJobCoreStat(jobId int) (*job.CoreStatsResp, error) {
	resp, err := s.command.GetJob().Stats(jobId)
	if err != nil {
		return nil, err
	}

	var data *job.CoreStatsResp
	if err = json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("unmarshal job stats error: %v", err)
	}

	return data, nil
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

func (s *service) List(fileParam *models.FileParam, driveId string) (*models.CloudListResponse, error) {
	var fs, fsPrefix string
	var err error

	if fileParam.FileType == common.GoogleDrive && driveId != "" {
		fsPrefix, err = s.command.GetGoogleDriveFsPrefix(fileParam, driveId)
		fs = fsPrefix
	} else {
		fsPrefix, err = s.command.GetFsPrefix(fileParam)
		fs = fsPrefix + fileParam.Path
	}

	if err != nil {
		return nil, err
	}

	var opts = &operations.OperationsOpt{
		NoMimeType: true,
		NoModTime:  true,
		Metadata:   false,
	}

	filePathName, _ := files.GetFileNameFromPath(fileParam.Path)

	klog.Infof("[service] list, fs: %s, param: %s", fs, common.ToJson(fileParam))

	data, err := s.command.GetOperation().List(fs, opts, nil)
	if err != nil {
		return nil, err
	}

	var result = &models.CloudListResponse{
		FileType:   fileParam.FileType,
		FileExtend: fileParam.Extend,
		FilePath:   fileParam.Path,
		Name:       filePathName,
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

func (s *service) createUploadParam(src, dst *models.FileParam, uploadFileName string, parentPath string, newFileRelativePath string) (*models.PasteParam, error) {
	klog.Infof("[service] upload, start, uploadFileName: %s, relativePath: %s, src: %s, dst: %s", uploadFileName, newFileRelativePath, common.ToJson(src), common.ToJson(dst))

	var data = &models.PasteParam{
		Owner:                   src.Owner,
		Action:                  common.ActionUpload,
		UploadToCloud:           true,
		UploadToCloudParentPath: parentPath,
		Src: &models.FileParam{
			Owner:    src.Owner,
			FileType: src.FileType,
			Extend:   src.Extend,
			Path:     src.Path,
		},
		Dst: &models.FileParam{
			Owner:    dst.Owner,
			FileType: dst.FileType,
			Extend:   dst.Extend,
			Path:     dst.Path,
		},
	}

	return data, nil
}

func (s *service) uploadReady() error {
	return nil
}
