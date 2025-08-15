package upload

import (
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func HandleUploadLink(fileParam *models.FileParam, from string) ([]byte, error) {
	_, _, _, uploadsDir, err := GetPVC(fileParam)
	if err != nil {
		return nil, errors.New("bfl header missing or invalid")
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}
	path := uri + fileParam.Path

	// change temp file location
	extracted := ExtractPart(path)
	if extracted != "" {
		uploadsDir = ExternalPathPrefix + extracted + "/.uploadstemp"
	}

	if !PathExists(uploadsDir) {
		if err = os.MkdirAll(uploadsDir, os.ModePerm); err != nil {
			klog.Warning("err:", err)
			return nil, err
		}
	}

	if !CheckDirExist(path) {
		if err = files.MkdirAllWithChown(nil, path, os.ModePerm); err != nil {
			klog.Error("err:", err)
			return nil, err
		}
	}

	uploadID := MakeUid(path)
	uploadLink := fmt.Sprintf("/upload/upload-link/%s/%s", common.NodeName, uploadID)

	return []byte(uploadLink), nil
}

func HandleUploadedBytes(fileParam *models.FileParam, fileName string) ([]byte, error) {
	_, _, _, uploadsDir, err := GetPVC(fileParam)
	if err != nil {
		return nil, errors.New("bfl header missing or invalid")
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}
	parentDir := uri + fileParam.Path

	if !CheckDirExist(parentDir) {
		klog.Warningf("Storage path %s is not exist or is not a dir", parentDir)
		return nil, errors.New("storage path is not exist or is not a dir")
	}

	responseData := make(map[string]interface{})
	responseData["uploadedBytes"] = 0

	extracted := ExtractPart(parentDir)
	if extracted != "" {
		uploadsDir = ExternalPathPrefix + extracted + "/.uploadstemp"
	}

	if !PathExists(uploadsDir) {
		return common.ToBytes(responseData), errors.New("uploads path is not exist or is not a dir")
	}

	fullPath := filepath.Join(parentDir, fileName)
	dirPath := filepath.Dir(fullPath)

	if !CheckDirExist(dirPath) {
		return common.ToBytes(responseData), errors.New("storage path is not exist or is not a dir")
	}

	if strings.HasSuffix(fileName, "/") {
		return nil, errors.New(fmt.Sprintf("full path %s is a dir", fullPath))
	}

	innerIdentifier := MakeUid(fullPath)
	tmpName := innerIdentifier
	UploadsFiles[innerIdentifier] = filepath.Join(uploadsDir, tmpName)

	exist, info := FileInfoManager.ExistFileInfo(innerIdentifier)

	if !exist {
		fileInfo := FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
		}

		if err = FileInfoManager.AddFileInfo(innerIdentifier, fileInfo); err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return nil, errors.New("Error save file info")
		}

		klog.Infof("innerIdentifier:%s, fileInfo:%+v", innerIdentifier, fileInfo)
		info = fileInfo
		exist = true
	}

	fileExist, fileLen := FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
	klog.Info("***exist: ", exist, " , fileExist: ", fileExist, " , fileLen: ", fileLen)

	if exist {
		if fileExist {
			if info.Offset != fileLen {
				info.Offset = fileLen
				FileInfoManager.UpdateInfo(innerIdentifier, info)
			}
			klog.Infof("innerIdentifier:%s, info.Offset:%d", innerIdentifier, info.Offset)
			responseData["uploadedBytes"] = info.Offset
		} else if info.Offset == 0 {
			klog.Warningf("innerIdentifier:%s, info.Offset:%d", innerIdentifier, info.Offset)
		} else {
			FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)
		}
	}

	return common.ToBytes(responseData), nil
}

func HandleUploadChunks(fileParam *models.FileParam, uploadId string, resumableInfo models.ResumableInfo, ranges string) ([]byte, error) {
	startTime := time.Now()
	klog.Infof("Function UploadChunks started at: %s\n", startTime)
	defer func() {
		endTime := time.Now()
		klog.Infof("Function UploadChunks ended at: %s\n", endTime)
	}()

	_, _, _, uploadsDir, err := GetPVC(fileParam)
	if err != nil {
		return nil, errors.New("bfl header missing or invalid")
	}

	responseData := map[string]interface{}{
		"success": true,
	}

	p := resumableInfo.ParentDir
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}
	parentDir := uri + fileParam.Path

	extracted := ExtractPart(parentDir)
	if extracted != "" {
		uploadsDir = ExternalPathPrefix + extracted + "/.uploadstemp"
	}
	if !PathExists(uploadsDir) {
		if err = os.MkdirAll(uploadsDir, os.ModePerm); err != nil {
			klog.Warningf("uploadId:%s, err:%v", uploadId, err)
			return nil, err
		}
	}

	fullPath := filepath.Join(parentDir, resumableInfo.ResumableRelativePath)
	innerIdentifier := MakeUid(fullPath)
	tmpName := innerIdentifier
	UploadsFiles[innerIdentifier] = filepath.Join(uploadsDir, tmpName)

	exist, info := FileInfoManager.ExistFileInfo(innerIdentifier)
	if !exist {
		klog.Warningf("innerIdentifier %s not exist", innerIdentifier)
	}

	if !exist || innerIdentifier != info.ID {
		RemoveTempFileAndInfoFile(tmpName, uploadsDir)
		if info.Offset != 0 {
			info.Offset = 0
			FileInfoManager.UpdateInfo(innerIdentifier, info)
		}

		if !CheckDirExist(parentDir) {
			klog.Warningf("Parent dir %s is not exist or is not a dir", resumableInfo.ParentDir)
			return nil, errors.New("parent dir is not exist or is not a dir")
		}

		dirPath := filepath.Dir(fullPath)
		if !CheckDirExist(dirPath) {
			if err = files.MkdirAllWithChown(nil, dirPath, os.ModePerm); err != nil {
				klog.Error("err:", err)
				return nil, err
			}
		}

		if resumableInfo.ResumableRelativePath == "" {
			return nil, errors.New("file_relative_path invalid")
		}

		if strings.HasSuffix(resumableInfo.ResumableRelativePath, "/") {
			klog.Warningf("full path %s is a dir", fullPath)
			return nil, errors.New(fmt.Sprintf("full path %s is a dir", fullPath))
		}

		if !CheckType(resumableInfo.ResumableType) {
			klog.Warningf("unsupported filetype:%s", resumableInfo.ResumableType)
			return nil, errors.New(fmt.Sprintf("unsupported filetype:%s", resumableInfo.ResumableType))
		}

		if !CheckSize(resumableInfo.ResumableTotalSize) {
			if info.Offset == info.FileSize {
				klog.Warningf("All file chunks have been uploaded, skip upload")
				finishData := []map[string]interface{}{
					{
						"name": resumableInfo.ResumableFilename,
						"id":   MakeUid(info.FullPath),
						"size": info.FileSize,
					},
				}
				return common.ToBytes(finishData), nil
			}
			klog.Warningf("Unsupported file size uploadSize:%d", resumableInfo.ResumableTotalSize)
			return nil, errors.New("unsupported file size")
		}

		info.FileSize = resumableInfo.ResumableTotalSize
		FileInfoManager.UpdateInfo(innerIdentifier, info)

		oExist, oInfo := FileInfoManager.ExistFileInfo(innerIdentifier)
		oFileExist, oFileLen := FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
		if oExist {
			if oFileExist {
				if oInfo.Offset != oFileLen {
					oInfo.Offset = oFileLen
					FileInfoManager.UpdateInfo(innerIdentifier, oInfo)
				}
			} else if oInfo.Offset == 0 {
			} else {
				FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)
			}
		}

		fileInfo := FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
			FileMetaData: FileMetaData{
				FileRelativePath: resumableInfo.ResumableRelativePath,
				FileType:         resumableInfo.ResumableType,
				FileSize:         resumableInfo.ResumableTotalSize,
				StoragePath:      parentDir,
				FullPath:         fullPath,
			},
		}

		if oFileExist {
			fileInfo.Offset = oFileLen
		}

		if err = FileInfoManager.AddFileInfo(innerIdentifier, fileInfo); err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return nil, errors.New("error save file info")
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
				StoragePath:      parentDir,
				FullPath:         fullPath,
			},
		}

		FileInfoManager.UpdateInfo(innerIdentifier, fileInfo)
		if err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return nil, errors.New("error save file info")
		}

		info = fileInfo
	}

	fileExist, fileLen := FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
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
			return nil, errors.New(fmt.Sprintf("Invalid content range:%s", ranges))
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
			finishData := []map[string]interface{}{
				{
					"name": resumableInfo.ResumableFilename,
					"id":   MakeUid(info.FullPath),
					"size": info.FileSize,
				},
			}
			return common.ToBytes(finishData), nil
		}
		return nil, errors.New("Unsupported file size")
	}

	const maxRetries = 100
	for retry := 0; retry < maxRetries; retry++ {
		if info.Offset-offsetEnd > 0 {
			return common.ToBytes(responseData), nil
		}

		if info.Offset == offsetStart || (info.Offset-offsetEnd < 0 && info.Offset-offsetStart > 0) {
			if resumableInfo.MD5 != "" {
				md5, err := common.MD5FileHeader(fileHeader)
				if err != nil {
					return nil, err
				}
				if md5 != resumableInfo.MD5 {
					msg := fmt.Sprintf("Invalid MD5, accepted %s, calculated %s", resumableInfo.MD5, md5)
					return nil, errors.New(msg)
				}
			}

			fileSize, err := SaveFile(fileHeader, filepath.Join(uploadsDir, tmpName), newFile, offsetStart)
			if err != nil {
				klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
				return nil, err
			}
			info.Offset = fileSize
			FileInfoManager.UpdateInfo(innerIdentifier, info)
			break
		}

		time.Sleep(1000 * time.Millisecond)

		if retry < maxRetries-1 {
			exist, info = FileInfoManager.ExistFileInfo(innerIdentifier)
			if !exist {
				return nil, errors.New("invalid innerIdentifier")
			}
			continue
		}

		return nil, errors.New("Failed to match offset after multiple retries")
	}

	if err = UpdateFileInfo(info, uploadsDir); err != nil {
		klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
		return nil, err
	}

	klog.Info("offsetStart:", offsetStart, ", offsetEnd:", offsetEnd, ", info.Offset:", info.Offset, ", info.FileSize:", info.FileSize)
	if offsetEnd == info.FileSize-1 {
		if err = MoveFileByInfo(info, uploadsDir); err != nil {
			klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
			return nil, err
		}
		FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)

		finishData := []map[string]interface{}{
			{
				"name": resumableInfo.ResumableFilename,
				"id":   MakeUid(info.FullPath),
				"size": info.FileSize,
			},
		}
		return common.ToBytes(finishData), nil
	}
	return common.ToBytes(responseData), nil
}
