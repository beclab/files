package clouds

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/diskcache"
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/drivers/posix/upload"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/preview"
	"files/pkg/tasks"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/afero"
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

/**
 * ~ List
 */
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

	return common.ToBytes(fileData), nil
}

/**
 * ~ Preview
 */
func (s *CloudStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner
	var path = fileParam.Path

	var _, isFile = files.GetFileNameFromPath(path)

	if !isFile {
		return nil, fmt.Errorf("not a file")
	}

	klog.Infof("Cloud preview, user: %s, args: %s", owner, common.ToJson(contextArgs))

	res, err := s.service.FileStat(fileParam)
	if err != nil {
		return nil, fmt.Errorf("service get file meta error: %v", err)
	}
	if res == nil { // file not found
		return nil, fmt.Errorf("file %s not exists", path)
	}

	var fileMeta *operations.OperationsStat
	if err := json.Unmarshal(res, &fileMeta); err != nil {
		return nil, err
	}

	klog.Infof("Cloud preview, file meta: %s", string(res))

	fileType := common.MimeTypeByExtension(fileMeta.Item.Name)
	if !strings.HasPrefix(fileType, "image") {
		return nil, fmt.Errorf("can't create preview for %s type", fileType)
	}

	var previewCacheName = fileParam.FileType + fileParam.Extend + fileMeta.Item.Path + fileMeta.Item.ModTime + queryParam.PreviewSize
	var key = diskcache.GenerateCacheKey(previewCacheName)
	cachedData, ok, err := preview.GetPreviewCache(owner, key, common.CacheThumb)
	if err != nil {
		klog.Errorf("Cloud preview, get cache failed, user: %s, error: %v", owner, err)

	} else if ok {

		klog.Infof("Cloud preview, get cache, file: %s, cache name: %s, exists: %v", fileMeta.Item.Path, previewCacheName, ok)

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
	}

	previewCachedPath := diskcache.GenerateCacheBufferPath(owner, fileMeta.Item.Path)

	if !files.FilePathExists(previewCachedPath) {
		if err := files.MkdirAllWithChown(nil, previewCachedPath, 0755); err != nil {
			klog.Errorln(err)
			return nil, err
		}
	}

	var downloader = NewDownloader(s.handler.Ctx, s.service, fileParam, fileMeta.Item.Name, fileMeta.Item.Path, fileMeta.Item.Size, previewCachedPath)
	if err := downloader.download(); err != nil {
		return nil, err
	}

	var imageFilePath = filepath.Join(previewCachedPath, fileMeta.Item.Name)

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

	klog.Infof("Cloud preview, download success, file path: %s", imageFilePath)

	switch file.Type {
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

/**
 * ~ Raw
 */
func (s *CloudStorage) Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {
	var err error

	var fileParam = contextArgs.FileParam
	var user = fileParam.Owner
	var pathPrefix = files.GetPrefixPath(fileParam.Path)
	var fileName, isFile = files.GetFileNameFromPath(fileParam.Path)
	if !isFile {
		return nil, fmt.Errorf("not a file")
	}

	fileParam.Path = pathPrefix + fileName

	var path = pathPrefix + url.PathEscape(fileName)

	klog.Infof("Cloud raw, user: %s, path: %s, args: %s", user, path, common.ToJson(contextArgs))

	var configName = fmt.Sprintf("%s_%s_%s", user, fileParam.FileType, fileParam.Extend)
	res, err := s.service.FileStat(fileParam)
	if err != nil {
		return nil, fmt.Errorf("service get file meta error: %v", err)
	}

	if res == nil {
		return nil, fmt.Errorf("file %s not exists", path)
	}

	var fileMeta *operations.OperationsStat
	if err := json.Unmarshal(res, &fileMeta); err != nil {
		return nil, err
	}

	klog.Infof("Cloud raw, file meta: %s", string(res))

	var serve = s.service.command.GetServe().Get(configName, path, &contextArgs.QueryParam.Header)

	var modTime, _ = time.Parse(time.RFC3339Nano, fileMeta.Item.ModTime)
	return &models.RawHandlerResponse{
		IsCloud:      true,
		ReadCloser:   serve.Body,
		StatusCode:   serve.StatusCode,
		RespHeader:   serve.Header,
		FileName:     fileMeta.Item.Name,
		FileLength:   fileMeta.Item.Size,
		FileModified: modTime,
	}, nil
}

/**
 * ~ Tree
 */
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

/**
 * ~ Create
 */
func (s *CloudStorage) Create(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	prefixPath := files.GetPrefixPath(fileParam.Path)
	fileName, isFile := files.GetFileNameFromPath(fileParam.Path)

	klog.Infof("Cloud create, user: %s, param: %s, prefixPath: %s, name: %s, isFile: %v", owner, fileParam.Json(), prefixPath, fileName, isFile)

	var createName, createExt string
	_, isDstFile := fileParam.IsFile()

	if isDstFile {
		createName, createExt = common.SplitNameExt(fileName)
	}

	fsPrefix, err := s.service.command.GetFsPrefix(fileParam)
	if err != nil {
		return nil, err
	}

	var opts = &operations.OperationsOpt{
		Recurse:    false,
		NoModTime:  true,
		NoMimeType: true,
		Metadata:   false,
	}
	var filter = &operations.OperationsFilter{
		FilterRule: s.service.command.FormatFilter(createName, true, true, true),
	}

	existsItems, err := s.service.command.GetMatchedItems(fsPrefix+prefixPath, opts, filter)
	if err != nil {
		klog.Errorf("Cloud create, user: %s, get dst matched items error: %v", owner, err)
		return nil, err
	}

	var dupNames []string
	if existsItems != nil && existsItems.List != nil && len(existsItems.List) > 0 {
		for _, l := range existsItems.List {
			dupNames = append(dupNames, l.Name)
		}
	}

	newName := files.GenerateDupName(dupNames, fileName, isFile)
	if newName != "" {
		newName = newName + createExt
	} else {
		newName = fileName
	}

	klog.Infof("Cloud create, dupNames: %d, newName: %s", len(dupNames), newName)

	if !isFile {
		var dstParam = &models.FileParam{
			Owner:    fileParam.Owner,
			FileType: fileParam.FileType,
			Extend:   fileParam.Extend,
			Path:     prefixPath + newName + "/",
		}

		klog.Infof("Cloud create, dir, service request param: %s", common.ToJson(dstParam))

		res, err := s.service.CreateFolder(dstParam)
		if err != nil {
			klog.Errorf("Cloud create, dir error: %v, user: %s, path: %s", err, owner, fileParam.Path)
			return nil, err
		}
		klog.Infof("Cloud create, dir done! result: %s, user: %s, path: %s", string(res), owner, fileParam.Path)
		return nil, nil
	} else {
		if _, err := s.service.CopyFile(fileParam, prefixPath, newName); err != nil {
			klog.Errorf("Cloud create, file error: %v, dstPrefixPath: %s, newName: %s", err, prefixPath, newName)
			return nil, err
		}

		klog.Errorf("Cloud create, file done! dstPrefixPath: %s, newName: %s", prefixPath, newName)
	}

	return nil, nil
}

/**
 * ~ Delete
 */
func (s *CloudStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	var cmd = s.service.command
	var fileParam = fileDeleteArg.FileParam
	var fileType = fileParam.FileType
	_ = fileType
	var user = fileParam.Owner
	var dirents = fileDeleteArg.Dirents
	var deleteFailedPaths []string

	klog.Infof("Cloud delete, user: %s, param: %s, dirents: %v", user, common.ToJson(fileParam), dirents)

	// todo need to copy .keep file in fileParam.Path first

	var invalidPaths []string

	for _, dirent := range dirents {
		dirent = strings.TrimSpace(dirent)
		if dirent == "" || dirent == "/" || !strings.HasPrefix(dirent, "/") {
			invalidPaths = append(invalidPaths, dirent)
			break
		}
	}

	if len(invalidPaths) > 0 {
		return common.ToBytes(invalidPaths), fmt.Errorf("invalid path")
	}

	if fileParam.IsGoogleDrive() {
		fsPrefix, err := cmd.GetFsPrefix(fileParam)
		if err != nil {
			return nil, err
		}

		var fs string
		fs = fsPrefix + fileParam.Path

		items, err := cmd.GetOperation().List(fs, &operations.OperationsOpt{
			Recurse:    false,
			NoModTime:  true,
			NoMimeType: true,
			Metadata:   false,
		}, nil)
		if err != nil {
			return nil, err
		}

		if items == nil || items.List == nil || len(items.List) == 0 {
			return nil, fmt.Errorf("object not found")
		}

		sort.Slice(items.List, func(i, j int) bool {
			return items.List[i].Name < items.List[j].Name
		})

		var existsDupname bool
		var existsName string
		for _, dirent := range dirents {
			var count int
			var isDir = strings.HasSuffix(dirent, "/")
			for _, item := range items.List {

				if item.IsDir != isDir {
					continue
				}
				var tmp = strings.Trim(dirent, "/")
				if tmp == item.Name {
					count = count + 1
					existsName = item.Name
				}
			}

			if count >= 2 {
				existsDupname = true
				break
			}
		}

		if existsDupname {
			return nil, fmt.Errorf("There is a folder or file with the same name %s as the current operation. Please rename the object and ensure its name is unique.", existsName)
		}
	}

	deleteFailedPaths, _ = s.service.command.Delete(fileParam, dirents)

	if len(deleteFailedPaths) > 0 {
		return common.ToBytes(deleteFailedPaths), fmt.Errorf("delete failed paths")
	}

	return nil, nil
}

/**
 * ~ Rename
 */
func (s *CloudStorage) Rename(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var owner = contextArgs.FileParam.Owner
	var fileParam = contextArgs.FileParam
	var driveId = contextArgs.QueryParam.DriveId

	klog.Infof("Cloud rename, user: %s, param: %s, driveId: %s", owner, common.ToJson(contextArgs), driveId)

	var srcName, isSrcFile = files.GetFileNameFromPath(fileParam.Path)
	var srcPrefixPath = files.GetPrefixPath(fileParam.Path)
	var dstName, err = url.PathUnescape(contextArgs.QueryParam.Destination)
	if err != nil {
		klog.Errorf("Cloud rename, path unescape error: %v", err)
		return nil, err
	}

	if srcName == dstName {
		klog.Infof("Cloud rename, name not changed, user: %s, srcName: %s, dstName: %s", owner, srcName, dstName)
		return nil, nil
	}

	if fileParam.IsGoogleDrive() && driveId == "" {
		fsPrefix, err := s.service.command.GetFsPrefix(fileParam)
		if err != nil {
			return nil, err
		}
		fsName, _ := files.GetFileNameFromPath(fileParam.Path)
		fsPathPrefix := files.GetPrefixPath(fileParam.Path)
		var fs = fsPrefix + fsPathPrefix
		var fileter = s.service.command.FormatFilter(fsName, false, true, true)
		items, err := s.service.command.GetOperation().List(fs, &operations.OperationsOpt{
			NoModTime:  true,
			NoMimeType: true,
			Metadata:   false,
		}, &operations.OperationsFilter{
			FilterRule: fileter,
		})
		if err != nil {
			return nil, err
		}

		var count int
		if items != nil && items.List != nil && len(items.List) > 0 {
			for _, item := range items.List {
				if fsName == item.Name {
					count = count + 1
				}
			}
		}

		if count >= 2 {
			return nil, fmt.Errorf("The file or folder you selected has multiple objects with the same name. Please log in to the Google Drive website to modify the name and ensure its uniqueness.")
		}
	}

	var dstFs, dstRemote string
	_ = dstFs
	dstRemote = srcPrefixPath + dstName
	if !isSrcFile {
		dstRemote = dstRemote + "/"
	}

	var dstParam = &models.FileParam{
		Owner:    fileParam.Owner,
		FileType: fileParam.FileType,
		Extend:   fileParam.Extend,
		Path:     dstRemote,
	}

	fsPrefix, err := s.service.command.GetFsPrefix(fileParam)
	if err != nil {
		klog.Errorf("Cloud rename, user: %s, fs prefix not found, error: %v", owner, err)
		return nil, err
	}

	var opts = &operations.OperationsOpt{
		Recurse:    false,
		NoModTime:  true,
		NoMimeType: true,
		Metadata:   false,
	}

	var filter = &operations.OperationsFilter{
		FilterRule: s.service.command.FormatFilter(dstName, false, true, true),
	}

	dstItems, err := s.service.command.GetMatchedItems(fsPrefix+srcPrefixPath, opts, filter)
	if err != nil {
		klog.Errorf("Cloud rename, user: %s, get dst matched items error: %v", owner, err)
		return nil, fmt.Errorf("Path %s not found", fileParam.Path)
	}

	if dstItems != nil && dstItems.List != nil && len(dstItems.List) > 0 {
		return nil, fmt.Errorf("The name %s already exists. Please choose another name.", dstName)
	}

	klog.Infof("Cloud rename, src: %s, dst: %s", common.ToJson(fileParam), common.ToJson(dstParam))
	resp, err := s.service.Rename(fileParam, dstParam, driveId)
	if err != nil {
		// + todo
		// if strings.Contains(err.Error(), "object not found") {
		// }
		return nil, err
	}

	klog.Infof("Cloud rename, user: %s, resp: %s", owner, string(resp))

	return nil, nil
}

/**
 * ~ Edit
 */
func (s *CloudStorage) Edit(contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error) {
	return nil, errors.New("not supported")
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
		firstItemPath = firstItem.Path

		if firstItem.IsDir {
			if !strings.HasSuffix(firstItemPath, "/") {
				firstItemPath += "/"
			}
			var nestFileParam = &models.FileParam{
				Owner:    fileParam.Owner,
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

func (s *CloudStorage) getFiles(fileParam *models.FileParam) (*models.CloudListResponse, error) {
	res, err := s.service.List(fileParam)
	if err != nil {
		if strings.Contains(err.Error(), "config not found,") {
			return nil, fmt.Errorf("Query failed. Please check whether the integration account has been added. If it has been added, the integration account may be in the process of being configured. Please try again later.")
		}
		if strings.Contains(err.Error(), "directory not found") {
			return nil, fmt.Errorf("Directory does not exist.")
		}
	}

	return res, nil
}

/**
 * UploadLink
 */
func (s *CloudStorage) UploadLink(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	var user = fileUploadArg.FileParam.Owner
	var node = fileUploadArg.Node
	var from = fileUploadArg.From

	klog.Infof("Cloud uploadLink, user: %s, node: %s, from: %s, param: %s", user, node, from, common.ToJson(fileUploadArg))

	data, err := upload.HandleUploadLink(fileUploadArg.FileParam, fileUploadArg.From)

	klog.Infof("Cloud uploadLink, done! data: %s", string(data))

	return data, err
}

/**
 * UploadedBytes
 */
func (s *CloudStorage) UploadedBytes(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	var user = fileUploadArg.FileParam.Owner
	var node = fileUploadArg.Node
	var identy = fileUploadArg.Identy
	var ua = fileUploadArg.UserAgentHash

	klog.Infof("Cloud uploadBytes, user: %s, node: %s, identy: %s, ua: %s, param: %s", user, node, identy, ua, common.ToJson(fileUploadArg))

	data, err := upload.HandleUploadedBytes(fileUploadArg.FileParam, fileUploadArg.FileName, identy, ua)

	klog.Infof("Cloud uploadBytes, done! data: %s", string(data))

	return data, err
}

/**
 * UploadChunks
 */
func (s *CloudStorage) UploadChunks(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	var param = fileUploadArg.FileParam
	var chunkInfo = fileUploadArg.ChunkInfo
	var uploadId = fileUploadArg.UploadId
	var identy = chunkInfo.ResumableIdenty
	var ua = fileUploadArg.UserAgentHash

	var ranges = fileUploadArg.Ranges
	var lastChunk = chunkInfo.ResumableChunkNumber == chunkInfo.ResumableTotalChunks

	klog.Infof("Cloud uploadChunks, uploadId: %s, identy: %s, ua: %s, param: %s, parentDir: %s", uploadId, identy, ua, common.ToJson(param), chunkInfo.ParentDir)

	_, fileInfo, err := upload.HandleUploadChunks(param, uploadId, *chunkInfo, ua, ranges)
	if err != nil {
		return nil, err
	}

	if fileInfo == nil && !lastChunk {
		return common.ToBytes(&upload.FileUploadSucced{Success: true}), nil
	}

	if fileInfo == nil {
		uri, err := param.GetResourceUri()
		if err != nil {
			return nil, err
		}

		uploadPath := uri + param.Path
		fullPath := filepath.Join(uploadPath, chunkInfo.ResumableRelativePath)
		innerIdentifier := upload.MakeUid(fullPath)
		exist, info := upload.FileInfoManager.ExistFileInfo(innerIdentifier)
		if !exist {
			return nil, fmt.Errorf("upload file not exists")
		}

		var cachePvcPath = global.GlobalData.GetPvcCache(param.Owner)
		if cachePvcPath == "" {
			return nil, fmt.Errorf("pvc cache not found")
		}

		fileInfo = &upload.FileUploadState{
			Name:           chunkInfo.ResumableFilename,
			Id:             innerIdentifier,
			Size:           info.FileSize,
			UploadTempPath: filepath.Join(common.CACHE_PREFIX, cachePvcPath, common.DefaultUploadToCloudTempPath),
			FileInfo:       &info,
		}
	}

	klog.Infof("Cloud uploadChunks, phase done, tempPath: %s, data: %s", fileInfo.UploadTempPath, common.ToJson(fileInfo))

	var uploadTempPath = fileInfo.UploadTempPath
	if !strings.HasSuffix(uploadTempPath, "/") {
		uploadTempPath = uploadTempPath + "/"
	}
	var srcParam = &models.FileParam{}
	srcParam.GetFileParam(uploadTempPath)
	srcParam.Path = srcParam.Path + fileInfo.Id

	var dstParam = fileUploadArg.FileParam
	if !strings.HasSuffix(dstParam.Path, "/") {
		dstParam.Path += "/"
	}
	dstParam.Path += chunkInfo.ResumableRelativePath

	uploadParam, err := s.service.createUploadParam(srcParam, dstParam, fileInfo.Name, chunkInfo.ParentDir, fileInfo.FileInfo.FileRelativePath)
	if err != nil {
		klog.Errorf("Cloud uploadChunks, uploadId: %s, create copy param error: %v", uploadId, err)
		return nil, err
	}

	klog.Infof("Cloud uploadChunks, uploadId: %s, uploadToCloud param: %s", uploadId, common.ToJson(uploadParam))

	task := tasks.TaskManager.CreateTask(uploadParam)
	task.SetTotalSize(fileInfo.Size)
	if err := task.Execute(s.service.uploadReady, task.UploadToCloud); err != nil {
		return nil, err
	}

	taskId := task.Id()

	var result []*upload.FileUploadState
	result = append(result, &upload.FileUploadState{
		Id:     fileInfo.Id,
		Name:   fileInfo.Name,
		Size:   fileInfo.Size,
		TaskId: taskId,
	})

	klog.Infof("Cloud uploadChunks, uploadId: %s, new task %s", uploadId, taskId)

	return common.ToBytes(result), nil
}
