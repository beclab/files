package clouds

import (
	"encoding/json"
	"files/pkg/constant"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"io"
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

func (s *CloudStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	var owner = s.Handler.Owner

	klog.Infof("CLOUD preview, owner: %s, param: %s", owner, fileParam.Json())

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

	// todo task manager refactor

	return res, nil
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
