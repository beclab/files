package http

import (
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/files"
	"files/pkg/models"
	"files/pkg/upload"
	"files/pkg/utils"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

func uploadLinkHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	owner, _, _, uploadsDir, err := upload.GetPVC(r)
	if err != nil {
		return http.StatusBadRequest, e.New("bfl header missing or invalid")
	}

	p := r.URL.Query().Get("file_path")
	if p == "" {
		return http.StatusBadRequest, e.New("missing path query parameter")
	}

	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	fileParam, err := models.CreateFileParam(owner, p)
	if err != nil {
		return http.StatusBadRequest, err
	}
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	path := uri + fileParam.Path
	klog.Infof("~~~Debug log: path=%s", path)

	if fileParam.FileType == "sync" {
		return seahub.HandleUploadLink(w, r, d)
	}

	// change temp file location
	extracted := upload.ExtractPart(path)
	if extracted != "" {
		uploadsDir = upload.ExternalPathPrefix + extracted + "/.uploadstemp"
	}

	if !upload.PathExists(uploadsDir) {
		if err := os.MkdirAll(uploadsDir, os.ModePerm); err != nil {
			klog.Warning("err:", err)
			return http.StatusInternalServerError, err
		}
	}

	if !upload.CheckDirExist(path) {
		if err := files.MkdirAllWithChown(nil, path, os.ModePerm); err != nil {
			klog.Error("err:", err)
			return http.StatusInternalServerError, err
		}
	}

	uploadID := upload.MakeUid(path)
	uploadLink := fmt.Sprintf("/upload/upload-link/%s/%s", utils.NodeName, uploadID)

	w.Write([]byte(uploadLink))
	return 0, nil
}

func uploadedBytesHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	owner, _, _, uploadsDir, err := upload.GetPVC(r)
	if err != nil {
		return http.StatusBadRequest, e.New("bfl header missing or invalid")
	}

	p := r.URL.Query().Get("parent_dir")
	if p == "" {
		return http.StatusBadRequest, e.New("missing parent_dir query parameter")
	}

	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	fileParam, err := models.CreateFileParam(owner, p)
	if err != nil {
		return http.StatusBadRequest, err
	}
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	parentDir := uri + fileParam.Path
	klog.Infof("~~~Debug log: parentDir=%s", parentDir)

	if fileParam.FileType == "sync" {
		return seahub.HandleUploadedBytes(w, r, d)
	}

	if !upload.CheckDirExist(parentDir) {
		klog.Warningf("Storage path %s is not exist or is not a dir", parentDir)
		return http.StatusBadRequest, e.New("storage path is not exist or is not a dir")
	}

	fileName := r.URL.Query().Get("file_name")
	if fileName == "" {
		return http.StatusBadRequest, e.New("file_relative_path invalid")
	}

	responseData := make(map[string]interface{})
	responseData["uploadedBytes"] = 0

	extracted := upload.ExtractPart(parentDir)
	if extracted != "" {
		uploadsDir = upload.ExternalPathPrefix + extracted + "/.uploadstemp"
	}

	if !upload.PathExists(uploadsDir) {
		return common.RenderJSON(w, r, responseData)
	}
	klog.Infof("r:%+v", r)

	fullPath := filepath.Join(parentDir, fileName)
	dirPath := filepath.Dir(fullPath)

	if !upload.CheckDirExist(dirPath) {
		return common.RenderJSON(w, r, responseData)
	}

	if strings.HasSuffix(fileName, "/") {
		return http.StatusBadRequest, e.New(fmt.Sprintf("full path %s is a dir", fullPath))
	}

	innerIdentifier := upload.MakeUid(fullPath)
	tmpName := innerIdentifier
	upload.UploadsFiles[innerIdentifier] = filepath.Join(uploadsDir, tmpName)

	exist, info := upload.FileInfoManager.ExistFileInfo(innerIdentifier)

	if !exist {
		fileInfo := upload.FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
		}

		if err := upload.FileInfoManager.AddFileInfo(innerIdentifier, fileInfo); err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return http.StatusInternalServerError, e.New("Error save file info")
		}

		klog.Infof("innerIdentifier:%s, fileInfo:%+v", innerIdentifier, fileInfo)
		info = fileInfo
		exist = true
	}

	fileExist, fileLen := upload.FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
	klog.Info("***exist: ", exist, " , fileExist: ", fileExist, " , fileLen: ", fileLen)

	if exist {
		if fileExist {
			if info.Offset != fileLen {
				info.Offset = fileLen
				upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
			}
			klog.Infof("innerIdentifier:%s, info.Offset:%d", innerIdentifier, info.Offset)
			responseData["uploadedBytes"] = info.Offset
		} else if info.Offset == 0 {
			klog.Warningf("innerIdentifier:%s, info.Offset:%d", innerIdentifier, info.Offset)
		} else {
			upload.FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)
		}
	}

	return common.RenderJSON(w, r, responseData)
}

func uploadChunksHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	startTime := time.Now()
	klog.Infof("Function UploadChunks started at: %s\n", startTime)
	defer func() {
		endTime := time.Now()
		klog.Infof("Function UploadChunks ended at: %s\n", endTime)
	}()

	vars := mux.Vars(r)
	uploadID := vars["uid"]

	owner, _, _, uploadsDir, err := upload.GetPVC(r)
	if err != nil {
		return http.StatusBadRequest, e.New("bfl header missing or invalid")
	}

	responseData := map[string]interface{}{
		"success": true,
	}

	var resumableInfo upload.ResumableInfo
	if err := ParseFormData(r, &resumableInfo); err != nil {
		klog.Warningf("uploadID:%s, err:%v", uploadID, err)
		return http.StatusBadRequest, e.New("param invalid")
	}

	p := resumableInfo.ParentDir
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	fileParam, err := models.CreateFileParam(owner, p)
	if err != nil {
		return http.StatusBadRequest, err
	}
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	parentDir := uri + fileParam.Path
	klog.Infof("~~~Debug log: parentDir=%s", parentDir)

	extracted := upload.ExtractPart(parentDir)
	if extracted != "" {
		uploadsDir = upload.ExternalPathPrefix + extracted + "/.uploadstemp"
	}
	if !upload.PathExists(uploadsDir) {
		if err := os.MkdirAll(uploadsDir, os.ModePerm); err != nil {
			klog.Warningf("uploadID:%s, err:%v", uploadID, err)
			return http.StatusInternalServerError, err
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		klog.Warningf("uploadID:%s, Failed to parse file: %v\n", uploadID, err)
		return http.StatusBadRequest, e.New("param invalid")
	}
	defer file.Close()

	resumableInfo.File = header

	fullPath := filepath.Join(parentDir, resumableInfo.ResumableRelativePath)
	innerIdentifier := upload.MakeUid(fullPath)
	tmpName := innerIdentifier
	upload.UploadsFiles[innerIdentifier] = filepath.Join(uploadsDir, tmpName)

	exist, info := upload.FileInfoManager.ExistFileInfo(innerIdentifier)
	if !exist {
		klog.Warningf("innerIdentifier %s not exist", innerIdentifier)
	}

	if !exist || innerIdentifier != info.ID {
		upload.RemoveTempFileAndInfoFile(tmpName, uploadsDir)
		if info.Offset != 0 {
			info.Offset = 0
			upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
		}

		if !upload.CheckDirExist(parentDir) {
			klog.Warningf("Parent dir %s is not exist or is not a dir", resumableInfo.ParentDir)
			return http.StatusBadRequest, e.New("parent dir is not exist or is not a dir")
		}

		dirPath := filepath.Dir(fullPath)
		if !upload.CheckDirExist(dirPath) {
			if err := files.MkdirAllWithChown(nil, dirPath, os.ModePerm); err != nil {
				klog.Error("err:", err)
				return http.StatusInternalServerError, err
			}
		}

		if resumableInfo.ResumableRelativePath == "" {
			return http.StatusBadRequest, e.New("file_relative_path invalid")
		}

		if strings.HasSuffix(resumableInfo.ResumableRelativePath, "/") {
			klog.Warningf("full path %s is a dir", fullPath)
			return http.StatusBadRequest, e.New(fmt.Sprintf("full path %s is a dir", fullPath))
		}

		if !upload.CheckType(resumableInfo.ResumableType) {
			klog.Warningf("unsupported filetype:%s", resumableInfo.ResumableType)
			return http.StatusBadRequest, e.New(fmt.Sprintf("unsupported filetype:%s", resumableInfo.ResumableType))
		}

		if !upload.CheckSize(resumableInfo.ResumableTotalSize) {
			if info.Offset == info.FileSize {
				klog.Warningf("All file chunks have been uploaded, skip upload")
				finishData := []map[string]interface{}{
					{
						"name": resumableInfo.ResumableFilename,
						"id":   upload.MakeUid(info.FullPath),
						"size": info.FileSize,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(finishData)
			}
			klog.Warningf("Unsupported file size uploadSize:%d", resumableInfo.ResumableTotalSize)
			return http.StatusBadRequest, e.New("unsupported file size")
		}

		info.FileSize = resumableInfo.ResumableTotalSize
		upload.FileInfoManager.UpdateInfo(innerIdentifier, info)

		oExist, oInfo := upload.FileInfoManager.ExistFileInfo(innerIdentifier)
		oFileExist, oFileLen := upload.FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
		if oExist {
			if oFileExist {
				if oInfo.Offset != oFileLen {
					oInfo.Offset = oFileLen
					upload.FileInfoManager.UpdateInfo(innerIdentifier, oInfo)
				}
			} else if oInfo.Offset == 0 {
			} else {
				upload.FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)
			}
		}

		fileInfo := upload.FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
			FileMetaData: upload.FileMetaData{
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

		if err = upload.FileInfoManager.AddFileInfo(innerIdentifier, fileInfo); err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return http.StatusInternalServerError, e.New("error save file info")
		}

		info = fileInfo
	} else {
		fileInfo := upload.FileInfo{
			ID:     innerIdentifier,
			Offset: 0,
			FileMetaData: upload.FileMetaData{
				FileRelativePath: resumableInfo.ResumableRelativePath,
				FileType:         resumableInfo.ResumableType,
				FileSize:         resumableInfo.ResumableTotalSize,
				StoragePath:      parentDir,
				FullPath:         fullPath,
			},
		}

		upload.FileInfoManager.UpdateInfo(innerIdentifier, fileInfo)
		if err != nil {
			klog.Warningf("innerIdentifier:%s, err:%v", innerIdentifier, err)
			return http.StatusInternalServerError, e.New("error save file info")
		}

		info = fileInfo
	}

	fileExist, fileLen := upload.FileInfoManager.CheckTempFile(innerIdentifier, uploadsDir)
	if fileExist {
		if info.Offset != fileLen {
			info.Offset = fileLen
			upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
		}
	}

	fileHeader := resumableInfo.File
	size := fileHeader.Size

	ranges := r.Header.Get("Content-Range")
	var offsetStart, offsetEnd int64
	var parsed bool
	if ranges != "" {
		offsetStart, offsetEnd, parsed = upload.ParseContentRange(ranges)
		if !parsed {
			return http.StatusBadRequest, e.New(fmt.Sprintf("Invalid content range:%s", ranges))
		}
	}

	var newFile bool = false
	if info.Offset != offsetStart && offsetStart == 0 {
		newFile = true
		info.Offset = offsetStart
		upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
	}

	if !upload.CheckSize(size) || offsetEnd >= info.FileSize {
		if info.Offset == info.FileSize {
			finishData := []map[string]interface{}{
				{
					"name": resumableInfo.ResumableFilename,
					"id":   upload.MakeUid(info.FullPath),
					"size": info.FileSize,
				},
			}
			return common.RenderJSON(w, r, finishData)
		}
		return http.StatusBadRequest, e.New("Unsupported file size")
	}

	const maxRetries = 100
	for retry := 0; retry < maxRetries; retry++ {
		if info.Offset-offsetEnd > 0 {
			return common.RenderJSON(w, r, responseData)
		}

		if info.Offset == offsetStart || (info.Offset-offsetEnd < 0 && info.Offset-offsetStart > 0) {
			if resumableInfo.MD5 != "" {
				md5, err := upload.CalculateMD5(fileHeader)
				if err != nil {
					return http.StatusInternalServerError, err
				}
				if md5 != resumableInfo.MD5 {
					msg := fmt.Sprintf("Invalid MD5, accepted %s, calculated %s", resumableInfo.MD5, md5)
					return http.StatusBadRequest, e.New(msg)
				}
			}

			fileSize, err := upload.SaveFile(fileHeader, filepath.Join(uploadsDir, tmpName), newFile, offsetStart)
			if err != nil {
				klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
				return http.StatusInternalServerError, err
			}
			info.Offset = fileSize
			upload.FileInfoManager.UpdateInfo(innerIdentifier, info)
			break
		}

		time.Sleep(1000 * time.Millisecond)

		if retry < maxRetries-1 {
			exist, info = upload.FileInfoManager.ExistFileInfo(innerIdentifier)
			if !exist {
				return http.StatusBadRequest, e.New("invalid innerIdentifier")
			}
			continue
		}

		return http.StatusInternalServerError, e.New("Failed to match offset after multiple retries")
	}

	if err = upload.UpdateFileInfo(info, uploadsDir); err != nil {
		klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
		return http.StatusInternalServerError, err
	}

	klog.Info("offsetStart:", offsetStart, ", offsetEnd:", offsetEnd, ", info.Offset:", info.Offset, ", info.FileSize:", info.FileSize)
	if offsetEnd == info.FileSize-1 {
		if err = upload.MoveFileByInfo(info, uploadsDir); err != nil {
			klog.Warningf("innerIdentifier:%s, info:%+v, err:%v", innerIdentifier, info, err)
			return http.StatusInternalServerError, err
		}
		upload.FileInfoManager.DelFileInfo(innerIdentifier, tmpName, uploadsDir)

		finishData := []map[string]interface{}{
			{
				"name": resumableInfo.ResumableFilename,
				"id":   upload.MakeUid(info.FullPath),
				"size": info.FileSize,
			},
		}
		return common.RenderJSON(w, r, finishData)
	}
	return common.RenderJSON(w, r, responseData)
}
