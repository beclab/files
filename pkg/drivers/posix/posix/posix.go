package posix

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/diskcache"
	"files/pkg/drivers/base"
	"files/pkg/drivers/posix/upload"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/preview"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

var (
	Expand    = true
	Content   = true
	NoExpand  = false
	NoContent = false
)

type PosixStorage struct {
	handler *base.HandlerParam
	paste   *models.PasteParam
}

func NewPosixStorage(handler *base.HandlerParam) *PosixStorage {
	return &PosixStorage{
		handler: handler,
	}
}

func (s *PosixStorage) List(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	klog.Infof("Posix list, user: %s, args: %s", owner, fileParam.Json())

	fileData, err := s.getFiles(fileParam, Expand, Content)
	if err != nil {
		return nil, err
	}

	if fileData.IsDir {
		fileData.Listing.Sorting = files.DefaultSorting
		fileData.Listing.ApplySort()
	}

	res, err := json.Marshal(fileData)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *PosixStorage) Preview(contextArgs *models.HttpContextArgs) (*models.PreviewHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var queryParam = contextArgs.QueryParam
	var owner = fileParam.Owner

	klog.Infof("Posix preview, user: %s, args: %s", owner, common.ToJson(contextArgs))

	fileData, err := s.getFiles(fileParam, Expand, Content)
	if err != nil {
		return nil, err
	}

	var previewFileName = fileParam.FileType + fileParam.Extend + fileData.Path + fileData.ModTime.String() + queryParam.PreviewSize
	var key = diskcache.GenerateCacheKey(previewFileName)

	klog.Infof("Posix preview, user: %s, fileType: %s, ext: %s, name: %s", owner, fileData.Type, fileData.Extension, fileData.Name)

	// get cache
	cachedData, ok, err := preview.GetPreviewCache(owner, key, diskcache.CacheThumb)
	if err != nil {
		klog.Errorf("Posix preview, get cache failed, user: %s, error: %v", owner, err)

	} else if ok {

		klog.Infof("Posix preview, get cache, file: %s, cache name: %s, exists: %v", fileData.Path, previewFileName, ok)

		if cachedData != nil {
			return &models.PreviewHandlerResponse{
				FileName:     fileData.Name,
				FileModified: fileData.ModTime,
				Data:         cachedData,
			}, nil
		}
	}

	switch fileData.Type {
	case "image":
		data, err := preview.CreatePreview(owner, key, fileData, queryParam)
		if err != nil {
			return nil, err
		}
		return &models.PreviewHandlerResponse{
			FileName:     fileData.Name,
			FileModified: fileData.ModTime,
			Data:         data,
		}, nil
	default:
		return nil, fmt.Errorf("can't create preview for %s type", fileData.Type)
	}
}

func (s *PosixStorage) Raw(contextArgs *models.HttpContextArgs) (*models.RawHandlerResponse, error) {

	var fileParam = contextArgs.FileParam
	var user = fileParam.Owner

	klog.Infof("Posix raw, user: %s, args: %s", user, common.ToJson(contextArgs))

	fileData, err := s.getFiles(fileParam, NoExpand, NoContent)
	if err != nil {
		return nil, err
	}

	if fileData.IsDir {
		return nil, fmt.Errorf("not supported currently")
	}

	klog.Infof("Posix raw, file: %s", common.ToJson(fileData))

	r, err := getRawFile(fileData)
	if err != nil {
		return nil, err
	}

	return &models.RawHandlerResponse{
		FileName:     fileData.Name,
		FileModified: fileData.ModTime,
		Reader:       r,
	}, nil
}

func (s *PosixStorage) Tree(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	var owner = fileParam.Owner

	klog.Infof("Posix tree, user: %s, args: %s", owner, fileParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return err
	}

	var fs = afero.NewBasePathFs(afero.NewOsFs(), resourceUri)

	fileData, err := s.getFiles(fileParam, Expand, NoContent)
	if err != nil {
		return err
	}

	go s.generateListingData(fs, fileParam, fileData.Listing, stopChan, dataChan)

	return nil
}

func (s *PosixStorage) Create(contextArgs *models.HttpContextArgs) ([]byte, error) {
	var user = contextArgs.FileParam.Owner
	var fileParam = contextArgs.FileParam

	klog.Infof("Posix create, user: %s, param: %s", user, common.ToJson(contextArgs))

	dstFileOrDirName, isFile := files.GetFileNameFromPath(fileParam.Path)
	dstPrefixPath := files.GetPrefixPath(fileParam.Path)
	dstFileExt := filepath.Ext(dstFileOrDirName)

	resourceUri, err := contextArgs.FileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	dirName := filepath.Join(resourceUri, contextArgs.FileParam.Path)

	mode, err := strconv.ParseUint(contextArgs.QueryParam.FileMode, 8, 32)
	if err != nil {
		mode = 0755
	}

	fileMode := os.FileMode(mode)

	var afs = afero.NewOsFs()
	entries, err := afero.ReadDir(afs, filepath.Join(resourceUri, dstPrefixPath))
	if err != nil {
		return nil, err
	}

	var dupNames []string
	for _, entry := range entries {
		infoName := entry.Name()
		if isFile {
			infoExt := filepath.Ext(infoName)
			if infoExt != dstFileExt {
				continue
			}
			dupNames = append(dupNames, strings.TrimSuffix(infoName, dstFileExt))
		} else {
			if strings.Contains(infoName, dstFileOrDirName) {
				dupNames = append(dupNames, infoName)
			}
		}
	}

	klog.Infof("Posix create, dupNames: %d", len(dupNames))

	newName := files.GenerateDupName(dupNames, dstFileOrDirName, isFile)
	if newName != "" {
		if isFile {
			newName = newName + dstFileExt
		}

	} else {
		newName = dstFileOrDirName
	}

	dirName = filepath.Join(resourceUri, dstPrefixPath, newName)
	klog.Infof("Posix create, file path: %s, filename: %s", dirName, dstFileOrDirName)
	if isFile {
		file, err := os.OpenFile(dirName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
		if err != nil {
			klog.Errorf("Posix create, file error: %v, dirName: %s", err, dirName)
			return nil, err
		}
		defer file.Close()
	} else {
		if !strings.HasSuffix(dirName, "/") {
			dirName = dirName + "/"
		}

		if err := afs.MkdirAll(dirName, fileMode); err != nil {
			klog.Errorf("Posix create, dir error: %v, dir: %s", err, dirName)
			return nil, err
		}

	}

	return nil, nil
}

func (s *PosixStorage) Delete(fileDeleteArg *models.FileDeleteArgs) ([]byte, error) {
	var fileParam = fileDeleteArg.FileParam
	var dirents = fileDeleteArg.Dirents
	var user = fileParam.Owner

	var err error
	var deleteFailedPaths []string

	if dirents == nil || len(dirents) == 0 {
		return nil, fmt.Errorf("dirents is empty")
	}

	fileData, err := s.getFiles(fileParam, NoExpand, NoContent)
	if err != nil {
		klog.Errorf("Posix delete, get file data error: %s, user: %s, path: %s", err, user, fileParam.Path)
		return nil, fmt.Errorf("%s: no such file or directory", fileParam.Path)
	}

	var invalidPaths []string

	var errmsg = make(map[string]string)

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

	for _, dirent := range dirents {
		dirent = strings.TrimSpace(dirent)
		direntPath := fileData.Path + strings.TrimLeft(dirent, "/")
		klog.Infof("Posix delete, remove dirent path: %s", direntPath)
		if err = fileData.Fs.RemoveAll(direntPath); err != nil {
			klog.Errorf("Posix delete, remove path error: %v, user: %s, path: %s", err, user, direntPath)
			e := extractErrMsg(err)
			_, ok := errmsg[e]
			if !ok {
				errmsg[e] = e
				deleteFailedPaths = append(deleteFailedPaths, e)
			}
		}
	}

	if len(deleteFailedPaths) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(deleteFailedPaths, ";"))
	}

	return nil, nil
}

func (s *PosixStorage) Rename(contextArgs *models.HttpContextArgs) ([]byte, error) {

	var fileParam = contextArgs.FileParam
	var owner = fileParam.Owner

	if fileParam.Path == "/" {
		return nil, fmt.Errorf("path invalid, path: %s", fileParam.Path)
	}

	var uri, err = fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	klog.Infof("Posix rename, user: %s, uri: %s, args: %s", owner, uri, common.ToJson(contextArgs))

	var srcName, isSrcFile = files.GetFileNameFromPath(fileParam.Path)
	srcName, err = url.PathUnescape(srcName)
	if err != nil {
		return nil, err
	}
	dstName, err := url.PathUnescape(contextArgs.QueryParam.Destination) // no /
	if err != nil {
		return nil, err
	}

	klog.Infof("Posix rename, user: %s, uri: %s, isFile: %v, src: %s, dst: %s, args: %s", owner, uri, isSrcFile, srcName, dstName, common.ToJson(contextArgs))

	if srcName == dstName {
		return nil, nil
	}

	var srcFilenamePrefix = srcName
	var dstFilenamePrefix = dstName
	var srcFilenameExt, dstFilenameExt string

	if isSrcFile {
		srcFilenameExt = filepath.Ext(srcName)
		srcFilenamePrefix = strings.TrimSuffix(srcFilenamePrefix, srcFilenameExt)

		dstFilenameExt = filepath.Ext(dstName)
		dstFilenamePrefix = strings.TrimSuffix(dstFilenamePrefix, dstFilenameExt)
	}

	var afs = afero.NewOsFs()
	var srcPrefixPath = files.GetPrefixPath(fileParam.Path)
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:       afero.NewBasePathFs(afs, uri),
		FsType:   fileParam.FileType,
		FsExtend: fileParam.Extend,
		Path:     srcPrefixPath,
		Expand:   Expand,
		Content:  NoContent,
	})
	if err != nil {
		return nil, err
	}

	if file == nil || file.Items == nil || len(file.Items) == 0 {
		return nil, fmt.Errorf("file %s not exists", fileParam.Path)
	}

	var existsName bool
	for _, item := range file.Items {
		if item.Name == dstName {
			existsName = true
			break
		}
	}

	if existsName {
		return nil, fmt.Errorf("The name '%s' already exists. Please choose another name.", dstName)
	}

	var rSrcPath = uri + fileParam.Path
	var rDstPath = uri + srcPrefixPath + dstName

	klog.Infof("Posix rename, src: %s, dst: %s", rSrcPath, rDstPath)

	if err = afs.Rename(rSrcPath, rDstPath); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *PosixStorage) Edit(contextArgs *models.HttpContextArgs) (*models.EditHandlerResponse, error) {
	var fileParam = contextArgs.FileParam
	var user = fileParam.Owner
	klog.Infof("Posix edit, user: %s, path: %s", user, fileParam.Path)

	fileName, isFile := files.GetFileNameFromPath(fileParam.Path)
	if !isFile {
		return nil, fmt.Errorf("path %s is not file", fileParam.Path)
	}

	var fileExt = filepath.Ext(fileName)
	if fileExt != ".txt" {
		return nil, fmt.Errorf("file %s not supported", fileName)
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	filePath := uri + fileParam.Path

	exists, err := afero.Exists(files.DefaultFs, filePath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("file %s not exists", fileParam.Path)
	}

	info, err := files.WriteFile(files.DefaultFs, filePath, contextArgs.QueryParam.Body)
	etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())

	return &models.EditHandlerResponse{
		Etag: etag,
	}, nil
}

func (s *PosixStorage) generateListingData(fs afero.Fs, fileParam *models.FileParam, listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string) {
	defer close(dataChan)

	var streamFiles []*files.FileInfo
	streamFiles = append(streamFiles, listing.Items...)

	for len(streamFiles) > 0 {
		firstItem := streamFiles[0]

		if firstItem.IsDir {
			var nestFileParam = &models.FileParam{
				Owner:    fileParam.Owner,
				FileType: fileParam.FileType,
				Extend:   fileParam.Extend,
				Path:     firstItem.Path,
			}

			nestFileData, err := s.getFiles(nestFileParam, Expand, NoContent)
			if err != nil {
				klog.Error(err)
				return
			}

			var nestedItems []*files.FileInfo
			if nestFileData.Listing != nil {
				nestedItems = append(nestedItems, nestFileData.Listing.Items...)
			}
			streamFiles = append(nestedItems, streamFiles[1:]...)
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

func (s *PosixStorage) isExternal(fileType string, extend string) bool {
	return (fileType == common.External || fileType == common.Usb || fileType == common.Hdd || fileType == common.Internal || fileType == common.Smb) && extend != ""
}

func (s *PosixStorage) getFiles(fileParam *models.FileParam, expand, content bool) (*files.FileInfo, error) {
	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:       afero.NewBasePathFs(afero.NewOsFs(), resourceUri),
		FsType:   fileParam.FileType,
		FsExtend: fileParam.Extend,
		Path:     fileParam.Path,
		Expand:   expand,
		Content:  content,
	})
	if err != nil {
		return nil, err
	}

	if file == nil {
		return nil, fmt.Errorf("file %s not exists", fileParam.Path)
	}

	if s.isExternal(fileParam.FileType, fileParam.Extend) {
		klog.Infof("getFiles fileType: %s, extend: %s", fileParam.FileType, fileParam.Extend)
		file.ExternalType = global.GlobalMounted.CheckExternalType(file.Path, file.IsDir)
		if file.IsDir && file.Listing != nil {
			for _, f := range file.Items {
				f.ExternalType = global.GlobalMounted.CheckExternalType(f.Path, f.IsDir)
			}
		}
	}

	return file, nil

}

/**
 * UploadLink
 */
func (s *PosixStorage) UploadLink(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	var user = fileUploadArg.FileParam.Owner

	klog.Infof("Posix uploadLink, user: %s, param: %s", user, common.ToJson(fileUploadArg.FileParam))

	data, err := upload.HandleUploadLink(fileUploadArg.FileParam, fileUploadArg.From)

	klog.Infof("Posix uploadLink, done! data: %s", string(data))

	return data, err
}

/**
 * UploadedBytes
 */
func (s *PosixStorage) UploadedBytes(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	var user = fileUploadArg.FileParam.Owner
	klog.Infof("Posix uploadBytes, user: %s, param: %s", user, common.ToJson(fileUploadArg.FileParam))

	data, err := upload.HandleUploadedBytes(fileUploadArg.FileParam, fileUploadArg.FileName)

	klog.Infof("Posix uploadBytes, done! data: %s", string(data))

	return data, err
}

/**
 * UploadChunks
 */
func (s *PosixStorage) UploadChunks(fileUploadArg *models.FileUploadArgs) ([]byte, error) {
	var user = fileUploadArg.FileParam.Owner

	klog.Infof("Posix uploadChunks, user: %s, uploadId: %s, param: %s", user, fileUploadArg.UploadId, common.ToJson(fileUploadArg.FileParam))

	_, fileInfo, err := upload.HandleUploadChunks(fileUploadArg.FileParam, fileUploadArg.UploadId, *fileUploadArg.ChunkInfo, fileUploadArg.Ranges)

	if err != nil {
		return nil, err
	}

	if fileInfo == nil {
		return common.ToBytes(&upload.FileUploadSucced{Success: true, State: common.Running}), nil
	} else {
		fileInfo.State = common.Completed
	}

	klog.Infof("Posix uploadChunks, done! data: %s", common.ToJson(fileInfo))

	return common.ToBytes(fileInfo), err
}
