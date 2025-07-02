package clouds

import (
	"encoding/json"
	"files/pkg/constant"
	"files/pkg/drivers/base"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/parser"
	"files/pkg/previewer"
	"files/pkg/utils"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

type CloudStorage struct {
	Handler *base.HandlerParam
	Service base.CloudServiceInterface
}

func (s *CloudStorage) List(fileParam *models.FileParam) ([]byte, error) {
	klog.Infof("CLOUD list, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	fileData, err := s.getFiles(fileParam)
	if err != nil {
		return nil, err
	}

	if fileData.Data != nil && len(fileData.Data) > 0 {
		for _, item := range fileData.Data {
			item.FsType = fileParam.FileType
			item.FsExtend = fileParam.Extend
			item.FsPath = fileParam.Path
		}
	}

	return utils.ToBytes(fileData), nil
}

func (s *CloudStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) (*models.PreviewHandlerResponse, error) {
	var owner = s.Handler.Owner

	klog.Infof("Cloud preview, owner: %s, param: %s", owner, fileParam.Json())

	var path = fileParam.Path
	if strings.HasSuffix(path, "/") {
		return nil, fmt.Errorf("can't preview folder")
	}

	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  path,
	}

	res, err := s.Service.GetFileMetaData(data)
	if err != nil {
		return nil, fmt.Errorf("service get file meta error: %v", err)
	}

	var fileMeta *models.CloudResponse
	if err := json.Unmarshal(res, &fileMeta); err != nil {
		return nil, err
	}

	klog.Infof("Cloud preview, query: %s, file meta: %s", utils.ToJson(data), string(res))

	if !fileMeta.IsSuccess() {
		return nil, fmt.Errorf(fileMeta.FailMessage())
	}

	fileType := parser.MimeTypeByExtension(fileMeta.Data.Name)
	if !strings.HasPrefix(fileType, "image") {
		return nil, fmt.Errorf("can't create preview for %s type", fileType)
	}

	previewCacheKey := previewer.GeneratePreviewCacheKey(fileMeta.Data.Path, fileMeta.Modified(), queryParam.Size)

	cachedData, ok, err := previewer.GetCache(previewCacheKey)
	if err != nil {
		return nil, err
	}

	klog.Infof("Cloud preview cached, file: %s, key: %s, exists: %v", fileMeta.Data.Path, previewCacheKey, ok)

	if cachedData != nil {
		return &models.PreviewHandlerResponse{
			FileName:     fileMeta.Data.Name,
			FileModified: fileMeta.Modified(),
			Data:         cachedData,
		}, nil
	}

	fileTargetPath := CreateFileDownloadFolder(s.Handler.Owner, fileMeta.Data.Path)

	if !fileutils.FilePathExists(fileTargetPath) {
		if err := fileutils.MkdirAllWithChown(nil, fileTargetPath, 0755); err != nil {
			klog.Errorln(err)
			return nil, err
		}
	}

	var downloader = NewDownloader(s.Handler.Ctx, s.Handler.Owner, s.Service, fileParam, fileMeta.Data.Name, fileMeta.Data.FileSize, fileTargetPath)
	if err := downloader.download(); err != nil {
		return nil, err
	}

	var imagePath = filepath.Join(fileTargetPath, fileMeta.Data.Name)

	klog.Infof("Cloud preview, download success, file path: %s", imagePath)

	imageData, err := previewer.OpenFile(s.Handler.Ctx, imagePath, queryParam.Size)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := previewer.SetCache(previewCacheKey, imageData); err != nil {
			klog.Errorf("Cloud preview, set cache error: %v, file: %s", err, fileMeta.Data.Path)
		}
	}()

	return &models.PreviewHandlerResponse{
		FileName:     fileMeta.Data.Name,
		FileModified: fileMeta.Modified(),
		Data:         imageData,
	}, nil
}

func (s *CloudStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, map[string]string, error) {
	return nil, nil, nil
}

func (s *CloudStorage) Stream(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	klog.Infof("CLOUD stream, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

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

func (s *CloudStorage) generateListingData(fileParam *models.FileParam,
	files *models.CloudListResponse, stopChan <-chan struct{}, dataChan chan<- string) {
	defer close(dataChan)

	var streamFiles []*models.CloudResponseData
	streamFiles = append(streamFiles, files.Data...)

	for len(streamFiles) > 0 {
		klog.Infof("Cloud stream, files count: %d", len(streamFiles))
		firstItem := streamFiles[0]
		klog.Infof("Cloud stream, firstItem Path: %s", firstItem.Path)
		klog.Infof("Cloud stream, firstItem Name: %s", firstItem.Name)
		var firstItemPath string
		if fileParam.FileType == constant.GoogleDrive {
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
	var data = &models.ListParam{
		Drive: fileParam.FileType,
		Name:  fileParam.Extend,
		Path:  fileParam.Path,
	}

	res, err := s.Service.List(data)
	if err != nil {
		return nil, fmt.Errorf("service list error: %v", err)
	}

	var filesData *models.CloudListResponse
	if err := json.Unmarshal(res, &filesData); err != nil {
		return nil, err
	}

	return filesData, nil
}
