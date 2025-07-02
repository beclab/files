package sync

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/klog/v2"
)

type SyncStorage struct {
	Handler *base.HandlerParam
	Service *Service
}

type Files struct {
	DirId    string  `json:"dir_id"`
	Items    []*File `json:"dirent_list"`
	UserPerm string  `json:"user_perm"`
	FsType   string  `json:"fileType"`
	FsExtend string  `json:"fileExtend"`
	FsPath   string  `json:"filePath"`
}

type File struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	FsType        string `json:"fileType"`
	FsExtend      string `json:"fileExtend"`
	FsPath        string `json:"filePath"`
	NumDirs       int    `json:"numDirs"`
	NumFiles      int    `json:"numFiles"`
	NumTotalFiles int    `json:"numTotalFiles"`
	ParentDir     string `json:"parent_dir"`
	Path          string `json:"path"`
	Permission    string `json:"permission"`
	Size          int64  `json:"size"`
	Type          string `json:"type"`
}

func (s *SyncStorage) List(fileParam *models.FileParam) ([]byte, error) {
	var owner = s.Handler.Owner

	klog.Infof("SYNC list, owner: %s, param: %s", owner, fileParam.Json())

	filesData, err := s.getFiles(fileParam)
	if err != nil {
		return nil, err
	}

	return utils.ToBytes(filesData), nil
}

func (s *SyncStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) (*models.PreviewHandlerResponse, error) {
	klog.Infof("SYNC preview, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())
	var seahubUrl string
	var previewSize string

	var size = queryParam.Size
	if size != "big" {
		size = "thumb"
	}
	if size == "big" {
		previewSize = "/1080"
	} else {
		previewSize = "/128"
	}
	seahubUrl = "http://127.0.0.1:80/seahub/thumbnail/" + fileParam.Extend + previewSize + fileParam.Path

	klog.Infof("SYNC preview, owner: %s, url: %s", s.Handler.Owner, seahubUrl)

	res, err := s.Service.Get(seahubUrl, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	return &models.PreviewHandlerResponse{
		Data: res,
	}, nil
}

func (s *SyncStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, map[string]string, error) {
	var owner = s.Handler.Owner

	klog.Infof("SYNC raw, owner: %s, param: %s", owner, fileParam.Json())
	fileName, ok := fileParam.IsFile()
	if !ok {
		return nil, nil, fmt.Errorf("not a file")
	}
	safeFilename := url.QueryEscape(fileName)
	safeFilename = strings.ReplaceAll(safeFilename, "+", "%20")

	dlUrl := "http://127.0.0.1:80/seahub/lib/" + fileParam.Extend + "/file" + common.EscapeAndJoin(fileParam.Path, "/") + "/" + "?dl=1"
	klog.Infof("redirect url: %s", dlUrl)

	request, err := http.NewRequest("GET", dlUrl, nil)
	if err != nil {
		return nil, nil, err
	}

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}

	var header = make(map[string]string)
	header["contentType"] = response.Header.Get("Content-Type")
	header["contentLength"] = response.Header.Get("Content-Length")
	header["fileName"] = fileName
	header["safeFileName"] = safeFilename

	return response.Body, header, nil
}

func (s *SyncStorage) Stream(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	var owner = s.Handler.Owner

	klog.Infof("SYNC stream, owner: %s, param: %s", owner, fileParam.Json())

	filesData, err := s.getFiles(fileParam)
	if err != nil {
		return err
	}

	go s.generateDirentsData(fileParam, filesData, stopChan, dataChan)

	return nil
}

func (s *SyncStorage) generateDirentsData(fileParam *models.FileParam, filesData *Files, stopChan <-chan struct{}, dataChan chan<- string) {
	defer close(dataChan)

	var streamFiles []*File
	streamFiles = append(streamFiles, filesData.Items...)

	for len(streamFiles) > 0 {
		klog.Infoln("len(A): ", len(streamFiles))
		firstItem := streamFiles[0]
		klog.Infoln("firstItem Path: ", firstItem.Path)
		klog.Infoln("firstItem Name:", firstItem.Name)

		if firstItem.Type == "dir" {
			path := firstItem.Path
			if path != "/" {
				path += "/"
			}
			var nestFileParam = &models.FileParam{
				FileType: fileParam.FileType,
				Extend:   fileParam.Extend,
				Path:     path,
			}

			var nestFilesData, err = s.getFiles(nestFileParam)
			if err != nil {
				return
			}

			streamFiles = append(nestFilesData.Items, streamFiles[1:]...)
		} else {
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

func (s *SyncStorage) getFiles(fileParam *models.FileParam) (*Files, error) {
	getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + fileParam.Extend + "/dir/?p=" + common.EscapeURLWithSpace(fileParam.Path) + "&with_thumbnail=true"

	res, err := s.Service.Get(getUrl, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	var data *Files
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	data.FsType = fileParam.FileType
	data.FsExtend = fileParam.Extend
	data.FsPath = fileParam.Path

	if data.Items != nil && len(data.Items) > 0 {
		for _, item := range data.Items {
			item.FsType = fileParam.FileType
			item.FsExtend = fileParam.Extend
			item.FsPath = fileParam.Path
		}
	}

	return data, nil
}
