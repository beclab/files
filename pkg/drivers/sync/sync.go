package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type SyncStorage struct {
	handler *base.HandlerParam
	service *Service
	paste   *models.PasteParam
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

func NewSyncStorage(handler *base.HandlerParam) *SyncStorage {
	return &SyncStorage{
		handler: handler,
		service: NewService(handler),
	}
}

func (s *SyncStorage) List(contextArgs *models.HttpContextArgs) ([]byte, error) {

	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	klog.Infof("Sync list, owner: %s, param: %s", owner, fileParam.Json())

	filesData, err := s.getFiles(fileParam)
	if err != nil {
		return nil, err
	}

	return utils.ToBytes(filesData), nil
}

func (s *SyncStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner

	klog.Infof("Sync preview, user: %s, args: %s", owner, utils.ToJson(contextArgs))

	var seahubUrl string
	var previewSize string

	getUrl := "http://127.0.0.1:80/seahub/lib/" + fileParam.Extend + "/file" + common.EscapeURLWithSpace(fileParam.Path) + "?dict=1"

	klog.Infof("Sync preview, user: %s, get url: %s", owner, getUrl)

	filesData, err := s.service.Get(getUrl, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	var fileInfo *models.SyncFile
	if err := json.Unmarshal(filesData, &fileInfo); err != nil {
		return nil, errors.New("file not found")
	}

	var parts = strings.Split(fileParam.Path, "/")
	var fileName = parts[len(parts)-1]
	if fileName == "" {
		return nil, fmt.Errorf("invalid path")
	}

	var size = queryParam.PreviewSize
	if size != "big" {
		size = "thumb"
	}
	if size == "big" {
		previewSize = "/1080"
	} else {
		previewSize = "/128"
	}
	seahubUrl = "http://127.0.0.1:80/seahub/thumbnail/" + fileParam.Extend + previewSize + fileParam.Path

	klog.Infof("SYNC preview, user: %s, url: %s", owner, seahubUrl)

	res, err := s.service.Get(seahubUrl, http.MethodGet, nil)
	if err != nil {
		klog.Errorf("Sync preview, get file failed, user: %s, url: %s, err: %s", owner, seahubUrl, err.Error())
		return nil, err
	}

	return &models.PreviewHandlerResponse{
		FileName:     fileName,
		FileModified: time.Time{},
		Data:         res,
	}, nil
}

func (s *SyncStorage) Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {

	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner

	klog.Infof("Sync raw, user: %s, param: %s", owner, fileParam.Json())
	fileName, ok := fileParam.IsFile()
	if !ok {
		return nil, fmt.Errorf("not a file")
	}

	var data []byte
	var err error

	if queryParam.RawMeta == "true" {
		getUrl := "http://127.0.0.1:80/seahub/lib/" + fileParam.Extend + "/file" + common.EscapeURLWithSpace(fileParam.Path) + "?dict=1"

		klog.Infof("Sync raw, user: %s, get meta url: %s", fileParam.Owner, getUrl)

		data, err = s.service.Get(getUrl, http.MethodGet, nil)
		if err != nil {
			return nil, err
		}

		var fileInfo *models.SyncFile
		if err := json.Unmarshal(data, &fileInfo); err != nil {
			return nil, errors.New("file not found")
		}
	} else {
		dlUrl := "http://127.0.0.1:80/seahub/lib/" + fileParam.Extend + "/file" + common.EscapeAndJoin(fileParam.Path, "/") + "/" + "?dl=1"

		klog.Infof("Sync raw, user: %s, get download url: %s", fileParam.Owner, dlUrl)

		data, err = s.service.Get(dlUrl, http.MethodGet, nil)
		if err != nil {
			return nil, err
		}
	}

	return &models.RawHandlerResponse{
		FileName:     fileName,
		FileModified: time.Time{},
		Reader:       bytes.NewReader(data),
	}, nil
}

func (s *SyncStorage) Tree(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	var owner = fileParam.Owner

	klog.Infof("SYNC tree, owner: %s, param: %s", owner, fileParam.Json())

	filesData, err := s.getFiles(fileParam)
	if err != nil {
		return err
	}

	go s.generateDirentsData(fileParam, filesData, stopChan, dataChan)

	return nil
}

func (s *SyncStorage) Create(contextArgs *models.HttpContextArgs) ([]byte, error) {

	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	klog.Infof("Sync create, owner: %s, args: %s", owner, utils.ToJson(contextArgs))

	p := strings.Trim(fileParam.Path, "/")
	parts := strings.Split(p, "/")
	subFolder := "/"

	for _, part := range parts {
		subFolder = filepath.Join(subFolder, part)
		if !strings.HasPrefix(subFolder, "/") {
			subFolder = "/" + subFolder
		}

		var url = "http://127.0.0.1:80/seahub/api/v2.1/repos/" + fileParam.Extend + "/dir/?p=" + common.EscapeURLWithSpace(subFolder)
		data := make(map[string]string)
		data["operation"] = "mkdir"
		res, err := s.service.Get(url, http.MethodPost, []byte(utils.ToJson(data)))
		if err != nil {
			klog.Errorf("Sync create error: %v, path: %s", err, subFolder)
			return nil, err
		}

		klog.Infof("Sync create success, result: %s, path: %s", string(res), subFolder)
	}

	return nil, nil
}

func (s *SyncStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	var fileParam = fileDeleteArg.FileParam
	var dirents = fileDeleteArg.Dirents
	var owner = fileParam.Owner
	var deleteFailedPaths []string

	klog.Infof("Sync delete, user: %s, param: %s, dirents: %v", owner, fileParam.Json(), dirents)

	for _, dirent := range dirents {
		var data = make(map[string]interface{})
		data["repo_id"] = fileParam.Extend
		data["parent_dir"] = fileParam.Path
		data["dirents"] = []string{strings.Trim(dirent, "/")}

		klog.Infof("Sync delete, delete dirent, param: %s", []byte(utils.ToJson(data)))

		deleteUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/batch-delete-item/"
		res, err := s.service.Get(deleteUrl, http.MethodDelete, []byte(utils.ToJson(data)))
		if err != nil {
			klog.Errorf("Sync delete, delete files error: %v, user: %s", err, owner)
			deleteFailedPaths = append(deleteFailedPaths, dirent)
			continue
		}

		var result = &models.SyncDeleteResponse{}
		err = json.Unmarshal(res, result)
		if err != nil {
			klog.Errorf("Sync delete, parse json error: %v, user: %s", err, owner)
			deleteFailedPaths = append(deleteFailedPaths, dirent)
		}
	}

	if len(deleteFailedPaths) > 0 {
		return utils.ToBytes(deleteFailedPaths), fmt.Errorf("delete failed paths")
	}

	return nil, nil
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

	res, err := s.service.Get(getUrl, http.MethodGet, nil)
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
