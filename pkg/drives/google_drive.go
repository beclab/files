package drives

import (
	"bytes"
	"context"
	e "errors"
	"files/pkg/common"
	"files/pkg/drives/model"
	"files/pkg/drives/storage"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/img"
	"files/pkg/models"
	"files/pkg/parser"
	"files/pkg/pool"
	"files/pkg/preview"
	"files/pkg/redisutils"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

func GetGoogleDriveMetadataFileParam(fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (*model.GoogleDriveResponseData, error) {
	param := &model.ListParam{
		Path:  strings.Trim(fileParam.Path, "/"),
		Drive: fileParam.FileType, // "my_drive",
		Name:  fileParam.Extend,   // "file_name",
	}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := googleDriveStorage.GetFileMetaData(param)
	if err != nil {
		klog.Errorf("Google Drive get_file_meta_data error: %v", err)
		return nil, err
	}

	fileMetadata := res.(*model.GoogleDriveResponse)
	return fileMetadata.Data, nil
}

func GetGoogleDriveMetadata(src string, w http.ResponseWriter, r *http.Request) (*model.GoogleDriveResponseData, error) {
	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	param := &model.ListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := googleDriveStorage.GetFileMetaData(param)
	if err != nil {
		klog.Errorf("Google Drive get_file_meta_data error: %v", err)
		return nil, err
	}

	fileMetadata := res.(*model.GoogleDriveResponse)
	return fileMetadata.Data, nil
}

type GoogleDriveIdFocusedMetaInfos struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Name         string `json:"name"`
	FileSize     int64  `json:"fileSize"`
	Size         int64  `json:"size"`
	IsDir        bool   `json:"is_dir"`
	CanDownload  bool   `json:"canDownload"`
	CanExport    bool   `json:"canExport"`
	ExportSuffix string `json:"exportSuffix"`
}

func GetGoogleDriveIdFocusedMetaInfosFileParam(task *pool.Task, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (info *GoogleDriveIdFocusedMetaInfos, err error) {
	info = nil
	err = nil

	param := &model.ListParam{
		Path:  strings.Trim(fileParam.Path, "/"),
		Drive: fileParam.FileType, // "my_drive",
		Name:  fileParam.Extend,   // "file_name",
	}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := googleDriveStorage.GetFileMetaData(param)
	if err != nil {
		TaskLog(task, "error", fmt.Sprintf("Google Drive get_file_meta_data error: %v", err))
		return
	}

	var fileMetadata = res.(*model.GoogleDriveResponse)

	if !fileMetadata.IsSuccess() {
		err = e.New(fileMetadata.FailMessage())
		TaskLog(task, "error", fileMetadata.FailMessage())
		return
	}

	info = &GoogleDriveIdFocusedMetaInfos{
		ID:           strings.Trim(fileParam.Path, "/"),
		Path:         fileMetadata.Data.Path,
		Name:         fileMetadata.Data.Name,
		Size:         fileMetadata.Data.FileSize,
		FileSize:     fileMetadata.Data.FileSize,
		IsDir:        fileMetadata.Data.IsDir,
		CanDownload:  fileMetadata.Data.CanDownload,
		CanExport:    fileMetadata.Data.CanExport,
		ExportSuffix: fileMetadata.Data.ExportSuffix,
	}
	if info.Path == "/My Drive" {
		info.Name = "/"
	}
	return
}

func GetGoogleDriveIdFocusedMetaInfos(task *pool.Task, src string, w http.ResponseWriter, r *http.Request) (info *GoogleDriveIdFocusedMetaInfos, err error) {
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	info = nil
	err = nil

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)
	if strings.Index(pathId, "/") != -1 {
		err = e.New("PathId Parse Error")
		TaskLog(task, "error", "PathId Parse Error")
		return
	}

	param := &model.ListParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := googleDriveStorage.GetFileMetaData(param)
	if err != nil {
		TaskLog(task, "error", fmt.Sprintf("Google Drive get_file_meta_data error: %v", err))
		return
	}

	var fileMetadata = res.(*model.GoogleDriveResponse)

	if !fileMetadata.IsSuccess() {
		err = e.New(fileMetadata.FailMessage())
		TaskLog(task, "error", fileMetadata.FailMessage())
		return
	}

	info = &GoogleDriveIdFocusedMetaInfos{
		ID:           pathId,
		Path:         fileMetadata.Data.Path,
		Name:         fileMetadata.Data.Name,
		Size:         fileMetadata.Data.FileSize,
		FileSize:     fileMetadata.Data.FileSize,
		IsDir:        fileMetadata.Data.IsDir,
		CanDownload:  fileMetadata.Data.CanDownload,
		CanExport:    fileMetadata.Data.CanExport,
		ExportSuffix: fileMetadata.Data.ExportSuffix,
	}
	if info.Path == "/My Drive" {
		info.Name = "/"
	}
	return
}

func CopyGoogleDriveSingleFile(task *pool.Task, srcFileParam, dstFileParam *models.FileParam, w http.ResponseWriter, r *http.Request, fileSize int64) error {
	//srcDrive, srcName, srcPathId, srcFilename := ParseGoogleDrivePath(src)
	srcDrive := srcFileParam.FileType
	srcName := srcFileParam.Extend
	srcPathId, srcFilename := filepath.Split(srcFileParam.Path)
	srcPathId = strings.Trim(srcPathId, "/")
	TaskLog(task, "info", "srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		TaskLog(task, "info", "Src parse failed.")
		return nil
	}
	//dstDrive, dstName, dstPathId, dstFilename := ParseGoogleDrivePath(dst)
	dstDrive := dstFileParam.FileType
	dstName := dstFileParam.Extend
	dstPathId, dstFilename := filepath.Split(dstFileParam.Path)
	dstPathId = strings.Trim(dstPathId, "/")
	TaskLog(task, "info", "dstDrive:", dstDrive, "dstName:", dstName, "dstPathId:", dstPathId, "dstFilename:", dstFilename)
	if dstPathId == "" || dstFilename == "" {
		TaskLog(task, "info", "Dst parse failed.")
		return nil
	}
	dstFilename = strings.TrimSuffix(dstFilename, "/")

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.CopyFileParam{
		CloudFilePath:     srcPathId,   // id of "path/to/cloud/file.txt",
		NewCloudDirectory: dstPathId,   // id of "new/cloud/directory",
		NewCloudFileName:  dstFilename, // "new_file_name.txt",
		Drive:             dstDrive,    // "my_drive",
		Name:              dstName,     // "file_name",
	}

	res, err := googleDriveStorage.CopyFile(param)
	if err != nil {
		TaskLog(task, "error", fmt.Sprintf("Google Drive copy_file error: %v", err))
		return fmt.Errorf("Google Drive copy_file error: %v", err)
	}

	_ = res

	_, _, right := CalculateProgressRange(task, fileSize)
	task.Transferred += fileSize
	task.Progress = right

	return nil
}

func CopyGoogleDriveFolder(task *pool.Task, srcFileParam, dstFileParam *models.FileParam, w http.ResponseWriter, r *http.Request, srcPath string) error {
	//srcDrive, srcName, srcPathId, srcFilename := ParseGoogleDrivePath(src)
	srcDrive := srcFileParam.FileType
	srcName := srcFileParam.Extend
	srcPathId, srcFilename := filepath.Split(srcFileParam.Path)
	srcPathId = strings.Trim(srcPathId, "/")
	TaskLog(task, "info", "srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		klog.Infoln("Src parse failed.")
		return nil
	}
	//dstDrive, dstName, dstPathId, dstFilename := ParseGoogleDrivePath(dst)
	dstDrive := dstFileParam.FileType
	dstName := dstFileParam.Extend
	dstPathId, dstFilename := filepath.Split(dstFileParam.Path)
	dstPathId = strings.Trim(dstPathId, "/")
	TaskLog(task, "info", "dstDrive:", dstDrive, "dstName:", dstName, "dstPathId:", dstPathId, "dstFilename:", dstFilename)
	if dstPathId == "" || dstFilename == "" {
		klog.Infoln("Dst parse failed.")
		return nil
	}

	var googleDriveStorage = &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	var CopyTempGoogleDrivePathIdCache = make(map[string]string)
	var recursivePath = srcPath
	var recursivePathId = srcPathId
	var A []*model.GoogleDriveResponseData
	for {
		select {
		case <-task.Ctx.Done():
			return nil
		default:
		}

		TaskLog(task, "info", "len(A): ", len(A))

		var isDir = true
		var firstItem *model.GoogleDriveResponseData
		if len(A) > 0 {
			firstItem = A[0]
			recursivePathId = firstItem.Meta.ID
			recursivePath = firstItem.Path
			isDir = firstItem.IsDir
		}

		if isDir {
			var parentPathId string
			var folderName string
			if srcPathId == recursivePathId {
				parentPathId = dstPathId
				folderName = dstFilename
			} else {
				parentPathId = CopyTempGoogleDrivePathIdCache[filepath.Dir(firstItem.Path)]
				folderName = filepath.Base(firstItem.Path)
			}
			postParam := &model.PostParam{
				ParentPath: parentPathId,
				FolderName: folderName,
				Drive:      srcDrive,
				Name:       srcName,
			}

			folderres, err := googleDriveStorage.CreateFolder(postParam)
			if err != nil {
				TaskLog(task, "error", fmt.Sprintf("Google Drive create_folder error: %v", err))
				return err
			}

			createFolder := folderres.(*model.GoogleDriveResponse)
			CopyTempGoogleDrivePathIdCache[recursivePath] = createFolder.Data.Meta.ID

			// list it and get its sub folders and files
			firstParam := &model.ListParam{
				Path:  recursivePathId,
				Drive: srcDrive,
				Name:  srcName,
			}

			TaskLog(task, "info", "firstParam pathId:", recursivePathId)
			listres, err := googleDriveStorage.List(firstParam)
			if err != nil {
				TaskLog(task, "error", fmt.Sprintf("Google Drive list error: %v", err))
				return err
			}

			files := listres.(*model.GoogleDriveListResponse)

			if len(A) == 0 {
				A = files.Data
			} else {
				A = append(files.Data, A[1:]...)
			}
		} else {
			if len(A) > 0 {
				TaskLog(task, "info", CopyTempGoogleDrivePathIdCache)
				copyPathPrefix := "/Drive/google/" + srcName + "/"
				copySrc := copyPathPrefix + firstItem.Meta.ID + "/"
				parentPathId := CopyTempGoogleDrivePathIdCache[filepath.Dir(firstItem.Path)]
				copyDst := filepath.Join(copyPathPrefix+parentPathId, firstItem.Name)
				TaskLog(task, "info", "copySrc: ", copySrc)
				TaskLog(task, "info", "copyDst: ", copyDst)
				copySrcFileParam := &models.FileParam{
					Owner:    srcFileParam.Owner,
					FileType: srcFileParam.FileType,
					Extend:   srcFileParam.Extend,
					Path:     firstItem.Meta.ID + "/",
				}
				copyDstFileParam := &models.FileParam{
					Owner:    dstFileParam.Owner,
					FileType: dstFileParam.FileType,
					Extend:   dstFileParam.Extend,
					Path:     filepath.Join(parentPathId, firstItem.Name),
				}
				err := CopyGoogleDriveSingleFile(task, copySrcFileParam, copyDstFileParam, w, r, firstItem.FileSize)
				if err != nil {
					return err
				}
				A = A[1:]
			}
		}
		if len(A) == 0 {
			return nil
		}
	}
}

func GooglePauseTask(taskId string, w http.ResponseWriter, r *http.Request) error {
	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := googleDriveStorage.PauseTask(taskId)
	if err != nil {
		klog.Errorf("Google Drive pause task error: %v", err)
		return err
	}

	taskResp := res.(*model.TaskResponse)

	if taskResp.StatusCode == "SUCCESS" {
		return e.New("Task paused successfully")
	} else {
		klog.Errorln("Failed to pause task")
	}
	return nil
}

func GoogleFileToBufferFileParam(task *pool.Task, fileParam *models.FileParam, bufferFilePath, bufferFileName string, w http.ResponseWriter, r *http.Request, left, right int) (string, error) {
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}

	param := &model.DownloadAsyncParam{
		LocalFolder:   bufferFilePath,
		CloudFilePath: strings.Trim(fileParam.Path, "/"),
		Drive:         fileParam.FileType,
		Name:          fileParam.Extend,
	}
	if bufferFileName != "" {
		param.LocalFileName = bufferFileName
	}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := googleDriveStorage.DownloadAsync(param)
	if err != nil {
		klog.Errorln("Error calling drive/download_async:", err)
		return bufferFileName, err
	}

	downloadAsyncResp := res.(*model.TaskResponse)
	// todo check Success

	taskId := downloadAsyncResp.Data.ID
	taskParam := &model.QueryTaskParam{
		TaskIds: []string{taskId},
	}

	if task == nil {
		for {
			time.Sleep(1000 * time.Millisecond)
			res, err := googleDriveStorage.QueryTask(taskParam)
			if err != nil {
				klog.Errorln("Error calling drive/download_async:", err)
				return bufferFileName, err
			}
			var taskQueryResp = res.(*model.TaskQueryResponse)

			if len(taskQueryResp.Data) == 0 {
				return bufferFileName, e.New("Task Info Not Found")
			}
			if taskQueryResp.Data[0].Status != "Waiting" && taskQueryResp.Data[0].Status != "InProgress" {
				if taskQueryResp.Data[0].Status == "Completed" {
					return bufferFileName, nil
				}
				return bufferFileName, e.New(taskQueryResp.Data[0].Status)
			}
		}
	}

	for {
		select {
		case <-task.Ctx.Done():
			err = GooglePauseTask(taskId, w, r)
			if err != nil {
				return bufferFileName, err
			}

		default:
			time.Sleep(1000 * time.Millisecond)
			res, err := googleDriveStorage.QueryTask(taskParam)
			if err != nil {
				klog.Errorln("Error calling drive/download_async:", err)
				return bufferFileName, err
			}
			var taskQueryResp = res.(*model.TaskQueryResponse)
			if len(taskQueryResp.Data) == 0 {
				return bufferFileName, e.New("Task Info Not Found")
			}
			if taskQueryResp.Data[0].Status != "Waiting" && taskQueryResp.Data[0].Status != "InProgress" {
				if taskQueryResp.Data[0].Status == "Completed" {
					task.Progress = right
					return bufferFileName, nil
				}
				return bufferFileName, e.New(taskQueryResp.Data[0].Status)
			} else if taskQueryResp.Data[0].Status == "InProgress" {
				task.Progress = MapProgress(taskQueryResp.Data[0].Progress, left, right)
			}
		}
	}
}

func GoogleFileToBuffer(task *pool.Task, src, bufferFilePath, bufferFileName string, w http.ResponseWriter, r *http.Request, left, right int) (string, error) {
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}
	srcDrive, srcName, srcPathId, srcFilename := ParseGoogleDrivePath(src)
	klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPathId:", srcPathId, "srcFilename:", srcFilename)
	if srcPathId == "" {
		klog.Infoln("Src parse failed.")
		return bufferFileName, nil
	}

	param := &model.DownloadAsyncParam{
		LocalFolder:   bufferFilePath,
		CloudFilePath: srcPathId,
		Drive:         srcDrive,
		Name:          srcName,
	}
	if bufferFileName != "" {
		param.LocalFileName = bufferFileName
	}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := googleDriveStorage.DownloadAsync(param)
	if err != nil {
		klog.Errorln("Error calling drive/download_async:", err)
		return bufferFileName, err
	}

	downloadAsyncResp := res.(*model.TaskResponse)
	// todo check Success

	taskId := downloadAsyncResp.Data.ID
	taskParam := &model.QueryTaskParam{
		TaskIds: []string{taskId},
	}

	if task == nil {
		for {
			time.Sleep(1000 * time.Millisecond)
			res, err := googleDriveStorage.QueryTask(taskParam)
			if err != nil {
				klog.Errorln("Error calling drive/download_async:", err)
				return bufferFileName, err
			}
			var taskQueryResp = res.(*model.TaskQueryResponse)

			if len(taskQueryResp.Data) == 0 {
				return bufferFileName, e.New("Task Info Not Found")
			}
			if taskQueryResp.Data[0].Status != "Waiting" && taskQueryResp.Data[0].Status != "InProgress" {
				if taskQueryResp.Data[0].Status == "Completed" {
					return bufferFileName, nil
				}
				return bufferFileName, e.New(taskQueryResp.Data[0].Status)
			}
		}
	}

	for {
		select {
		case <-task.Ctx.Done():
			err = GooglePauseTask(taskId, w, r)
			if err != nil {
				return bufferFileName, err
			}

		default:
			time.Sleep(1000 * time.Millisecond)
			res, err := googleDriveStorage.QueryTask(taskParam)
			if err != nil {
				klog.Errorln("Error calling drive/download_async:", err)
				return bufferFileName, err
			}
			var taskQueryResp = res.(*model.TaskQueryResponse)
			if len(taskQueryResp.Data) == 0 {
				return bufferFileName, e.New("Task Info Not Found")
			}
			if taskQueryResp.Data[0].Status != "Waiting" && taskQueryResp.Data[0].Status != "InProgress" {
				if taskQueryResp.Data[0].Status == "Completed" {
					task.Progress = right
					return bufferFileName, nil
				}
				return bufferFileName, e.New(taskQueryResp.Data[0].Status)
			} else if taskQueryResp.Data[0].Status == "InProgress" {
				task.Progress = MapProgress(taskQueryResp.Data[0].Progress, left, right)
			}
		}
	}
}

func GoogleBufferToFile(task *pool.Task, bufferFilePath string, dstFileParam *models.FileParam, w http.ResponseWriter, r *http.Request, left, right int) (int, error) {
	//dstDrive, dstName, dstPathId, dstFilename := ParseGoogleDrivePath(dst)
	dstDrive := dstFileParam.FileType
	dstName := dstFileParam.Extend
	dstPathId, dstFilename := filepath.Split(dstFileParam.Path)
	klog.Infoln("srcDrive:", dstDrive, "srcName:", dstName, "srcPathId:", dstPathId, "srcFilename:", dstFilename)
	if dstPathId == "" {
		klog.Infoln("Src parse failed.")
		return http.StatusBadRequest, nil
	}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.UploadAsyncParam{
		ParentPath:    dstPathId,
		LocalFilePath: bufferFilePath,
		Drive:         dstDrive,
		Name:          dstName,
	}

	uploadRes, err := googleDriveStorage.UploadAsync(param)
	if err != nil {
		klog.Errorf("UploadAsync error: %v", err)
		return common.ErrToStatus(err), err
	}

	uploadResp := uploadRes.(*model.TaskResponse)

	taskId := uploadResp.Data.ID
	taskParam := &model.QueryTaskParam{
		TaskIds: []string{taskId},
	}

	for {
		select {
		case <-task.Ctx.Done():
			err = GooglePauseTask(taskId, w, r)
			if err != nil {
				return common.ErrToStatus(err), err
			}
		default:
			time.Sleep(500 * time.Millisecond)
			queryTaskRes, err := googleDriveStorage.QueryTask(taskParam)
			if err != nil {
				klog.Errorf("DownloadAsync error: %v", err)
				return common.ErrToStatus(err), err
			}
			queryTask := queryTaskRes.(*model.TaskQueryResponse)

			if len(queryTask.Data) == 0 {
				err = e.New("Task Info Not Found")
				return common.ErrToStatus(err), err
			}
			if queryTask.Data[0].Status != "Waiting" && queryTask.Data[0].Status != "InProgress" {
				if queryTask.Data[0].Status == "Completed" {
					task.Progress = right
					return http.StatusOK, nil
				}
				err = e.New(queryTask.Data[0].Status)
				return common.ErrToStatus(err), err
			} else if queryTask.Data[0].Status == "InProgress" {
				if task != nil {
					task.Progress = MapProgress(queryTask.Data[0].Progress, left, right)
				}
			}
		}
	}
}

func MoveGoogleDriveFolderOrFiles(task *pool.Task, srcFileParam, dstFileParam *models.FileParam, w http.ResponseWriter, r *http.Request) error {
	//srcDrive, srcName, srcPathId, _ := ParseGoogleDrivePath(src)
	//_, _, dstPathId, _ := ParseGoogleDrivePath(dst)
	srcDrive := srcFileParam.FileType
	srcName := srcFileParam.Extend
	srcPathId := strings.Trim(srcFileParam.Path, "/")
	dstPathId := strings.Trim(dstFileParam.Path, "/")

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.MoveFileParam{
		CloudFilePath:     srcPathId,
		NewCloudDirectory: dstPathId,
		Drive:             srcDrive, // "my_drive",
		Name:              srcName,  // "file_name",
	}

	_, err := googleDriveStorage.MoveFile(param)
	if err != nil {
		TaskLog(task, "error", "Error calling drive/move_file:", err)
		return err
	}

	// todo error
	return nil
}

func ParseGoogleDrivePath(path string) (drive, name, dir, filename string) {
	if strings.HasPrefix(path, "/Drive/google") || strings.HasPrefix(path, "/drive/google") {
		path = path[13:]
		drive = "google"
	}

	slashes := []int{}
	for i, char := range path {
		if char == '/' {
			slashes = append(slashes, i)
		}
	}

	if len(slashes) < 2 {
		klog.Infoln("Path does not contain enough slashes.")
		return drive, "", "", ""
	}

	name = path[1:slashes[1]]

	if len(slashes) == 2 {
		return drive, name, "/", path[slashes[1]+1:]
	}

	dir = path[slashes[1]+1 : slashes[2]]
	filename = strings.TrimPrefix(path[slashes[2]:], "/")

	if dir == "root" {
		dir = "/"
	}

	return drive, name, dir, filename
}

type GoogleDriveResourceService struct {
	BaseResourceService
}

func (rc *GoogleDriveResourceService) DeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		_, status, err := ResourceDeleteGoogle(fileCache, r.URL.Path, w, r, true)
		return status, err
	}
}

func (rc *GoogleDriveResourceService) PutHandler(fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		// not public api for google drive, so it is not implemented
		return http.StatusNotImplemented, fmt.Errorf("google drive does not supoort editing files")
	}
}

func (rc *GoogleDriveResourceService) PatchHandler(fileCache fileutils.FileCache, fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		return ResourcePatchGoogle(fileCache, fileParam, w, r)
	}
}

func (rc *GoogleDriveResourceService) BatchDeleteHandler(fileCache fileutils.FileCache, fileParams []*models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		failDirents := []string{}
		for _, fileParam := range fileParams {
			_, status, err := ResourceDeleteGoogleFileParam(fileCache, fileParam, w, r, true)
			if (status != http.StatusOK && status != 0) || err != nil {
				klog.Errorf("delete %s failed with status %d and err %v", fileParam.Path, status, err)
				failDirents = append(failDirents, fileParam.Path)
				continue
			}
		}
		if len(failDirents) > 0 {
			return http.StatusInternalServerError, fmt.Errorf("delete %s failed", strings.Join(failDirents, "; "))
		}
		return common.RenderJSON(w, r, map[string]interface{}{"msg": "all dirents deleted"})
	}
}

func (rs *GoogleDriveResourceService) RawHandler(fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		bflName := r.Header.Get("X-Bfl-User")
		metaData, err := GetGoogleDriveMetadataFileParam(fileParam, w, r)
		if err != nil {
			klog.Error(err)
			return common.ErrToStatus(err), err
		}
		if metaData.IsDir {
			return http.StatusNotImplemented, fmt.Errorf("doesn't support directory download for google drive now")
		}
		return RawFileHandlerGoogleFileParam(fileParam, w, r, metaData, bflName)
	}
}

func (rc *GoogleDriveResourceService) PasteSame(task *pool.Task, action, src, dst string, srcFileParam, dstFileParam *models.FileParam,
	fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	var err error
	switch action {
	case "copy":
		if !strings.HasSuffix(src, "/") {
			src += "/"
		}
		var metaInfo *GoogleDriveIdFocusedMetaInfos
		metaInfo, err = GetGoogleDriveIdFocusedMetaInfosFileParam(task, srcFileParam, w, r)
		if err != nil {
			TaskLog(task, "info", fmt.Sprintf("%s from %s to %s failed when getting meta info", action, src, dst))
			TaskLog(task, "error", err.Error())
			pool.FailTask(task.ID)
			return err
		}

		if metaInfo.IsDir {
			err = CopyGoogleDriveFolder(task, srcFileParam, dstFileParam, w, r, metaInfo.Path)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel() // Ensure context is canceled when main exits
		go SimulateProgress(ctx, 0, 99, task.TotalFileSize, 50000000, task)
		err = CopyGoogleDriveSingleFile(task, srcFileParam, dstFileParam, w, r, metaInfo.FileSize)

	case "move":
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel() // Ensure context is canceled when main exits
		go SimulateProgress(ctx, 0, 99, task.TotalFileSize, 50000000, task)

		if !strings.HasSuffix(src, "/") {
			src += "/"
		}
		err = MoveGoogleDriveFolderOrFiles(task, srcFileParam, dstFileParam, w, r)

	default:
		err = fmt.Errorf("unknown action: %s", action)
	}

	if err != nil {
		TaskLog(task, "info", fmt.Sprintf("%s from %s to %s failed", action, src, dst))
		TaskLog(task, "error", err.Error())
		pool.FailTask(task.ID)
	} else {
		TaskLog(task, "info", fmt.Sprintf("%s from %s to %s successfully", action, src, dst))
		pool.CompleteTask(task.ID)
	}
	return err
}

func (rs *GoogleDriveResourceService) PasteDirFrom(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
	dstFileParam *models.FileParam, dstType, dst string, d *common.Data,
	fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	mode := fileMode

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteDirTo(task, fs, src, dst, srcFileParam, dstFileParam, mode, fileCount, w, r, d, driveIdCache)
	if err != nil {
		return err
	}

	//srcUri, err := srcFileParam.GetResourceUri()
	//if err != nil {
	//	return err
	//}

	dstUri, err := dstFileParam.GetResourceUri()
	if err != nil {
		return err
	}

	var fdstBase string = dst
	if driveIdCache[src] != "" {
		fdstBase = filepath.Join(filepath.Dir(filepath.Dir(strings.TrimSuffix(dst, "/"))), driveIdCache[src])
	}

	//if !strings.HasSuffix(src, "/") {
	//	src += "/"
	//}

	//srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	param := &model.ListParam{
		Path:  strings.Trim(srcFileParam.Path, "/"),
		Drive: srcFileParam.FileType,
		Name:  srcFileParam.Extend,
	}
	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := googleDriveStorage.List(param)
	if err != nil {
		klog.Errorf("List error: %v", err)
		return err
	}
	result := res.(*model.GoogleDriveListResponse)
	for _, item := range result.Data {
		select {
		case <-task.Ctx.Done():
			return nil
		default:
		}

		fsrc := filepath.Join(filepath.Dir(strings.TrimSuffix(src, "/")), item.Meta.ID)
		fdst := filepath.Join(fdstBase, item.Name)
		klog.Infoln(fsrc, fdst)

		fsrcFileParam := &models.FileParam{
			Owner:    srcFileParam.Owner,
			FileType: srcFileParam.FileType,
			Extend:   srcFileParam.Extend,
			Path:     item.Meta.ID,
		}
		fdstFileParam := &models.FileParam{
			Owner:    dstFileParam.Owner,
			FileType: dstFileParam.FileType,
			Extend:   dstFileParam.Extend,
			Path:     strings.TrimPrefix(fdst, strings.TrimPrefix(dstUri, "/data")),
		}
		if item.IsDir {
			err = rs.PasteDirFrom(task, fs, fsrcFileParam, srcType, fsrc, fdstFileParam, dstType, fdst, d, os.FileMode(0755), fileCount, w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			fdst += item.ExportSuffix
			err = rs.PasteFileFrom(task, fs, fsrcFileParam, srcType, fsrc, fdstFileParam, dstType, fdst, d, os.FileMode(0755), fileCount, item.FileSize, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (rs *GoogleDriveResourceService) PasteDirTo(task *pool.Task,
	fs afero.Fs,
	src, dst string,
	srcFileParam, dstFileParam *models.FileParam,
	fileMode os.FileMode,
	fileCount int64,
	w http.ResponseWriter,
	r *http.Request,
	d *common.Data, driveIdCache map[string]string) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	resp, _, err := ResourcePostGoogleFileParam(dstFileParam, w, r, true)
	driveIdCache[src] = resp.Data.Meta.ID
	if err != nil {
		return err
	}
	return nil
}

func (rs *GoogleDriveResourceService) PasteFileFrom(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
	dstFileParam *models.FileParam, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	var bufferPath string

	var err error
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return err
	}

	srcInfo, err := GetGoogleDriveIdFocusedMetaInfosFileParam(nil, srcFileParam, w, r)
	bufferFilePath, err := GenerateBufferFolder(srcInfo.Path, bflName)
	if err != nil {
		return err
	}
	bufferFileName := common.RemoveSlash(srcInfo.Name) + srcInfo.ExportSuffix
	bufferPath = filepath.Join(bufferFilePath, bufferFileName)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)
	task.AddBuffer(bufferPath)

	defer func() {
		logMsg := fmt.Sprintf("Remove copy buffer")
		TaskLog(task, "info", logMsg)
		RemoveDiskBuffer(task, bufferPath, srcType)
	}()

	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return err
	}

	left, mid, right := CalculateProgressRange(task, diskSize)

	_, err = GoogleFileToBufferFileParam(task, srcFileParam, bufferFilePath, bufferFileName, w, r, left, mid)
	if err != nil {
		return err
	}

	// only srcType == google need this now
	rename := r.URL.Query().Get("rename") == "true"
	if rename && dstType != SrcTypeGoogle {
		dst = PasteAddVersionSuffix(dst, dstFileParam, false, files.DefaultFs, w, r)
	}

	if task.Status == "running" {
		handler, err := GetResourceService(dstType)
		if err != nil {
			return err
		}

		err = handler.PasteFileTo(task, fs, bufferPath, dst, srcFileParam, dstFileParam, mode, mid, right, w, r, d, diskSize)
		if err != nil {
			return err
		}
	}

	logMsg := fmt.Sprintf("Copy from %s to %s sucessfully!", src, dst)
	TaskLog(task, "info", logMsg)
	return nil
}

func (rs *GoogleDriveResourceService) PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string,
	srcFileParam, dstFileParam *models.FileParam, fileMode os.FileMode,
	left, right int, w http.ResponseWriter, r *http.Request, d *common.Data, diskSize int64) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	klog.Infoln("Begin to paste!")
	klog.Infoln("dst: ", dst)
	status, err := GoogleBufferToFile(task, bufferPath, dstFileParam, w, r, left, right)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	task.Transferred += diskSize
	return nil
}

func (rs *GoogleDriveResourceService) GetStat(fs afero.Fs, fileParam *models.FileParam, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	metaInfo, err := GetGoogleDriveIdFocusedMetaInfosFileParam(nil, fileParam, w, r)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return nil, metaInfo.Size, 0755, metaInfo.IsDir, nil
}

func (rs *GoogleDriveResourceService) MoveDelete(task *pool.Task, fileCache fileutils.FileCache, fileParam *models.FileParam, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	_, status, err := ResourceDeleteGoogleFileParam(fileCache, fileParam, w, r, true)
	if status != http.StatusOK && status != 0 {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *GoogleDriveResourceService) GeneratePathList(db *gorm.DB, rootPath string, processor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error {
	if rootPath == "" {
		rootPath = "/"
	}

	processedPaths := make(map[string]bool)

	for bflName, cookie := range common.BflCookieCache {
		klog.Infof("Key: %s, Value: %s\n", bflName, cookie)

		tempRequest := &http.Request{}
		tempRequest.Header.Set("Content-Type", "application/json")
		tempRequest.Header.Set("X-Bfl-User", bflName)
		tempRequest.Header.Set("Cookie", cookie)

		googleDriveStorage := &storage.GoogleDriveStorage{
			Owner:   bflName,
			Request: tempRequest,
		}

		accountRes, err := googleDriveStorage.QueryAccount()
		if err != nil {
			klog.Errorf("QueryAccount error: %v", err)
			return err
		}

		accountResp := accountRes.(*model.AccountResponse)
		for _, datum := range accountResp.Data {
			klog.Infof("datum=%v", datum)

			if datum.Type != SrcTypeGoogle {
				continue
			}

			rootParam := &model.ListParam{
				Path:  rootPath,
				Drive: datum.Type,
				Name:  datum.Name,
			}

			listRes, err := googleDriveStorage.List(rootParam)
			if err != nil {
				klog.Errorf("List error: %v", err)
				return err
			}

			listResp := listRes.(*model.GoogleDriveListResponse)

			generator := walkGoogleDriveDirentsGenerator(listResp, googleDriveStorage, datum)

			for dirent := range generator {
				key := fmt.Sprintf("%s:%s", dirent.Drive, dirent.Path)
				processedPaths[key] = true

				_, err = processor(db, dirent.Drive, dirent.Path, dirent.Mtime)
				if err != nil {
					klog.Errorf("generate path list failed: %v\n", err)
					return err
				}
			}
		}
	}

	err := recordsStatusProcessor(db, processedPaths, []string{SrcTypeGoogle}, 1)
	if err != nil {
		klog.Errorf("records status processor failed: %v\n", err)
		return err
	}

	return nil
}

func (rs *GoogleDriveResourceService) GetFileCount(fs afero.Fs, fileParam *models.FileParam, countType string, w http.ResponseWriter, r *http.Request) (int64, error) {
	var count int64

	metaInfo, err := GetGoogleDriveIdFocusedMetaInfosFileParam(nil, fileParam, w, r)
	if err != nil {
		return 0, err
	}

	if !metaInfo.IsDir {
		if countType == "size" {
			count += metaInfo.FileSize
		} else {
			count++
		}
		return count, nil
	}

	queue := []string{strings.Trim(fileParam.Path, "/")}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		param := &model.ListParam{
			Path:  currentID,
			Drive: fileParam.FileType,
			Name:  fileParam.Extend,
		}

		res, err := googleDriveStorage.List(param)
		if err != nil {
			return 0, err
		}

		files := res.(*model.GoogleDriveListResponse)

		for _, item := range files.Data {
			if item.IsDir {
				queue = append(queue, item.Meta.ID)
			} else {
				if countType == "size" {
					count += item.FileSize
				} else {
					count++
				}
			}
		}
	}
	return count, nil
}

func (rs *GoogleDriveResourceService) GetTaskFileInfo(fs afero.Fs, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error) {
	metaInfo, err := GetGoogleDriveIdFocusedMetaInfosFileParam(nil, fileParam, w, r)
	if err != nil {
		return false, "", "", err
	}

	isDir = metaInfo.IsDir
	filename = metaInfo.Name
	fileType = ""
	if !isDir {
		fileType = parser.MimeTypeByExtension(filename)
	}
	return isDir, fileType, filename, nil
}

// just for complement, no need to use now
func (rs *GoogleDriveResourceService) parsePathToURI(path string) (string, string) {
	return SrcTypeDrive, path
}

func ResourceDeleteGoogleFileParam(fileCache fileutils.FileCache, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request, returnResp bool) (*model.GoogleDriveResponse, int, error) {
	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.DeleteParam{
		Path:  strings.Trim(fileParam.Path, "/"),
		Drive: fileParam.FileType, // "my_drive",
		Name:  fileParam.Extend,   // "file_name",
	}

	// delete thumbnails
	var err = delThumbsGoogleFileParam(r.Context(), fileCache, fileParam, w, r)
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	res, err := googleDriveStorage.Delete(param)
	if err != nil {
		klog.Errorf("Google Drive delete error: %v", err)
		return nil, common.ErrToStatus(err), err
	}

	deleteResp := res.(*model.GoogleDriveResponse)
	return deleteResp, 0, nil
}

func ResourceDeleteGoogle(fileCache fileutils.FileCache, src string, w http.ResponseWriter, r *http.Request, returnResp bool) (*model.GoogleDriveResponse, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	klog.Infoln("src Path:", src)
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}

	srcDrive, srcName, pathId, _ := ParseGoogleDrivePath(src)

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.DeleteParam{
		Path:  pathId,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// delete thumbnails
	var err = delThumbsGoogle(r.Context(), fileCache, src, w, r)
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	res, err := googleDriveStorage.Delete(param)
	if err != nil {
		klog.Errorf("Google Drive delete error: %v", err)
		return nil, common.ErrToStatus(err), err
	}

	deleteResp := res.(*model.GoogleDriveResponse)
	return deleteResp, 0, nil
}

func ResourcePostGoogleFileParam(fileParam *models.FileParam, w http.ResponseWriter, r *http.Request, returnResp bool) (*model.GoogleDriveResponse, int, error) {
	pathId, srcNewName := filepath.Split(strings.TrimSuffix(fileParam.Path, "/"))
	pathId = strings.Trim(pathId, "/")

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.PostParam{
		ParentPath: pathId,
		FolderName: srcNewName,
		Drive:      fileParam.FileType, // "my_drive",
		Name:       fileParam.Extend,   // "file_name",
	}

	res, err := googleDriveStorage.CreateFolder(param)
	if err != nil {
		klog.Errorf("Google Drive create_folder error: %v", err)
		return nil, common.ErrToStatus(err), err
	}

	createResp := res.(*model.GoogleDriveResponse)

	return createResp, 0, nil
}

func ResourcePostGoogle(src string, w http.ResponseWriter, r *http.Request, returnResp bool) (*model.GoogleDriveResponse, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	klog.Infoln("src Path:", src)
	src = strings.TrimSuffix(src, "/")

	srcDrive, srcName, pathId, srcNewName := ParseGoogleDrivePath(src)

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.PostParam{
		ParentPath: pathId,
		FolderName: srcNewName,
		Drive:      srcDrive, // "my_drive",
		Name:       srcName,  // "file_name",
	}

	res, err := googleDriveStorage.CreateFolder(param)
	if err != nil {
		klog.Errorf("Google Drive create_folder error: %v", err)
		return nil, common.ErrToStatus(err), err
	}

	createResp := res.(*model.GoogleDriveResponse)

	return createResp, 0, nil
}

func ResourcePatchGoogle(fileCache fileutils.FileCache, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (int, error) {
	dst := r.URL.Query().Get("destination")
	dstFilename, err := common.UnescapeURLIfEscaped(dst)
	if err != nil {
		return http.StatusBadRequest, err
	}

	param := &model.PatchParam{
		Path:        strings.Trim(fileParam.Path, "/"),
		NewFileName: dstFilename,
		Drive:       fileParam.FileType, // "my_drive",
		Name:        fileParam.Extend,   // "file_name",
	}

	googleDriveStorage := &storage.GoogleDriveStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	// delete thumbnails
	err = delThumbsGoogleFileParam(r.Context(), fileCache, fileParam, w, r)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	res, err := googleDriveStorage.Rename(param)
	if err != nil {
		klog.Errorf("Google Drive rename error: %v", err)
		return common.ErrToStatus(err), err
	}

	renameResult := res.(*model.GoogleDriveResponse)
	klog.Infoln("Google Drive Patch Result:", renameResult)

	return common.RenderSuccess(w, r)
}

func setContentDispositionGoogle(w http.ResponseWriter, r *http.Request, fileName string) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(fileName))
	}
}

func previewCacheKeyGoogle(f *model.GoogleDriveResponseData, previewSize preview.PreviewSize) string {
	return fmt.Sprintf("%x%x%x", f.ID, f.Modified.Unix(), previewSize)
}

func createPreviewGoogle(w http.ResponseWriter, r *http.Request, src string, imgSvc preview.ImgService, fileCache fileutils.FileCache,
	file *model.GoogleDriveResponseData, previewSize preview.PreviewSize, bflName string) ([]byte, error) {
	klog.Infoln("!!!!CreatePreview:", previewSize)

	var err error
	diskSize := file.Size
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return nil, err
	}

	bufferFilePath, err := GenerateBufferFolder(file.Path, bflName)
	if err != nil {
		return nil, err
	}
	bufferFileName := common.RemoveSlash(file.Name)
	bufferPath := filepath.Join(bufferFilePath, bufferFileName)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(nil, bufferPath, SrcTypeGoogle)
	}()

	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return nil, err
	}
	_, err = GoogleFileToBuffer(nil, src, bufferFilePath, bufferFileName, w, r, 0, 0)
	if err != nil {
		return nil, err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	var (
		width   int
		height  int
		options []img.Option
	)

	switch {
	case previewSize == preview.PreviewSizeBig:
		width = 1080
		height = 1080
		options = append(options, img.WithMode(img.ResizeModeFit), img.WithQuality(img.QualityMedium))
	case previewSize == preview.PreviewSizeThumb:
		width = 256
		height = 256
		options = append(options, img.WithMode(img.ResizeModeFill), img.WithQuality(img.QualityLow), img.WithFormat(img.FormatJpeg))
	default:
		return nil, img.ErrUnsupportedFormat
	}

	buf := &bytes.Buffer{}
	if err := imgSvc.Resize(context.Background(), fd, width, height, buf, options...); err != nil {
		return nil, err
	}

	go func() {
		cacheKey := previewCacheKeyGoogle(file, previewSize)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			klog.Errorf("failed to cache resized image: %v", err)
		}
	}()

	return buf.Bytes(), nil
}

func RawFileHandlerGoogleFileParam(fileParam *models.FileParam, w http.ResponseWriter, r *http.Request, file *model.GoogleDriveResponseData, bflName string) (int, error) {
	var err error
	diskSize := file.Size
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	bufferFilePath, err := GenerateBufferFolder(file.Path, bflName)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	bufferFileName := common.RemoveSlash(file.Name)
	bufferPath := filepath.Join(bufferFilePath, bufferFileName)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(nil, bufferPath, SrcTypeGoogle)
	}()

	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	_, err = GoogleFileToBufferFileParam(nil, fileParam, bufferFilePath, bufferFileName, w, r, 0, 0)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	defer fd.Close()

	setContentDispositionGoogle(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.Modified, fd)

	return 0, nil
}

func RawFileHandlerGoogle(src string, w http.ResponseWriter, r *http.Request, file *model.GoogleDriveResponseData, bflName string) (int, error) {
	var err error
	diskSize := file.Size
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	bufferFilePath, err := GenerateBufferFolder(file.Path, bflName)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	bufferFileName := common.RemoveSlash(file.Name)
	bufferPath := filepath.Join(bufferFilePath, bufferFileName)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(nil, bufferPath, SrcTypeGoogle)
	}()

	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	_, err = GoogleFileToBuffer(nil, src, bufferFilePath, bufferFileName, w, r, 0, 0)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	defer fd.Close()

	setContentDispositionGoogle(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.Modified, fd)

	return 0, nil
}

func delThumbsGoogleFileParam(ctx context.Context, fileCache fileutils.FileCache, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) error {
	metaData, err := GetGoogleDriveMetadataFileParam(fileParam, w, r)
	if err != nil {
		klog.Errorf("Google Drive get_file_meta_data error: %v", err)
		return err
	}

	for _, previewSizeName := range preview.PreviewSizeNames() {
		size, _ := preview.ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKeyGoogle(metaData, size)
		if err := fileCache.Delete(ctx, cacheKey); err != nil {
			return err
		}
		err := redisutils.DelThumbRedisKey(redisutils.GetFileName(cacheKey))
		if err != nil {
			return err
		}
	}

	return nil
}

func delThumbsGoogle(ctx context.Context, fileCache fileutils.FileCache, src string, w http.ResponseWriter, r *http.Request) error {
	metaData, err := GetGoogleDriveMetadata(src, w, r)
	if err != nil {
		klog.Errorf("Google Drive get_file_meta_data error: %v", err)
		return err
	}

	for _, previewSizeName := range preview.PreviewSizeNames() {
		size, _ := preview.ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKeyGoogle(metaData, size)
		if err := fileCache.Delete(ctx, cacheKey); err != nil {
			return err
		}
		err := redisutils.DelThumbRedisKey(redisutils.GetFileName(cacheKey))
		if err != nil {
			return err
		}
	}

	return nil
}

func walkGoogleDriveDirentsGenerator(list *model.GoogleDriveListResponse, googleDriveStorage *storage.GoogleDriveStorage, datum *model.AccounResponseItem) <-chan DirentGeneratedEntry {
	ch := make(chan DirentGeneratedEntry)
	go func() {
		defer close(ch)

		queue := make([]*model.GoogleDriveResponseData, 0)
		list.Lock()
		queue = append(queue, list.Data...)
		list.Unlock()

		for len(queue) > 0 {
			firstItem := queue[0]
			queue = queue[1:]

			if firstItem.IsDir {
				fullPath := filepath.Join(SrcTypeGoogle, datum.Name, firstItem.Meta.ID) + "/"
				entry := DirentGeneratedEntry{
					Drive: SrcTypeGoogle,
					Path:  fullPath,
					Mtime: firstItem.Modified,
				}
				ch <- entry

				firstParam := &model.ListParam{
					Path:  firstItem.Meta.ID,
					Drive: datum.Type,
					Name:  datum.Name,
				}
				nextRes, err := googleDriveStorage.List(firstParam)
				if err != nil {
					klog.Error(err)
					continue
				}
				nextList := nextRes.(*model.GoogleDriveListResponse)
				// todo if data is nil, or failed
				queue = append(queue, nextList.Data...)
			}
		}
	}()
	return ch
}
