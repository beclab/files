package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

	return common.ToBytes(filesData), nil
}

func (s *SyncStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner

	klog.Infof("Sync preview, user: %s, args: %s", owner, common.ToJson(contextArgs))

	var seahubUrl string
	var previewSize string

	filesData, err := seahub.ViewLibFile(s.service.Request.Header.Clone(), fileParam, "dict")
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
	res, err := seahub.GetThumbnail(s.service.Request.Header.Clone(), fileParam, previewSize)
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

	klog.Infof("Sync raw, user: %s, param: %s, queryParam: %v", owner, fileParam.Json(), queryParam)
	fileName, ok := fileParam.IsFile()
	if !ok {
		return nil, fmt.Errorf("not a file")
	}

	var data []byte
	var err error

	klog.Infof("~~~Charset Debug log: queryParam.RawMeta=%s", queryParam.RawMeta)
	if queryParam.RawMeta == "true" {
		data, err = seahub.ViewLibFile(s.service.Request.Header, fileParam, "dict")
		klog.Infof("~~~Debug log: from seaserv, data = %s", string(data))
		if err != nil {
			return nil, err
		}

		var fileInfo *models.SyncFile
		if err := json.Unmarshal(data, &fileInfo); err != nil {
			klog.Errorf("~~~Debug log, unmarshal json failed, fileInfo = %s, err = %v", string(data), err)
			return nil, errors.New("file not found")
		}
	} else {
		ext := strings.ToLower(filepath.Ext(fileName))
		if queryParam.RawInline == "true" && (ext == ".txt" || ext == ".log" || ext == ".md") {
			rawData, err := seahub.ViewLibFile(s.service.Request.Header, fileParam, "dict")
			if err != nil {
				return nil, err
			}

			var result map[string]interface{}
			if err := json.Unmarshal(rawData, &result); err != nil {
				klog.Errorf("JSON parse failed: data=%s, err=%v", string(rawData), err)
			}

			fileContent, ok := result["file_content"].(string)
			if !ok {
				klog.Errorf("no file_content field: data=%s", string(rawData))
				err = errors.New("invalid file content")
			}

			if err == nil {
				data = []byte(fileContent)
				klog.Infof("~~~Debug log: get file content，length=%d", len(data))
			} else {
				data = rawData
			}
		} else {
			data, err = seahub.ViewLibFile(s.service.Request.Header, fileParam, "dl")
			if err != nil {
				return nil, err
			}
			http.Redirect(s.service.ResponseWriter, s.service.Request, string(data), http.StatusFound)
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

	klog.Infof("Sync create, owner: %s, args: %s", owner, common.ToJson(contextArgs))

	res, err := seahub.HandleDirOperation(s.service.Request.Header.Clone(), fileParam.Extend, fileParam.Path, "", "mkdir")
	if err != nil {
		klog.Errorf("Sync create error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}

	klog.Infof("Sync create success, result: %s, path: %s", string(res), fileParam.Path)

	return nil, nil
}

func (s *SyncStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	var fileParam = fileDeleteArg.FileParam
	var dirents = fileDeleteArg.Dirents
	var owner = fileParam.Owner
	var deleteFailedPaths []string

	klog.Infof("Sync delete, user: %s, param: %s, dirents: %v", owner, fileParam.Json(), dirents)

	for _, dirent := range dirents {
		res, err := seahub.HandleBatchDelete(s.service.Request.Header.Clone(), fileParam, []string{strings.Trim(dirent, "/")})
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
		return common.ToBytes(deleteFailedPaths), fmt.Errorf("delete failed paths")
	}

	return nil, nil
}

func (s *SyncStorage) Rename(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	klog.Infof("Sync rename, owner: %s, args: %s", owner, common.ToJson(contextArgs))

	var respBody []byte
	var err error
	header := s.service.Request.Header.Clone()
	repoID := fileParam.Extend
	newFilename, err := url.QueryUnescape(contextArgs.QueryParam.Destination)
	if err != nil {
		klog.Errorf("Sync rename error: %v, path: %s", err, contextArgs.QueryParam.Destination)
		return nil, err
	}
	action := "rename"
	if strings.HasSuffix(fileParam.Path, "/") {
		respBody, err = seahub.HandleDirOperation(header, repoID, fileParam.Path, newFilename, action)
	} else {
		respBody, err = seahub.HandleFileOperation(header, repoID, fileParam.Path, newFilename, action)
	}
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return respBody, nil
}

func (s *SyncStorage) Edit(contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var user = fileParam.Owner
	klog.Infof("Sync edit, user: %s, path: %s", user, fileParam.Path)

	filePrefix := files.GetPrefixPath(fileParam.Path)
	fileName, isFile := files.GetFileNameFromPath(fileParam.Path)
	if !isFile {
		return nil, fmt.Errorf("path %s is not file", fileParam.Path)
	}

	getRespBody, err := seahub.HandleUpdateLink(contextArgs.QueryParam.Header, fileParam, "api")
	if err != nil {
		return nil, err
	}

	updateLink := string(getRespBody)
	updateLink = strings.Trim(updateLink, "\"")

	updateUrl := "http://127.0.0.1:80/" + updateLink
	klog.Infoln(updateUrl)

	bodyBytes, err := io.ReadAll(contextArgs.QueryParam.Body)
	if err != nil {
		return nil, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("target_file", filepath.Join(filePrefix, fileName))
	_ = writer.WriteField("filename", fileName)
	klog.Infoln("target_file", filepath.Join(filePrefix, fileName))

	fileWriter, err := writer.CreateFormFile("files_content", fileName)
	if err != nil {
		return nil, err
	}

	if _, err = fileWriter.Write(bodyBytes); err != nil {
		return nil, err
	}

	if err = writer.Close(); err != nil {
		return nil, err
	}

	postBody, err := s.service.Get(updateUrl, "POST", body.Bytes())
	if err != nil {
		klog.Errorf("Sync edit, path: %s, error: %v", fileParam.Path, err)
		return nil, err
	}
	klog.Infof("Sync edit, path: %s, resp: %s", fileParam.Path, string(postBody))

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
			dataChan <- fmt.Sprintf("%s\n\n", common.ToJson(firstItem))
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
	res, err := seahub.HandleGetRepoDir(s.service.Request.Header.Clone(), fileParam)
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
