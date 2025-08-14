package clouds

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/files"
	"files/pkg/models"
	"files/pkg/preview"
	"fmt"
	"net/url"
	"path/filepath"
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

func (s *CloudStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner
	var path, err = url.PathUnescape(fileParam.Path)
	if err != nil {
		return nil, fmt.Errorf("path %s urldecode error: %v", fileParam.Path, err)
	}

	var pathPrefix = common.GetPrefixPath(path)
	var fileName, isFile = common.GetFileNameFromPath(path)

	if !isFile {
		return nil, fmt.Errorf("not a file")
	}

	klog.Infof("Cloud preview, user: %s, args: %s", owner, common.ToJson(contextArgs))

	var configName = fmt.Sprintf("%s_%s_%s", owner, fileParam.FileType, fileParam.Extend)
	res, err := s.service.FileStat(configName, pathPrefix, fileName)
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

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:       afero.NewBasePathFs(afero.NewOsFs(), imagePath),
		FsType:   fileParam.FileType,
		FsExtend: fileParam.Extend,
		Expand:   true,
		Content:  true,
	})
	if err != nil {
		return nil, err
	}

	klog.Infof("Cloud preview, download success, file path: %s", imagePath)

	switch file.Type {
	case "image":
		return preview.HandleImagePreview(file, queryParam)
	default:
		return nil, fmt.Errorf("can't create preview for %s type", file.Type)
	}
}

func (s *CloudStorage) Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var user = fileParam.Owner
	var pathPrefix = common.GetPrefixPath(fileParam.Path)
	var fileName, isFile = common.GetFileNameFromPath(fileParam.Path)

	if !isFile {
		return nil, fmt.Errorf("not a file")
	}

	fileName, err := url.PathUnescape(fileName)
	if err != nil {
		return nil, fmt.Errorf("file %s urldecode error: %v", fileName, err)
	}

	fileParam.Path = pathPrefix + fileName

	fileName, _ = url.PathUnescape(fileName)
	var path = pathPrefix + fileName

	klog.Infof("Cloud raw, user: %s, path: %s, args: %s", user, path, common.ToJson(contextArgs))

	var configName = fmt.Sprintf("%s_%s_%s", user, fileParam.FileType, fileParam.Extend)
	res, err := s.service.FileStat(configName, pathPrefix, fileName)
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

	dstPrefixPath := common.GetPrefixPath(fileParam.Path)
	dstFileOrDirName, isFile := common.GetFileNameFromPath(fileParam.Path)

	klog.Infof("Cloud create, user: %s, param: %s, prefixPath: %s, name: %s, isFile: %v", owner, fileParam.Json(), dstPrefixPath, dstFileOrDirName, isFile)

	dstPrefixPath = strings.TrimPrefix(dstPrefixPath, "/")
	dstFileExt := filepath.Ext(dstFileOrDirName)

	var configName = fmt.Sprintf("%s_%s_%s", owner, fileParam.FileType, fileParam.Extend)
	var config, err = s.service.command.GetConfig().GetConfig(configName)
	if err != nil {
		return nil, err
	}
	var fs = s.service.getFs(configName, config.Type, config.Bucket, dstPrefixPath)
	var opts = &operations.OperationsOpt{
		Metadata:   false,
		NoModTime:  true,
		NoMimeType: true,
	}
	if isFile {
		opts.FilesOnly = true
	} else {
		opts.DirsOnly = true
	}
	lists, err := s.service.command.GetOperation().List(fs, opts)
	if err != nil {
		return nil, err
	}

	var dupNames []string
	if lists != nil && lists.List != nil && len(lists.List) > 0 {
		for _, item := range lists.List {
			var tmpExt = filepath.Ext(item.Name)
			if tmpExt != dstFileExt {
				continue
			}
			if isFile {
				if strings.Contains(strings.TrimSuffix(item.Name, tmpExt), strings.TrimSuffix(dstFileOrDirName, dstFileExt)) {
					dupNames = append(dupNames, strings.TrimSuffix(item.Name, tmpExt))
				}
			} else {
				if strings.Contains(item.Name, dstFileOrDirName) {
					dupNames = append(dupNames, item.Name)
				}
			}

		}
	}

	newName := common.GenerateDupCommonName(dupNames, strings.TrimSuffix(dstFileOrDirName, dstFileExt), dstFileOrDirName)
	if newName != "" {
		newName = newName + dstFileExt
	} else {
		newName = dstFileOrDirName
	}

	klog.Infof("Cloud create, dupNames: %d, newName: %s", len(dupNames), newName)

	if !isFile {
		var p = &models.PostParam{
			Drive:      fileParam.FileType,
			Name:       fileParam.Extend,
			ParentPath: dstPrefixPath,
			FolderName: newName,
		}

		klog.Infof("Cloud create, dir, service request param: %s", common.ToJson(p))

		res, err := s.service.CreateFolder(owner, p)
		if err != nil {
			klog.Errorf("Cloud create, dir error: %v, user: %s, path: %s", err, owner, fileParam.Path)
			return nil, err
		}
		klog.Infof("Cloud create, dir done! result: %s, user: %s, path: %s", string(res), owner, fileParam.Path)
		return nil, nil
	} else {
		if _, err := s.service.CopyFile(configName, dstPrefixPath, newName); err != nil {
			klog.Errorf("Cloud create, file error: %v, dstPrefixPath: %s, newName: %s", err, dstPrefixPath, newName)
			return nil, err
		}

		klog.Errorf("Cloud create, file done! dstPrefixPath: %s, newName: %s", dstPrefixPath, newName)
	}

	return nil, nil
}

func (s *CloudStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	var owner = fileDeleteArg.FileParam.Owner
	var fileParam = fileDeleteArg.FileParam
	var fileType = fileParam.FileType
	_ = fileType
	var user = fileParam.Owner
	var dirents = fileDeleteArg.Dirents
	var deleteFailedPaths []string

	klog.Infof("Cloud delete, user: %s, param: %s", user, common.ToJson(fileParam))

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

	for _, dp := range dirents {
		dp = strings.TrimSpace(dp) //  /path/ or /file

		klog.Infof("Cloud delete, user: %s, dirent: %s", user, dp)

		dpd, err := url.PathUnescape(dp)
		if err != nil {
			klog.Errorf("Cloud delete, path unescape error: %v, path: %s", err, dp)
			deleteFailedPaths = append(deleteFailedPaths, dp)
			continue
		}

		var data = &models.DeleteParam{
			Drive: fileParam.FileType,
			Name:  fileParam.Extend,
			Path:  dpd,
		}

		_, err = s.service.Delete(owner, fileDeleteArg.FileParam.Path, data)
		if err != nil {
			klog.Errorf("Cloud delete, delete files error: %v, user: %s", err, user)
			deleteFailedPaths = append(deleteFailedPaths, dp)
			continue
		}

		klog.Infof("Cloud delete, delete success, user: %s, file: %s", user, dpd)
	}

	if err := s.service.command.GetOperation().FsCacheClear(); err != nil {
		klog.Errorf("Cloud delete, fscache clear error: %v", err)
	}

	if len(deleteFailedPaths) > 0 {
		return common.ToBytes(deleteFailedPaths), fmt.Errorf("delete failed paths")
	}

	return nil, nil
}

func (s *CloudStorage) Rename(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var owner = contextArgs.FileParam.Owner
	var fileParam = contextArgs.FileParam
	klog.Infof("Cloud rename, user: %s, param: %s", owner, common.ToJson(contextArgs))

	var srcName, isSrcFile = getRenamedSrcName(fileParam.Path) // srcName have no /
	var dstName, _ = url.PathUnescape(contextArgs.QueryParam.Destination)
	var srcPrefixPath = getRenamedSrcPrefixPath(fileParam.Path)

	if srcName == dstName {
		klog.Infof("Cloud rename, name not changed, user: %s, srcName: %s, dstName: %s", owner, srcName, dstName)
		return nil, nil
	}

	var configName string = fmt.Sprintf("%s_%s_%s", owner, fileParam.FileType, fileParam.Extend)
	var srcFs, srcRemote string
	if isSrcFile {
		srcFs = srcPrefixPath
		srcRemote = srcName
	} else {
		srcRemote = fileParam.Path
	}

	srcStat, err := s.service.Stat(configName, srcFs, strings.TrimPrefix(srcRemote, "/"), isSrcFile)
	if err != nil {
		return nil, err
	}

	if srcStat == nil || srcStat.Item == nil {
		return nil, fmt.Errorf("path %s not exists", fileParam.Path)
	}

	var dstFs, dstRemote string
	dstRemote = srcPrefixPath + dstName
	if !isSrcFile {
		dstRemote = dstRemote + "/"
	}

	dstStat, err := s.service.Stat(configName, dstFs, strings.TrimPrefix(dstRemote, "/"), isSrcFile)
	if err != nil {
		klog.Errorf("Cloud rename, user: %s, stat error: %v", owner, err)
		return nil, err
	}

	klog.Infof("Cloud rename, user: %s, isSrcFile: %v, srcPrefixPath: %s, stat: %s", owner, isSrcFile, srcPrefixPath, common.ToJson(dstStat))

	if dstStat != nil && dstStat.Item != nil {
		return nil, fmt.Errorf("The name %s already exists. Please choose another name.", dstName)
	}

	resp, err := s.service.Rename(owner, contextArgs.FileParam, srcName, srcPrefixPath, dstName, isSrcFile)
	if err != nil {
		return nil, err
	}

	if err := s.service.command.GetOperation().FsCacheClear(); err != nil {
		klog.Errorf("Cloud rename, fscache clear error: %v", err)
	}

	klog.Infof("Cloud rename, user: %s, resp: %s", owner, string(resp))

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
		if fileParam.FileType == common.GoogleDrive {
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

func (s *CloudStorage) getFiles(fileParam *models.FileParam) (*models.CloudListResponse, error) {
	res, err := s.service.List(fileParam)
	if err != nil {
		return nil, fmt.Errorf("service list error: %v", err)
	}

	return res, nil
}
