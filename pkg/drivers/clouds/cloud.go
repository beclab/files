package clouds

import (
	"encoding/json"
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/files"
	"files/pkg/models"
	"files/pkg/preview"
	"files/pkg/utils"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type CloudStorage struct {
	handler *base.HandlerParam
	service *service
	paste   *models.PasteParam
}

func NewCloudStorage(handlerParam *base.HandlerParam) *CloudStorage {
	return &CloudStorage{
		handler: handlerParam,
		service: NewService(),
	}
}

func (s *CloudStorage) List(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	klog.Infof("Cloud list, user: %s, param: %s", owner, fileParam.Json())

	fileData, err := s.getFiles(fileParam)
	if err != nil {
		return nil, err
	}

	if fileData != nil && fileData.Data != nil && len(fileData.Data) > 0 {
		for _, item := range fileData.Data {
			item.FsType = fileParam.FileType
			item.FsExtend = fileParam.Extend
			item.FsPath = fileParam.Path
		}
	}

	return utils.ToBytes(fileData), nil
}

// + todo
func (s *CloudStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner

	klog.Infof("Cloud preview, user: %s, args: %s", owner, utils.ToJson(contextArgs))

	var path = fileParam.Path
	if strings.HasSuffix(path, "/") {
		return nil, fmt.Errorf("can't preview folder")
	}

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  path,
	}

	res, err := s.service.FileStat(owner, data)
	if err != nil {
		return nil, fmt.Errorf("service get file meta error: %v", err)
	}
	if res == nil { // file not found
		return nil, nil
	}

	var fileMeta *operations.OperationsStat
	if err := json.Unmarshal(res, &fileMeta); err != nil {
		return nil, err
	}

	klog.Infof("Cloud preview, query: %s, file meta: %s", utils.ToJson(data), string(res))

	fileType := utils.MimeTypeByExtension(fileMeta.Item.Name)
	if !strings.HasPrefix(fileType, "image") {
		return nil, fmt.Errorf("can't create preview for %s type", fileType)
	}

	previewCacheKey := preview.GeneratePreviewCacheKey(fileMeta.Item.Path, fileMeta.Item.ModTime, queryParam.PreviewSize)

	cachedData, ok, err := preview.GetCache(previewCacheKey)
	if err != nil {
		return nil, err
	}

	klog.Infof("Cloud preview cached, file: %s, key: %s, exists: %v", fileMeta.Item.Path, previewCacheKey, ok)
	var mods = fileMeta.Item.ModTime
	modeTime, err := time.Parse(time.RFC3339Nano, mods)
	if err != nil {
		return nil, err
	}

	if cachedData != nil {
		return &models.PreviewHandlerResponse{
			FileName:     fileMeta.Item.Name,
			FileModified: modeTime,
			Data:         cachedData,
		}, nil
	}

	fileTargetPath := CreateFileDownloadFolder(owner, fileMeta.Item.Path)

	if !files.FilePathExists(fileTargetPath) {
		if err := files.MkdirAllWithChown(nil, fileTargetPath, 0755); err != nil {
			klog.Errorln(err)
			return nil, err
		}
	}

	var downloader = NewDownloader(s.handler.Ctx, s.service, fileParam, fileMeta.Item.Name, fileMeta.Item.Size, fileTargetPath)
	if err := downloader.download(); err != nil {
		return nil, err
	}
	var imagePath = filepath.Join(fileTargetPath, fileMeta.Item.Name)

	klog.Infof("Cloud preview, download success, file path: %s", imagePath)

	imageData, err := preview.OpenFile(s.handler.Ctx, imagePath, queryParam.PreviewSize)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := preview.SetCache(previewCacheKey, imageData); err != nil {
			klog.Errorf("Cloud preview, set cache error: %v, file: %s", err, fileMeta.Item.Path)
		}
	}()

	var modTime, _ = time.Parse(time.RFC3339Nano, fileMeta.Item.ModTime)
	return &models.PreviewHandlerResponse{
		FileName:     fileMeta.Item.Name,
		FileModified: modTime, // fileMeta.Item.ModTime,
		Data:         imageData,
	}, nil
}

func (s *CloudStorage) Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var user = fileParam.Owner
	var path = fileParam.Path

	klog.Infof("Cloud raw, user: %s, path: %s, args: %s", user, path, utils.ToJson(contextArgs))

	if strings.HasSuffix(path, "/") {
		return nil, fmt.Errorf("not a file")
	}

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  path,
	}

	res, err := s.service.FileStat(user, data)
	if err != nil {
		return nil, fmt.Errorf("service get file meta error: %v", err)
	}

	var fileMeta *models.CloudResponse
	if err := json.Unmarshal(res, &fileMeta); err != nil {
		return nil, err
	}

	klog.Infof("Cloud raw, query: %s, file meta: %s", utils.ToJson(data), string(res))

	if !fileMeta.IsSuccess() {
		return nil, fmt.Errorf(fileMeta.FailMessage())
	}

	fileTargetPath := CreateFileDownloadFolder(user, fileMeta.Data.Path)

	if !files.FilePathExists(fileTargetPath) {
		if err := files.MkdirAllWithChown(nil, fileTargetPath, 0755); err != nil {
			klog.Errorln(err)
			return nil, err
		}
	}

	var downloader = NewDownloader(s.handler.Ctx, s.service, fileParam, fileMeta.Data.Name, fileMeta.Data.FileSize, fileTargetPath)
	if err := downloader.download(); err != nil {
		return nil, err
	}

	var downloadedFilePath = filepath.Join(fileTargetPath, fileMeta.Data.Name)

	f, err := os.Open(downloadedFilePath)
	if err != nil {
		return nil, err
	}

	return &models.RawHandlerResponse{
		Reader:       f,
		FileName:     fileMeta.Data.Name,
		FileModified: fileMeta.Data.Meta.ModifiedTime,
	}, nil
}

func (s *CloudStorage) Tree(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	var owner = fileParam.Owner
	klog.Infof("Cloud tree, user: %s, param: %s", owner, fileParam.Json())

	fileData, err := s.getFiles(fileParam)
	if err != nil {
		return err
	}

	if fileData.Data != nil && len(fileData.Data) > 0 {
		for _, item := range fileData.Data {
			item.FsType = fileParam.FileType
			item.FsExtend = fileParam.Extend
			item.FsPath = fileParam.Path
		}
	}

	go s.generateListingData(fileParam, fileData, stopChan, dataChan)

	return nil
}

func (s *CloudStorage) Create(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	klog.Infof("Cloud create, user: %s, param: %s", owner, fileParam.Json())

	var path = strings.TrimRight(fileParam.Path, "/")
	var parts = strings.Split(path, "/")
	var parentPath string
	var folderName string
	for i := 0; i < len(parts); i++ {
		if i < len(parts)-1 {
			parentPath += parts[i] + "/"
		}
		if i == len(parts)-1 {
			folderName = parts[i]
		}
	}

	if fileParam.FileType == utils.GoogleDrive {
		parentPath = strings.Trim(parentPath, "/")
	}

	var p = &models.PostParam{
		Drive:      fileParam.FileType,
		Name:       fileParam.Extend,
		ParentPath: parentPath,
		FolderName: folderName,
	}

	klog.Infof("Cloud create, service request param: %s", utils.ToJson(p))

	res, err := s.service.CreateFolder(owner, p)
	if err != nil {
		klog.Errorf("Cloud create, error: %v, user: %s, path: %s", err, owner, fileParam.Path)
		return nil, err
	}

	klog.Infof("Cloud create, success, result: %s, user: %s, path: %s", string(res), owner, fileParam.Path)

	return nil, nil
}

func (s *CloudStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	var owner = fileDeleteArg.FileParam.Owner
	var fileParam = fileDeleteArg.FileParam
	var fileType = fileParam.FileType
	var user = fileParam.Owner
	var dirents = fileDeleteArg.Dirents
	var deleteFailedPaths []string

	klog.Infof("Cloud delete, user: %s, param: %s", user, utils.ToJson(fileParam))

	var invalidPaths []string

	for _, dirent := range dirents {
		dirent = strings.TrimSpace(dirent)
		if dirent == "" || dirent == "/" || !strings.HasPrefix(dirent, "/") {
			invalidPaths = append(invalidPaths, dirent)
			break
		}
	}

	if len(invalidPaths) > 0 {
		return utils.ToBytes(invalidPaths), fmt.Errorf("invalid path")
	}

	for _, dp := range dirents {
		var direntPath string
		dp = strings.TrimSpace(dp)

		if fileType == utils.GoogleDrive {
			direntPath = dp
		} else {
			direntPath = fileParam.Path + strings.TrimLeft(dp, "/")
		}

		klog.Infof("Cloud delete, user: %s, dirent: %s", user, dp)

		dpd, err := url.PathUnescape(direntPath)
		if err != nil {
			klog.Errorf("Cloud delete, path unescape error: %v, path: %s", err, direntPath)
			deleteFailedPaths = append(deleteFailedPaths, dp)
			continue
		}

		var p = dpd
		if fileParam.FileType == utils.DropBox {
			p = strings.TrimRight(p, "/")
		} else if fileParam.FileType == utils.GoogleDrive {
			p = strings.Trim(p, "/")
		}

		var data = &models.DeleteParam{
			Drive: fileParam.FileType,
			Name:  fileParam.Extend,
			Path:  p,
		}

		_, err = s.service.Delete(owner, fileDeleteArg.FileParam.Path, data)
		if err != nil {
			klog.Errorf("Cloud delete, delete files error: %v, user: %s", err, user)
			deleteFailedPaths = append(deleteFailedPaths, dp)
			continue
		}

		klog.Infof("Cloud delete, delete success, user: %s, file: %s", user, p)
	}

	if len(deleteFailedPaths) > 0 {
		return utils.ToBytes(deleteFailedPaths), fmt.Errorf("delete failed paths")
	}

	return nil, nil
}

func (s *CloudStorage) Rename(contextArgs *models.HttpContextArgs) ([]byte, error) {
	return nil, nil
}

func (s *CloudStorage) generateListingData(fileParam *models.FileParam,
	files *models.CloudListResponse, stopChan <-chan struct{}, dataChan chan<- string) {
	defer close(dataChan)

	var streamFiles []*models.CloudResponseData
	streamFiles = append(streamFiles, files.Data...)

	for len(streamFiles) > 0 {
		klog.Infof("Cloud tree, files count: %d", len(streamFiles))
		firstItem := streamFiles[0]
		klog.Infof("Cloud tree, firstItem Path: %s", firstItem.Path)
		klog.Infof("Cloud tree, firstItem Name: %s", firstItem.Name)
		var firstItemPath string
		if fileParam.FileType == utils.GoogleDrive {
			firstItemPath = firstItem.Meta.ID
		} else {
			firstItemPath = firstItem.Path
		}

		if firstItem.IsDir {
			var nestFileParam = &models.FileParam{
				FileType: fileParam.FileType,
				Extend:   fileParam.Extend,
				Path:     firstItemPath,
			}

			nestFileData, err := s.getFiles(nestFileParam)
			if err != nil {
				return
			}

			streamFiles = append(nestFileData.Data, streamFiles[1:]...)
		} else {
			firstItem.FsType = fileParam.FileType
			firstItem.FsExtend = fileParam.Extend
			firstItem.FsPath = firstItem.Path
			dataChan <- fmt.Sprintf("%s\n\n", utils.ToJson(firstItem))
			streamFiles = streamFiles[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func (s *CloudStorage) getFiles(fileParam *models.FileParam) (*models.CloudListResponse, error) {
	res, err := s.service.List(fileParam)
	if err != nil {
		return nil, fmt.Errorf("service list error: %v", err)
	}

	return res, nil
}
