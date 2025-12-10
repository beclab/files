package upload

import (
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

/**
 * ~ HandleUploadLink
 */
func HandleUploadLink(fileParam *models.FileParam, from string) ([]byte, error) {
	var user = fileParam.Owner
	var uploadPath string
	var uploadTempPath string

	klog.Infof("[upload] uploadLink, user: %s, from: %s, param: %s", user, from, common.ToJson(fileParam))

	var cachePvcPath = global.GlobalData.GetPvcCache(user)
	if cachePvcPath == "" {
		return nil, fmt.Errorf("cache dir not found")
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	uploadPath = uri + fileParam.Path
	uploadTempPath = filepath.Join(common.CACHE_PREFIX, cachePvcPath, common.DefaultUploadToCloudTempPath)

	// if fileParam.FileType == common.Drive || fileParam.FileType == common.Cache || fileParam.FileType == common.External {
	// 	if !CheckDirExist(uploadPath) {
	// 		if err = files.MkdirAllWithChown(nil, uploadPath, os.ModePerm); err != nil {
	// 			klog.Error("err:", err)
	// 			return nil, err
	// 		}
	// 	}
	// }

	uploadID := MakeUid(uploadPath + from)
	uploadLink := fmt.Sprintf("/upload/upload-link/%s/%s", common.NodeName, uploadID)

	klog.Infof("[upload] uploadLink, uploadPath: %s, uploadTempPath: %s, uploadLink: %s", uploadPath, uploadTempPath, uploadLink)

	return []byte(uploadLink), nil
}

/**
 * ~ HandleUploadedBytes
 */
func HandleUploadedBytes(fileParam *models.FileParam, fileName string, fileIdenty string, reqUA string) ([]byte, error) {
	var user = fileParam.Owner
	klog.Infof("[upload] uploadedBytes, user: %s, fileName: %s, param: %s", user, fileName, common.ToJson(fileParam))

	var uploadPath string
	var uploadTempPath string

	var cachePvcPath = global.GlobalData.GetPvcCache(user)
	if cachePvcPath == "" {
		return nil, fmt.Errorf("cache dir not found")
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	uploadPath = uri + fileParam.Path
	uploadTempPath = filepath.Join(common.CACHE_PREFIX, cachePvcPath, common.DefaultUploadToCloudTempPath)

	// if fileParam.FileType == common.Drive || fileParam.FileType == common.Cache || fileParam.FileType == common.External {
	// 	if !common.PathExists(uploadPath) {
	// 		if err = files.MkdirAllWithChown(nil, uploadPath, os.ModePerm); err != nil {
	// 			klog.Error("err:", err)
	// 			return nil, err
	// 		}
	// 	}
	// }

	responseData := make(map[string]interface{})
	responseData["uploadedBytes"] = 0

	fullPath := filepath.Join(uploadPath, fileName)
	dirPath := filepath.Dir(fullPath)

	klog.Infof("[upload] uploadedBytes, fullPath: %s, dirPath: %s", fullPath, dirPath)

	// if fileParam.FileType == common.Drive || fileParam.FileType == common.Cache || fileParam.FileType == common.External {
	// 	if !common.PathExists(dirPath) {
	// 		return common.ToBytes(responseData), errors.New("storage path is not exist or is not a dir")
	// 	}
	// }

	if strings.HasSuffix(fileName, "/") {
		return nil, fmt.Errorf("full path %s is a dir", fullPath)
	}

	// todo add identy
	innerIdentifier := MakeUid(fullPath) // real upload file temp name
	tmpName := innerIdentifier
	UploadsFiles[innerIdentifier] = filepath.Join(uploadTempPath, tmpName)

	exist, info := FileInfoManager.ExistFileInfo(innerIdentifier)

	if !exist {
		fileInfo := FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
		}

		if err = FileInfoManager.AddFileInfo(innerIdentifier, fileInfo); err != nil {
			klog.Warningf("[upload] uploadedBytes, innerIdentifier:%s, err:%v", innerIdentifier, err)
			return nil, errors.New("Error save file info")
		}

		klog.Infof("[upload] uploadedBytes, innerIdentifier:%s, fileInfo:%+v", innerIdentifier, fileInfo)
		info = fileInfo
		exist = true
	}

	fileExist, fileLen := FileInfoManager.CheckTempFile(innerIdentifier, uploadTempPath)
	klog.Info("***exist: ", exist, " , fileExist: ", fileExist, " , fileLen: ", fileLen)

	if exist {
		if fileExist {
			if info.Offset != fileLen {
				info.Offset = fileLen
				FileInfoManager.UpdateInfo(innerIdentifier, info)
			}
			klog.Infof("[upload] uploadedBytes, innerIdentifier:%s, info.Offset:%d", innerIdentifier, info.Offset)
			responseData["uploadedBytes"] = info.Offset
		} else if info.Offset == 0 {
			klog.Warningf("innerIdentifier:%s, info.Offset:%d", innerIdentifier, info.Offset)
		} else {
			FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadTempPath) // handlerUploadedBytes
		}
	}

	return common.ToBytes(responseData), nil
}

/**
 * ~ HandleUploadChunks
 */
func HandleUploadChunks(fileParam *models.FileParam, uploadId string, resumableInfo models.ResumableInfo, reqUA string, ranges string) (bool, *FileUploadState, error) {
	var user = fileParam.Owner
	var uploadPath string
	var uploadTempPath string
	var share = resumableInfo.Share
	var shareby = resumableInfo.Shareby
	// _, _ = share, shareby

	klog.Infof("[upload] uploadedChunks, user: %s, uploadId: %s, param: %s", user, uploadId, common.ToJson(fileParam))

	var cachePvcPath = global.GlobalData.GetPvcCache(user)
	if cachePvcPath == "" {
		return false, nil, fmt.Errorf("pvc cache not found")
	}

	startTime := time.Now()
	klog.Infof("[upload] uploadId: %s, started at: %s\n", uploadId, startTime)
	defer func() {
		endTime := time.Now()
		klog.Infof("[upload] uploadId: %s, ended at: %s\n", uploadId, endTime)
	}()

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return false, nil, err
	}

	if fileParam.FileType == common.External {
		uploadTempPath = filepath.Join(uri, fileParam.Path, common.DefaultUploadTempDir)
	} else {
		uploadTempPath = filepath.Join(common.CACHE_PREFIX, cachePvcPath, common.DefaultUploadToCloudTempPath)
	}

	if !common.PathExists(uploadTempPath) {
		if err = os.MkdirAll(uploadTempPath, os.ModePerm); err != nil {
			klog.Errorf("[upload] uploadId: %s, mkdir uploadTempPath %s err: %v", uploadId, uploadTempPath, err)
			return false, nil, err
		}
	}

	p := resumableInfo.ParentDir
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	// dst storage path, if shared, fileParam must be sharedby Param
	// uploadPath = uri + fileParam.Path
	if share != "" {
		shareParam, err := models.CreateFileParam(shareby, resumableInfo.ParentDir) // sharebyPath
		if err != nil {
			return false, nil, err
		}
		sharebyUri, err := shareParam.GetResourceUri()
		if err != nil {
			return false, nil, err
		}
		uploadPath = sharebyUri + shareParam.Path
	} else {
		uploadPath = uri + fileParam.Path
	}
	fullPath := filepath.Join(uploadPath, resumableInfo.ResumableRelativePath)
	innerIdentifier := MakeUid(fullPath) // todo add resumableInfo.ResumableIdenty
	tmpName := innerIdentifier
	UploadsFiles[innerIdentifier] = filepath.Join(uploadTempPath, tmpName)

	klog.Infof("[upload] uploadId: %s, fullPath: %s", uploadId, fullPath)

	exist, info := FileInfoManager.ExistFileInfo(innerIdentifier)
	if !exist {
		klog.Warningf("[upload] uploadId: %s, innerIdentifier %s not exist", uploadId, innerIdentifier)
	}

	if !exist || innerIdentifier != info.ID {
		RemoveTempFileAndInfoFile(tmpName, uploadTempPath)

		dirPath := filepath.Dir(fullPath)
		_ = dirPath

		if info.Offset != 0 {
			info.Offset = 0
			FileInfoManager.UpdateInfo(innerIdentifier, info)
		}

		if fileParam.FileType == common.Drive || fileParam.FileType == common.Cache || fileParam.FileType == common.External {
			if !CheckDirExist(uploadPath) {
				klog.Warningf("[upload] uploadId: %s, Parent dir %s is not exist or is not a dir", uploadId, resumableInfo.ParentDir)
				return false, nil, errors.New("parent dir is not exist or is not a dir")
			}

			// if !CheckDirExist(dirPath) { // + todo comment
			// 	if err = files.MkdirAllWithChown(nil, dirPath, os.ModePerm); err != nil {
			// 		klog.Error("err:", err)
			// 		return false, nil, err
			// 	}
			// }
		}

		if resumableInfo.ResumableRelativePath == "" {
			return false, nil, errors.New("file_relative_path invalid")
		}

		if strings.HasSuffix(resumableInfo.ResumableRelativePath, "/") {
			klog.Warningf("[upload] uploadId: %s, full path %s is a dir", uploadId, fullPath)
			return false, nil, fmt.Errorf("full path %s is a dir", fullPath)
		}

		if !CheckType(resumableInfo.ResumableType) {
			klog.Warningf("[upload] uploadId: %s, unsupported filetype:%s", uploadId, resumableInfo.ResumableType)
			return false, nil, fmt.Errorf("unsupported filetype:%s", resumableInfo.ResumableType)
		}

		if !CheckSize(resumableInfo.ResumableTotalSize) {
			if info.Offset == info.FileSize {
				klog.Warningf("[upload] uploadId: %s, All file chunks have been uploaded, skip upload", uploadId)

				var data = &FileUploadState{
					Name:           resumableInfo.ResumableFilename,
					Id:             MakeUid(info.FullPath), // todo
					UploadTempPath: uploadTempPath,
					FileInfo:       &info,
				}
				return false, data, nil
			}
			klog.Warningf("[upload] uploadId: %s, Unsupported file size uploadSize:%d", uploadId, resumableInfo.ResumableTotalSize)
			return false, nil, errors.New("unsupported file size")
		}

		info.FileSize = resumableInfo.ResumableTotalSize
		FileInfoManager.UpdateInfo(innerIdentifier, info)

		oExist, oInfo := FileInfoManager.ExistFileInfo(innerIdentifier)
		oFileExist, oFileLen := FileInfoManager.CheckTempFile(innerIdentifier, uploadTempPath)
		if oExist {
			if oFileExist {
				if oInfo.Offset != oFileLen {
					oInfo.Offset = oFileLen
					FileInfoManager.UpdateInfo(innerIdentifier, oInfo)
				}
			} else if oInfo.Offset == 0 {
			} else {
				FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadTempPath)
			}
		}

		fileInfo := FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
			FileMetaData: FileMetaData{
				FileRelativePath: resumableInfo.ResumableRelativePath,
				FileType:         resumableInfo.ResumableType,
				FileSize:         resumableInfo.ResumableTotalSize,
				StoragePath:      uploadPath,
				FullPath:         fullPath,
			},
		}

		if oFileExist {
			fileInfo.Offset = oFileLen
		}

		if err = FileInfoManager.AddFileInfo(innerIdentifier, fileInfo); err != nil {
			klog.Warningf("[upload] uploadId: %s, innerIdentifier:%s, err:%v", uploadId, innerIdentifier, err)
			return false, nil, errors.New("error save file info")
		}

		info = fileInfo
	} else {
		fileInfo := FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
			FileMetaData: FileMetaData{
				FileRelativePath: resumableInfo.ResumableRelativePath,
				FileType:         resumableInfo.ResumableType,
				FileSize:         resumableInfo.ResumableTotalSize,
				StoragePath:      uploadPath,
				FullPath:         fullPath,
			},
		}

		FileInfoManager.UpdateInfo(innerIdentifier, fileInfo)
		if err != nil {
			klog.Warningf("[upload] uploadId: %s, innerIdentifier:%s, err:%v", uploadId, innerIdentifier, err)
			return false, nil, errors.New("error save file info")
		}

		info = fileInfo
	}

	fileExist, fileLen := FileInfoManager.CheckTempFile(innerIdentifier, uploadTempPath)
	if fileExist {
		if info.Offset != fileLen {
			info.Offset = fileLen
			FileInfoManager.UpdateInfo(innerIdentifier, info)
		}
	}

	fileHeader := resumableInfo.File
	size := fileHeader.Size

	var offsetStart, offsetEnd int64
	var parsed bool
	if ranges != "" {
		offsetStart, offsetEnd, parsed = ParseContentRange(ranges)
		if !parsed {
			return false, nil, fmt.Errorf("Invalid content range:%s", ranges)
		}
	}

	var newFile bool = false
	if info.Offset != offsetStart && offsetStart == 0 {
		newFile = true
		info.Offset = offsetStart
		FileInfoManager.UpdateInfo(innerIdentifier, info)
	}

	if !CheckSize(size) || offsetEnd >= info.FileSize {
		if info.Offset == info.FileSize {
			var data = &FileUploadState{
				Name:     resumableInfo.ResumableFilename,
				Id:       MakeUid(info.FullPath), // todo
				Size:     info.FileSize,
				FileInfo: &info,
			}
			return true, data, nil
		}
		return false, nil, errors.New("Unsupported file size")
	}

	if info.Offset-offsetEnd > 0 {
		return true, nil, nil
	}

	if info.Offset == offsetStart || (info.Offset-offsetEnd < 0 && info.Offset-offsetStart > 0) {
		klog.Infof("resumableInfo.MD5=%s", resumableInfo.MD5)
		fileSize, chunkMd5, err := SaveFile(fileHeader, filepath.Join(uploadTempPath, tmpName), newFile, offsetStart, resumableInfo.MD5 != "")
		if err != nil {
			klog.Warningf("[upload] uploadId: %s, innerIdentifier:%s, info:%+v, err:%v", uploadId, innerIdentifier, info, err)
			return false, nil, err
		}
		if resumableInfo.MD5 != "" && chunkMd5 != resumableInfo.MD5 {
			msg := fmt.Sprintf("Invalid MD5, accepted %s, calculated %s", resumableInfo.MD5, chunkMd5)
			return false, nil, errors.New(msg)
		}
		info.Offset = fileSize
		FileInfoManager.UpdateInfo(innerIdentifier, info)
	}

	if err = UpdateFileInfo(info, uploadTempPath); err != nil {
		klog.Warningf("[upload] uploadId: %s, innerIdentifier:%s, info:%+v, err:%v", uploadId, innerIdentifier, info, err)
		return false, nil, err
	}

	klog.Infof("[upload] uploadId: %s, offsetStart: %d, offsetEnd: %d, info.Offset: %d, info.FileSize:: %d", uploadId, offsetStart, offsetEnd, info.Offset, info.FileSize)

	if offsetEnd == info.FileSize-1 {
		var data = &FileUploadState{
			Name:           resumableInfo.ResumableFilename,
			Id:             MakeUid(info.FullPath), // todo
			Size:           info.FileSize,
			State:          common.Completed,
			UploadTempPath: uploadTempPath,
			FileInfo:       &info,
		}

		if fileParam.IsCloud() {
			return true, data, nil
		}

		if resumableInfo.Share != "" { // upload to shared directory
			sharedParam, err := models.CreateFileParam(resumableInfo.Shareby, resumableInfo.ParentDir)
			if err != nil {
				return false, data, err
			}

			uri, err := sharedParam.GetResourceUri()
			if err != nil {
				return false, data, err
			}

			info.FullPath = uri + sharedParam.Path + resumableInfo.ResumableRelativePath
		}

		klog.Infof("[upload] uploadId: %s, move by, src: %s, dst: %s", uploadId, uploadTempPath, info.FullPath)

		if err = MoveFileByInfo(info, uploadTempPath); err != nil {
			klog.Warningf("[upload] uploadId: %s, move by, innerIdentifier:%s, info:%+v, err:%v", uploadId, innerIdentifier, info, err)
			return false, nil, err
		}

		FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadTempPath) // handlerfunc

		return true, data, nil
	}
	return true, nil, nil
}
