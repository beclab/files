package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/diskcache"
	"files/pkg/drivers/base"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/files"
	"files/pkg/models"
	"files/pkg/preview"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"

	"k8s.io/klog/v2"
)

type SyncStorage struct {
	handler *base.HandlerParam
	paste   *models.PasteParam
}

type Files struct {
	DirId      string  `json:"dir_id"`
	Items      []*File `json:"dirent_list"`
	UserPerm   string  `json:"user_perm"`
	FsType     string  `json:"fileType"`
	FsExtend   string  `json:"fileExtend"`
	FsPath     string  `json:"filePath"`
	Name       string  `json:"name"`
	LastModify string  `json:"last_modify"`
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
	LastModify    string `json:"last_modify"`
}

func getSyncFileType(filename string) string {
	mimetype := common.MimeTypeByExtension(filename)

	switch {
	case strings.HasPrefix(mimetype, "video"):
		return "video"
	case strings.HasPrefix(mimetype, "audio"):
		return "audio"
	case strings.HasPrefix(mimetype, "image"):
		return "image"
	case strings.HasSuffix(mimetype, "pdf"):
		return "pdf"
	case strings.HasPrefix(mimetype, "text") || strings.HasPrefix(mimetype, "application"): // 10 MB
		return "text"
	default:
		return "blob"
	}
}

func TransSyncFilesToFileInfo(filesData *Files, contextArgs *models.HttpContextArgs) *files.FileInfo {
	klog.Infof("Begin to trans")
	if filesData == nil {
		klog.Info("filesData is nil")
		return nil
	}

	var shareId = contextArgs.QueryParam.ShareId
	var sharePath = contextArgs.QueryParam.SharePath
	var sharePermission = contextArgs.QueryParam.SharePermission
	var permission, _ = common.ParseInt(sharePermission)
	klog.Infof("shareId=%s, sharePath=%s, permission=%d", shareId, sharePath, permission)

	klog.Infof("Begin to identify result base infos")
	result := &files.FileInfo{}
	result.SyncDirId = filesData.DirId
	result.Extension = ""
	result.IsDir = true

	if shareId == "" {
		result.SyncPermission = filesData.UserPerm
		result.FsType = filesData.FsType
		result.FsExtend = filesData.FsExtend
		result.Path = filesData.FsPath
		result.Name = filesData.Name
	} else {
		result.FsType = common.Share
		result.FsExtend = shareId
		result.Path = sharePath
		result.Name = sharePath
		result.SharePermission = int32(permission)
		if sharePath == "/" {
			result.Name = ""
		}
	}
	if filesData.LastModify != "" {
		klog.Infof("filesData.LastModify=%s", filesData.LastModify)
		lastModify, err := time.Parse(time.RFC3339Nano, filesData.LastModify)
		if err != nil {
			klog.Error(err)
		} else {
			result.ModTime = lastModify
		}
	} else {
		klog.Infof("filesData.LastModify is empty")
	}

	klog.Infof("Begin to identify result recursive infos")
	if filesData.Items != nil {
		listing := &files.Listing{
			Items:         []*files.FileInfo{},
			NumDirs:       0,
			NumFiles:      0,
			NumTotalFiles: 0,
			Size:          0,
			FileSize:      0,
		}

		klog.Infof("Begin to recursive listing")
		for _, item := range filesData.Items {
			resItem := &files.FileInfo{}
			resItem.SyncItemId = item.Id
			resItem.Name = item.Name
			if item.Type == "dir" {
				resItem.Extension = ""
				resItem.IsDir = true
				resItem.Type = ""
			} else {
				resItem.Extension = filepath.Ext(item.Name)
				resItem.IsDir = false
				resItem.Type = getSyncFileType(item.Name)
			}
			if shareId == "" {
				resItem.FsType = item.FsType
				resItem.FsExtend = item.FsExtend
				resItem.SyncPermission = item.Permission
				if item.Type == "dir" {
					resItem.Path = item.Path + "/"
				} else {
					resItem.Path = filepath.Join(item.ParentDir, item.Name)
				}
				resItem.SyncParentDir = item.ParentDir
			} else {
				resItem.FsType = common.Share
				resItem.FsExtend = shareId
				resItem.SharePermission = int32(permission)
				if item.Type == "dir" {
					resItem.Path = filepath.Join(sharePath, item.Name) + "/"
				} else {
					resItem.Path = filepath.Join(sharePath, item.Name)
				}
				resItem.SyncParentDir = filepath.Join(sharePath, strings.TrimPrefix(item.ParentDir, filesData.FsPath))
			}
			resItem.Size = item.Size

			if item.LastModify != "" {
				klog.Infof("resItem.LastModify=%s", item.LastModify)
				itemLastModify, itemErr := time.Parse(time.RFC3339Nano, item.LastModify)
				if itemErr != nil {
					klog.Error(itemErr)
				} else {
					resItem.ModTime = itemLastModify
				}
			} else {
				klog.Infof("item.LastModify is empty")
			}

			listing.Size += item.Size
			listing.NumDirs += item.NumDirs
			listing.NumFiles += item.NumFiles
			listing.NumTotalFiles += item.NumTotalFiles
			klog.Info("resItem=%+v", resItem)

			listing.Items = append(listing.Items, resItem)
		}
		klog.Infof("listing=%+v", listing)

		result.Listing = listing
	}
	return result
}

func NewSyncStorage(handler *base.HandlerParam) *SyncStorage {
	return &SyncStorage{
		handler: handler,
	}
}

func (s *SyncStorage) List(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var shareId = contextArgs.QueryParam.ShareId
	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	klog.Infof("Sync list, owner: %s, param: %s", owner, fileParam.Json())

	filesData, err := s.getFiles(fileParam)
	if err != nil {
		if shareId != "" && strings.Contains(err.Error(), "not found") {
			return nil, errors.New(common.ErrorMessageShareNotExists)
		}
		return nil, err
	}

	fileData := TransSyncFilesToFileInfo(filesData, contextArgs)
	return common.ToBytes(fileData), nil
}

func (s *SyncStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner

	klog.Infof("Sync preview, user: %s, args: %s", owner, common.ToJson(contextArgs))

	filesData, err := seahub.ViewLibFile(fileParam, "dict")
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

	var previewFileName = fileParam.FileType + fileParam.Extend + fileInfo.Path + time.Unix(fileInfo.LastModified, 0).String() + queryParam.PreviewSize
	klog.Infof("Preview preview, fileName: %s", previewFileName)
	var key = diskcache.GenerateCacheKey(previewFileName)

	klog.Infof("Sync preview, user: %s, fileType: %s, ext: %s, name: %s", owner, fileInfo.FileType, fileInfo.FileExt, fileInfo.FileName)

	// get cache
	cachedData, ok, err := preview.GetPreviewCache(owner, key, common.CacheThumb)
	if err != nil {
		klog.Errorf("Sync preview, get cache failed, user: %s, error: %v", owner, err)

	} else if ok {
		klog.Infof("Sync preview, get cache, file: %s, cache name: %s, exists: %v", fileInfo.Path, previewFileName, ok)

		if cachedData != nil {
			return &models.PreviewHandlerResponse{
				FileName:     fileInfo.FileName,
				FileModified: time.Unix(fileInfo.LastModified, 0),
				Data:         cachedData,
			}, nil
		}
	}

	previewCachedPath := diskcache.GenerateCacheBufferPath(owner, filepath.Dir(fileInfo.Path))

	if !files.FilePathExists(previewCachedPath) {
		if err := files.MkdirAllWithChown(nil, previewCachedPath, 0755, true, 1000, 1000); err != nil {
			klog.Errorln(err)
			return nil, err
		}
	}

	var imageFilePath = filepath.Join(previewCachedPath, fileInfo.FileName)

	// sync download
	repoId := fileParam.Extend

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("repo %s not found", repoId)
		return nil, errors.New("repo not found")
	}

	path := filepath.Clean(fileParam.Path)
	fileId, err := seaserv.GlobalSeafileAPI.GetFileIdByPath(repoId, path)
	if err != nil {
		return nil, errors.New("internal server error")
	}
	if fileId == "" {
		klog.Errorf("file %s not found", path)
		return nil, errors.New("file not found")
	}

	encrypted, err := strconv.ParseBool(repo["encrypted"])
	if err != nil {
		klog.Errorf("Error parsing repo encrypted: %v", err)
		encrypted = false
	}
	if encrypted {
		return nil, errors.New("permission denied")
	}

	username := fileParam.Owner + "@auth.local"

	permission, err := seahub.CheckFolderPermission(username, repoId, path)
	if err != nil {
		return nil, err
	}
	if permission != "rw" {
		return nil, errors.New("permission denied")
	}

	tmpFile, err := os.OpenFile(imageFilePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		klog.Errorf("Open or create temp file failed: %v", err)
		return nil, err
	}
	defer os.RemoveAll(filepath.Dir(imageFilePath))
	defer tmpFile.Close()

	token, err := seaserv.GlobalSeafileAPI.GetFileServerAccessToken(repoId, fileId, "view", "", true)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if token == "" {
		return nil, err
	}

	innerPath := seahub.GenFileGetURL(token, filepath.Base(path))
	resp, err := http.Get("http://127.0.0.1:80/" + innerPath)
	if err != nil {
		klog.Errorf("Download failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		klog.Errorf("Unexpected status: %s", resp.Status)
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		klog.Errorf("Save temp file failed: %v", err)
		return nil, err
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:       afero.NewBasePathFs(afero.NewOsFs(), imageFilePath),
		FsType:   fileParam.FileType,
		FsExtend: fileParam.Extend,
		Expand:   true,
		Content:  true,
	})
	if err != nil {
		return nil, err
	}

	klog.Infof("Sync preview, download success, file path: %s", imageFilePath)

	switch strings.ToLower(file.Type) {
	case "image":
		data, err := preview.CreatePreview(owner, key, file, queryParam)
		if err != nil {
			return nil, err
		}
		return &models.PreviewHandlerResponse{
			FileName:     file.Name,
			FileModified: file.ModTime,
			Data:         data,
		}, nil
	default:
		return nil, fmt.Errorf("can't create preview for %s type", file.Type)
	}
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

	if queryParam.RawMeta == "true" {
		data, err = seahub.ViewLibFile(fileParam, "dict")
		if err != nil {
			return nil, err
		}

		var fileInfo *models.SyncFile
		if err = json.Unmarshal(data, &fileInfo); err != nil {
			return nil, errors.New("file not found")
		}
	} else {
		_, ext := common.SplitNameExt(fileName)
		ext = strings.ToLower(ext)
		if queryParam.RawInline == "true" && (ext == ".txt" || ext == ".log" || ext == ".md") {
			rawData, err := seahub.ViewLibFile(fileParam, "dict")
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
			} else {
				data = rawData
			}
		} else {
			data, err = seahub.ViewLibFile(fileParam, "dl")
			if err != nil {
				return nil, err
			}
			return &models.RawHandlerResponse{
				FileName: string(data),
				Redirect: true,
			}, nil
		}
	}

	return &models.RawHandlerResponse{
		FileName:     fileName,
		FileModified: time.Time{},
		Reader:       bytes.NewReader(data),
		FileLength:   int64(len(data)),
	}, nil
}

func (s *SyncStorage) Tree(contextArgs *models.HttpContextArgs, stopChan chan struct{}, dataChan chan string) error {
	var fileParam = contextArgs.FileParam
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

	isFile := !strings.HasSuffix(fileParam.Path, "/")
	var res []byte
	var err error

	// both OK for dir and file
	parentDir := filepath.Dir(strings.TrimSuffix(fileParam.Path, "/"))
	if parentDir != "/" {
		parentDirSplit := strings.Split(strings.Trim(parentDir, "/"), "/")
		tempDir := "/"
		for _, dir := range parentDirSplit {
			if dir == "" || dir == "." {
				continue
			}
			tempDir = filepath.Join(tempDir, dir)
			_, err = seahub.HandleDirOperation(owner, fileParam.Extend, tempDir, "", "mkdir", false)
			if err != nil {
				klog.Errorf("Sync create error: %v, path: %s", err, fileParam.Path)
				return nil, err
			}
		}
	}

	if isFile {
		res, err = seahub.HandleFileOperation(owner, fileParam.Extend, fileParam.Path, "", "create")
		if err != nil {
			klog.Errorf("Sync create error: %v, path: %s", err, fileParam.Path)
			return nil, err
		}
		if contextArgs.QueryParam.Body != nil {
			var resJson map[string]interface{}
			err = json.Unmarshal(res, &resJson)
			if err != nil {
				klog.Error(err)
				return nil, err
			}
			contextArgs.FileParam.Path = filepath.Join(filepath.Dir(fileParam.Path), resJson["obj_name"].(string))
			klog.Infof("contextArgs.FileParam.Path: %s", contextArgs.FileParam.Path)
			_, err = s.Edit(contextArgs)
			if err != nil {
				klog.Errorf("Sync create error: %v, path: %s", err, fileParam.Path)
				return nil, err
			}
		}
	} else {
		res, err = seahub.HandleDirOperation(owner, fileParam.Extend, fileParam.Path, "", "mkdir", true)
		if err != nil {
			klog.Errorf("Sync create error: %v, path: %s", err, fileParam.Path)
			return nil, err
		}
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
		res, err := seahub.HandleBatchDelete(fileParam, []string{strings.Trim(dirent, "/")})
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
	repoID := fileParam.Extend
	newFilename, err := url.QueryUnescape(contextArgs.QueryParam.Destination)
	if err != nil {
		klog.Errorf("Sync rename error: %v, path: %s", err, contextArgs.QueryParam.Destination)
		return nil, err
	}
	action := "rename"
	if strings.HasSuffix(fileParam.Path, "/") {
		respBody, err = seahub.HandleDirOperation(owner, repoID, fileParam.Path, newFilename, action, false)
	} else {
		respBody, err = seahub.HandleFileOperation(owner, repoID, fileParam.Path, newFilename, action)
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
		klog.Errorf("Sync edit, path %s is not file", fileParam.Path)
		return nil, fmt.Errorf("path %s is not file", fileParam.Path)
	}

	getRespBody, err := seahub.HandleUpdateLink(contextArgs.FileParam, "api")
	if err != nil {
		klog.Errorf("Sync edit, update link error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}

	updateLink := string(getRespBody)
	updateLink = strings.Trim(updateLink, "\"")

	updateUrl := "http://127.0.0.1:80/" + updateLink
	klog.Infoln(updateUrl)

	bodyBytes, err := io.ReadAll(contextArgs.QueryParam.Body)
	if err != nil {
		klog.Errorf("Sync edit, read body error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("target_file", filepath.Join(filePrefix, fileName))
	_ = writer.WriteField("filename", fileName)
	klog.Infoln("target_file", filepath.Join(filePrefix, fileName))

	fileWriter, err := writer.CreateFormFile("files_content", fileName)
	if err != nil {
		klog.Errorf("Sync edit, create form file error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}

	if _, err = fileWriter.Write(bodyBytes); err != nil {
		klog.Errorf("Sync edit, write form file error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}

	if err = writer.Close(); err != nil {
		klog.Errorf("Sync edit, close writer error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}

	boundary := writer.Boundary()
	contentType := "multipart/form-data; boundary=" + boundary

	client := &http.Client{}
	req, err := http.NewRequest("POST", updateUrl, bytes.NewReader(body.Bytes()))
	if err != nil {
		klog.Errorf("Sync edit, create request error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}
	req.Header = contextArgs.QueryParam.Header.Clone()
	req.Header.Set("Content-Type", contentType)

	resp, err := client.Do(req)
	if err != nil {
		klog.Errorf("Sync edit, http request error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}

	if resp == nil {
		klog.Errorf("not get response from %s", updateUrl)
		return nil, err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Errorf("Sync edit, read response body error: %v, path: %s", err, fileParam.Path)
		return nil, err
	}

	etag := resp.Header.Get("ETag")
	if etag != "" {
		klog.Infof("ETag: %s", etag)
	} else {
		klog.Info("No ETag in response header")
	}

	klog.Infof("Sync edit, path: %s, resp: %s", fileParam.Path, string(respBody))
	defer resp.Body.Close()

	return &models.EditHandlerResponse{
		Etag: etag,
	}, nil
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
				Owner:    fileParam.Owner,
				Extend:   fileParam.Extend,
				Path:     path,
			}

			var nestFilesData, err = s.getFiles(nestFileParam)
			if err != nil {
				return
			}

			streamFiles = append(nestFilesData.Items, streamFiles[1:]...)
		} else {
			dataChan <- common.ToJson(firstItem)
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
	res, err := seahub.HandleGetRepoDir(fileParam)
	if err != nil {
		return nil, err
	}
	klog.Infof("res=%s", string(res))

	var data *Files
	if err := json.Unmarshal(res, &data); err != nil {
		return nil, err
	}

	fileName, _ := files.GetFileNameFromPath(fileParam.Path)

	data.FsType = fileParam.FileType
	data.FsExtend = fileParam.Extend
	data.FsPath = fileParam.Path
	data.Name = fileName

	if data.Items != nil && len(data.Items) > 0 {
		for _, item := range data.Items {
			item.FsType = fileParam.FileType
			item.FsExtend = fileParam.Extend
			item.FsPath = fileParam.Path
		}
	}

	return data, nil
}

func (s *SyncStorage) UploadLink(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	var user = fileUploadArg.FileParam.Owner
	var from = fileUploadArg.From
	var share = fileUploadArg.Share
	var shareType = fileUploadArg.ShareType
	var shareBy = fileUploadArg.ShareBy

	klog.Infof("Sync uploadLink, user: %s, from: %s, share: %s %s %s, param: %s", user, from, share, shareType, shareBy, common.ToJson(fileUploadArg.FileParam))

	uploadLink, err := seahub.GetUploadLink(fileUploadArg.FileParam, fileUploadArg.From, false, false)
	if err != nil {
		return nil, err
	}
	uploadLink = strings.Trim(uploadLink, "\"")
	return []byte(uploadLink), nil
}

func (s *SyncStorage) UploadedBytes(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	res, err := seahub.GetUploadedBytes(fileUploadArg.FileParam, fileUploadArg.FileName)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *SyncStorage) UploadChunks(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	// this handler of sync is implemented by seafile-server
	return nil, nil
}
