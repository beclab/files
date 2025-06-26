package drives

import (
	"bytes"
	"context"
	e "errors"
	"files/pkg/common"
	"files/pkg/drives/model"
	"files/pkg/drives/storage"
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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

type CloudDriveFocusedMetaInfos struct {
	Key      string `json:"key"`
	Path     string `json:"path"`
	Name     string `json:"name"`
	FileSize int64  `json:"fileSize"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
}

func CloudDriveNormalizationPath(path, srcType string, same, addSuffix bool) string {
	if same != (srcType == SrcTypeAWSS3) {
		return path
	}

	if addSuffix && !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	if !addSuffix && strings.HasSuffix(path, "/") {
		return strings.TrimSuffix(path, "/")
	}

	return path
}

func GetCloudDriveMetadataFileParam(fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (*model.CloudResponseData, error) {
	param := &model.ListParam{
		Path:  fileParam.Path,
		Drive: fileParam.FileType, // "my_drive",
		Name:  fileParam.Extend,   // "file_name",
	}

	cloudStorage := &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := cloudStorage.GetFileMetaData(param)
	if err != nil {
		klog.Errorf("Cloud Drive get_file_meta_data error: %v", err)
		return nil, err
	}

	fileMetaDataResp := res.(*model.CloudResponse)
	return fileMetaDataResp.Data, nil
}

func GetCloudDriveMetadata(src string, w http.ResponseWriter, r *http.Request) (*model.CloudResponseData, error) {
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)

	param := &model.ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	cloudStorage := &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := cloudStorage.GetFileMetaData(param)
	if err != nil {
		klog.Errorf("Cloud Drive get_file_meta_data error: %v", err)
		return nil, err
	}

	fileMetaDataResp := res.(*model.CloudResponse)
	return fileMetaDataResp.Data, nil
}

func GetCloudDriveFocusedMetaInfos(task *pool.Task, src string, w http.ResponseWriter, r *http.Request) (info *CloudDriveFocusedMetaInfos, err error) {
	info = nil
	err = nil

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)

	param := &model.ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	cloudStorage := &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := cloudStorage.GetFileMetaData(param)
	if err != nil {
		TaskLog(task, "error", fmt.Sprintf("Cloud Drive get_file_meta_data error: %v", err))
		return nil, err
	}

	fileMetaDataResp := res.(*model.CloudResponse)

	if fileMetaDataResp.StatusCode == "FAIL" {
		err = e.New(*fileMetaDataResp.FailReason)
		TaskLog(task, "error", "API call failed:", err)
		return
	}

	info = &CloudDriveFocusedMetaInfos{
		Key:      fileMetaDataResp.Data.Meta.Key,
		Path:     fileMetaDataResp.Data.Path,
		Name:     fileMetaDataResp.Data.Name,
		FileSize: fileMetaDataResp.Data.FileSize,
		Size:     fileMetaDataResp.Data.FileSize,
		IsDir:    fileMetaDataResp.Data.IsDir,
	}
	return
}

func generateCloudDriveFilesData(storage *storage.CloudStorage, srcType string, fileResult *model.CloudListResponse, stopChan <-chan struct{}, dataChan chan<- string,
	param *model.ListParam) {
	defer close(dataChan)

	var A []*model.CloudResponseData
	fileResult.Lock()
	A = append(A, fileResult.Data...)
	fileResult.Unlock()

	for len(A) > 0 {
		klog.Infoln("len(A): ", len(A))
		firstItem := A[0]
		klog.Infoln("firstItem Path: ", firstItem.Path)
		klog.Infoln("firstItem Name:", firstItem.Name)
		firstItemPath := CloudDriveNormalizationPath(firstItem.Path, srcType, true, true)

		if firstItem.IsDir {
			firstParam := &model.ListParam{
				Path:  firstItemPath,
				Drive: param.Drive,
				Name:  param.Name,
			}

			res, err := storage.List(firstParam)
			if err != nil {
				klog.Error(err)
				return
			}
			files := res.(*model.CloudListResponse)

			A = append(files.Data, A[1:]...)
		} else {
			dataChan <- formatSSEvent(firstItem)

			A = A[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func streamCloudDriveFiles(storage *storage.CloudStorage, srcType string, filesResult *model.CloudListResponse, param *model.ListParam) {
	var w = storage.ResponseWriter
	var r = storage.Request
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateCloudDriveFilesData(storage, srcType, filesResult, stopChan, dataChan, param)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case event, ok := <-dataChan:
			if !ok {
				return
			}
			_, err := w.Write([]byte(event))
			if err != nil {
				klog.Error(err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			close(stopChan)
			return
		}
	}
}

func CopyCloudDriveSingleFile(task *pool.Task, src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	TaskLog(task, "info", "srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		TaskLog(task, "info", "Src parse failed.")
		return nil
	}
	dstDrive, dstName, dstPath := ParseCloudDrivePath(dst)
	TaskLog(task, "info", "dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	dstDir, dstFilename := path.Split(dstPath)
	if dstDir == "" || dstFilename == "" {
		TaskLog(task, "info", "Dst parse failed.")
		return nil
	}
	trimmedDstDir := CloudDriveNormalizationPath(dstDir, srcDrive, false, false)
	if trimmedDstDir == "" {
		trimmedDstDir = "/"
	}
	cloudStorage := &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.CopyFileParam{
		CloudFilePath:     srcPath,       // id of "path/to/cloud/file.txt",
		NewCloudDirectory: trimmedDstDir, // id of "new/cloud/directory",
		NewCloudFileName:  dstFilename,   // "new_file_name.txt",
		Drive:             dstDrive,      // "my_drive",
		Name:              dstName,       // "file_name",
	}

	_, err := cloudStorage.CopyFile(param)
	if err != nil {
		TaskLog(task, "error", fmt.Sprintf("Cloud Drive copy_file error: %v", err))
		return err
	}
	return nil
}

func CopyCloudDriveFolder(task *pool.Task, src, dst string, w http.ResponseWriter, r *http.Request, srcPath, srcPathName string) error {
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	TaskLog(task, "info", "srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		TaskLog(task, "info", "Src parse failed.")
		return nil
	}
	srcPath = CloudDriveNormalizationPath(srcPath, srcDrive, true, true)

	dstDrive, dstName, dstPath := ParseCloudDrivePath(dst)
	TaskLog(task, "info", "dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	dstDir, dstFilename := path.Split(strings.TrimSuffix(dstPath, "/"))
	if dstDir == "" || dstFilename == "" {
		TaskLog(task, "info", "Dst parse failed.")
		return nil
	}

	cloudStorage := &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.CopyFileParam{
		CloudFilePath:     srcPath,                              // id of "path/to/cloud/file.txt",
		NewCloudDirectory: dstDir,                               // id of "new/cloud/directory",
		NewCloudFileName:  strings.TrimSuffix(dstFilename, "/"), // "new_file_name.txt",
		Drive:             dstDrive,                             // "my_drive",
		Name:              dstName,                              // "file_name",
	}

	_, err := cloudStorage.CopyFile(param)
	if err != nil {
		TaskLog(task, "error", fmt.Sprintf("Cloud Drive copy_file error: %v", err))
		return err
	}

	return nil
}

func CloudPauseTask(taskId string, w http.ResponseWriter, r *http.Request) error {
	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := storage.PauseTask(taskId)
	if err != nil {
		klog.Errorln("Error calling patch/task/pause:", err)
		return err
	}

	taskResp := res.(*model.TaskResponse)

	if taskResp.StatusCode == "SUCCESS" {
		return e.New("Task paused successfully")
	}
	klog.Errorln("Failed to pause task")
	return nil
}

func CloudDriveFileToBuffer(task *pool.Task, src, bufferFilePath string, w http.ResponseWriter, r *http.Request, left, right int) error {
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	klog.Infoln("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		klog.Infoln("Src parse failed.")
		return nil
	}

	param := &model.DownloadAsyncParam{
		LocalFolder:   bufferFilePath,
		CloudFilePath: srcPath,
		Drive:         srcDrive,
		Name:          srcName,
	}
	if srcDrive == SrcTypeAWSS3 {
		param.LocalFileName = path.Base(srcPath)
	}

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	res, err := storage.DownloadAsync(param)
	if err != nil {
		klog.Errorf("Cloud Drive download_async error: %v", err)
		return err
	}

	var result = res.(*model.TaskResponse)
	// todo check SUCCESS

	taskId := result.Data.ID

	taskParam := &model.QueryTaskParam{
		TaskIds: []string{taskId},
	}

	// klog.Infoln("Task Params:", string(taskParam))

	if task == nil {
		for {
			time.Sleep(1000 * time.Millisecond)
			res, err := storage.QueryTask(taskParam)
			if err != nil {
				klog.Errorf("Cloud Drive download_async error: %v", err)
				return err
			}
			result := res.(*model.TaskQueryResponse)

			if len(result.Data) == 0 {
				return e.New("Task Info Not Found")
			}
			if srcDrive == SrcTypeTencent && result.Data[0].FailedReason != "" && result.Data[0].FailedReason == "Invalid task" {
				return nil
			}
			if result.Data[0].Status != "Waiting" && result.Data[0].Status != "InProgress" {
				if result.Data[0].Status == "Completed" {
					return nil
				}
				return e.New(result.Data[0].Status)
			}
		}
	}

	for {
		select {
		case <-task.Ctx.Done():
			err = CloudPauseTask(taskId, w, r)
			if err != nil {
				return err
			}
		default:
			time.Sleep(1000 * time.Millisecond)
			res, err := storage.QueryTask(taskParam)
			if err != nil {
				klog.Errorf("Cloud Drive download_async error: %v", err)
				return err
			}
			result := res.(*model.TaskQueryResponse)
			if len(result.Data) == 0 {
				return e.New("Task Info Not Found")
			}
			if srcDrive == SrcTypeTencent && result.Data[0].FailedReason != "" && result.Data[0].FailedReason == "Invalid task" {
				return nil
			}
			if result.Data[0].Status != "Waiting" && result.Data[0].Status != "InProgress" {
				if result.Data[0].Status == "Completed" {
					task.Mu.Lock()
					task.Progress = right
					return nil
				}
				return e.New(result.Data[0].Status)
			} else if result.Data[0].Status == "InProgress" {
				task.Mu.Lock()
				task.Progress = MapProgress(result.Data[0].Progress, left, right)
				task.Mu.Unlock()
			}
		}
	}
}

func CloudDriveBufferToFile(task *pool.Task, bufferFilePath, dst string, w http.ResponseWriter, r *http.Request, left, right int) (int, error) {
	dstDrive, dstName, dstPath := ParseCloudDrivePath(dst)
	klog.Infoln("dstDrive:", dstDrive, "dstName:", dstName, "dstPath:", dstPath)
	if dstPath == "" {
		klog.Infoln("Dst parse failed.")
		return http.StatusBadRequest, nil
	}
	dstDir, dstFileName := filepath.Split(strings.TrimSuffix(dstPath, "/"))

	trimmedDstDir := CloudDriveNormalizationPath(dstDir, dstDrive, false, false)
	if trimmedDstDir == "" {
		trimmedDstDir = "/"
	}

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.UploadAsyncParam{
		ParentPath:    trimmedDstDir,
		LocalFilePath: bufferFilePath,
		NewFileName:   dstFileName,
		Drive:         dstDrive,
		Name:          dstName,
	}

	res, err := storage.UploadAsync(param)
	if err != nil {
		klog.Errorln("Error calling drive/upload_async:", err)
		return common.ErrToStatus(err), err
	}

	result := res.(*model.TaskResponse)
	taskId := result.Data.ID
	taskParam := &model.QueryTaskParam{
		TaskIds: []string{taskId},
	}

	for {
		select {
		case <-task.Ctx.Done():
			err = CloudPauseTask(taskId, w, r)
			if err != nil {
				return common.ErrToStatus(err), err
			}
		default:
			time.Sleep(500 * time.Millisecond)
			res, err := storage.QueryTask(taskParam)
			if err != nil {
				klog.Errorln("Error calling drive/upload_async:", err)
				return common.ErrToStatus(err), err
			}
			result := res.(*model.TaskQueryResponse)
			if len(result.Data) == 0 {
				err = e.New("Task Info Not Found")
				return common.ErrToStatus(err), err
			}
			if dstDrive == SrcTypeTencent && result.Data[0].FailedReason != "" && result.Data[0].FailedReason == "Invalid task" {
				return http.StatusOK, nil
			}
			if result.Data[0].Status != "Waiting" && result.Data[0].Status != "InProgress" {
				if result.Data[0].Status == "Completed" {
					task.Mu.Lock()
					task.Progress = right
					return http.StatusOK, nil
				}
				err = e.New(result.Data[0].Status)
				return common.ErrToStatus(err), err
			} else if result.Data[0].Status == "InProgress" {
				if task != nil {
					task.Mu.Lock()
					task.Progress = MapProgress(result.Data[0].Progress, left, right)
					task.Mu.Unlock()
				}
			}
		}
	}
}

func MoveCloudDriveFolderOrFiles(task *pool.Task, src, dst string, w http.ResponseWriter, r *http.Request) error {
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	_, _, dstPath := ParseCloudDrivePath(dst)

	dstDir, _ := filepath.Split(dstPath)

	trimmedDstDir := CloudDriveNormalizationPath(dstDir, srcDrive, false, false)
	if trimmedDstDir == "" {
		trimmedDstDir = "/"
	}

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.MoveFileParam{
		CloudFilePath:     srcPath,
		NewCloudDirectory: trimmedDstDir,
		Drive:             srcDrive, // "my_drive",
		Name:              srcName,  // "file_name",
	}

	_, err := storage.MoveFile(param)
	if err != nil {
		TaskLog(task, "error", "Error calling drive/move_file:", err)
		return err
	}

	return nil
}

func ParseCloudDrivePath(src string) (drive, name, path string) {
	if strings.HasPrefix(src, "/Drive/") || strings.HasPrefix(src, "/drive/") {
		src = src[7:]
	}
	parts := strings.SplitN(src, "/", 2)
	drive = parts[0]

	trimSuffix := true
	if drive == SrcTypeAWSS3 {
		trimSuffix = false
	}

	src = "/"
	if len(parts) > 1 {
		src += parts[1]
	}

	slashes := []int{}
	for i, char := range src {
		if char == '/' {
			slashes = append(slashes, i)
		}
	}

	if len(slashes) < 2 {
		klog.Infoln("Path does not contain enough slashes.")
		return drive, "", ""
	}

	name = src[1:slashes[1]]
	path = src[slashes[1]:]
	if trimSuffix && path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	return drive, name, path
}

type CloudDriveResourceService struct {
	BaseResourceService
}

func (rc *CloudDriveResourceService) GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	streamStr := r.URL.Query().Get("stream")
	stream := 0
	var err error
	if streamStr != "" {
		stream, err = strconv.Atoi(streamStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	metaStr := r.URL.Query().Get("meta")
	meta := 0
	if metaStr != "" {
		meta, err = strconv.Atoi(metaStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	src := r.URL.Path
	klog.Infoln("src Path:", src)

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	klog.Infoln("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)

	var cloudStorage = &storage.CloudStorage{
		Owner:   r.Header.Get("X-Bfl-User"),
		Request: r,
	}

	var param = &model.ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}
	klog.Infof("Cloud Drive List Params: %+v, stream: %d, meta: %d", param, stream, meta)
	if stream == 1 {
		var res any
		res, err = cloudStorage.List(param)
		listResult := res.(*model.CloudListResponse)
		streamCloudDriveFiles(cloudStorage, srcDrive, listResult, param)
		return common.RenderSuccess(w, r)
	}
	if meta == 1 {
		res, err := cloudStorage.GetFileMetaData(param)
		if err != nil {
			klog.Errorf("GetFileMetaData error: %v", err)
			return common.ErrToStatus(err), err
		}

		var fileMetadata = res.(*model.CloudResponse)
		return common.RenderJSON(w, r, fileMetadata)
	}

	res, err := cloudStorage.List(param)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return common.ErrToStatus(err), err
	}
	listRes := res.(*model.CloudListResponse)
	return common.RenderJSON(w, r, listRes)
}

func (rc *CloudDriveResourceService) DeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		_, status, err := ResourceDeleteCloudDrive(fileCache, r.URL.Path, w, r, true)
		return status, err
	}
}

func (rc *CloudDriveResourceService) PostHandler(fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		_, status, err := ResourcePostCloudDriveFileParam(fileParam, w, r, true)
		return status, err
	}
}

func (rc *CloudDriveResourceService) PutHandler(fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		// not public api for cloud drive, so it is not implemented
		return http.StatusNotImplemented, fmt.Errorf("cloud drive does not supoort editing files")
	}
}

func (rc *CloudDriveResourceService) PatchHandler(fileCache fileutils.FileCache, fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		return ResourcePatchCloudDrive(fileCache, fileParam, w, r)
	}
}

func (rc *CloudDriveResourceService) BatchDeleteHandler(fileCache fileutils.FileCache, fileParams []*models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		failDirents := []string{}
		for _, fileParam := range fileParams {
			_, status, err := ResourceDeleteCloudDriveFileParam(fileCache, fileParam, w, r, true)
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

func (rs *CloudDriveResourceService) RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	src := r.URL.Path
	metaData, err := GetCloudDriveMetadata(src, w, r)
	if err != nil {
		klog.Error(err)
		return common.ErrToStatus(err), err
	}
	if metaData.IsDir {
		return http.StatusNotImplemented, fmt.Errorf("doesn't support directory download for cloud drive now")
	}
	return RawFileHandlerCloudDrive(src, w, r, metaData, bflName)
}

func (rs *CloudDriveResourceService) PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		vars := mux.Vars(r)

		previewSize, err := preview.ParsePreviewSize(vars["size"])
		if err != nil {
			return http.StatusBadRequest, err
		}
		path := "/" + vars["path"]

		return PreviewGetCloudDrive(w, r, previewSize, path, imgSvc, fileCache, enableThumbnails, resizePreview)
	}
}

func (rc *CloudDriveResourceService) PasteSame(task *pool.Task, action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is canceled when main exits
	go SimulateProgress(ctx, 0, 99, task.TotalFileSize, 25000000, task)

	var err error
	switch action {
	case "copy":
		var metaInfo *CloudDriveFocusedMetaInfos
		metaInfo, err = GetCloudDriveFocusedMetaInfos(task, src, w, r)
		if err != nil {
			TaskLog(task, "info", fmt.Sprintf("%s from %s to %s failed when getting meta info", action, src, dst))
			TaskLog(task, "error", err.Error())
			pool.FailTask(task.ID)
			return err
		}

		if metaInfo.IsDir {
			err = CopyCloudDriveFolder(task, src, dst, w, r, metaInfo.Path, metaInfo.Name)
		}
		err = CopyCloudDriveSingleFile(task, src, dst, w, r)
	case "rename":
		err = MoveCloudDriveFolderOrFiles(task, src, dst, w, r)
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

func (rs *CloudDriveResourceService) PasteDirFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
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

	err = handler.PasteDirTo(task, fs, src, dst, mode, fileCount, w, r, d, driveIdCache)
	if err != nil {
		return err
	}

	var fdstBase string = dst
	if driveIdCache[src] != "" {
		fdstBase = filepath.Join(filepath.Dir(filepath.Dir(strings.TrimSuffix(dst, "/"))), driveIdCache[src])
	}

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	srcPath = CloudDriveNormalizationPath(srcPath, srcDrive, true, true)

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.ListParam{
		Path:  srcPath,
		Drive: srcDrive,
		Name:  srcName,
	}

	res, err := storage.List(param)
	if err != nil {
		klog.Errorln("Error calling drive/ls:", err)
		return err
	}

	result := res.(*model.CloudListResponse)

	for _, item := range result.Data {
		fsrc := filepath.Join(src, item.Name)
		fdst := filepath.Join(fdstBase, item.Name)
		klog.Infoln(fsrc, fdst)
		if item.IsDir {
			fsrc = CloudDriveNormalizationPath(fsrc, srcType, true, true)
			fdst = CloudDriveNormalizationPath(fdst, dstType, true, true)
			err = rs.PasteDirFrom(task, fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), fileCount, w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			err = rs.PasteFileFrom(task, fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), item.FileSize, fileCount, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *CloudDriveResourceService) PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string, fileMode os.FileMode, fileCount int64, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	_, _, err := ResourcePostCloudDrive(dst, w, r, false)
	if err != nil {
		return err
	}
	return nil
}

func (rs *CloudDriveResourceService) PasteFileFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
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

	srcInfo, err := GetCloudDriveFocusedMetaInfos(nil, src, w, r)
	bufferFilePath, err := GenerateBufferFolder(srcInfo.Path, bflName)
	if err != nil {
		return err
	}
	bufferPath = filepath.Join(bufferFilePath, srcInfo.Name)
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

	err = CloudDriveFileToBuffer(task, src, bufferFilePath, w, r, left, mid)
	if err != nil {
		return err
	}

	if task.Status == "running" {
		handler, err := GetResourceService(dstType)
		if err != nil {
			return err
		}

		err = handler.PasteFileTo(task, fs, bufferPath, dst, mode, mid, right, w, r, d, diskSize)
		if err != nil {
			return err
		}
	}

	logMsg := fmt.Sprintf("Copy from %s to %s sucessfully!", src, dst)
	TaskLog(task, "info", logMsg)
	return nil
}

func (rs *CloudDriveResourceService) PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string, fileMode os.FileMode,
	left, right int, w http.ResponseWriter, r *http.Request, d *common.Data, diskSize int64) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	klog.Infoln("Begin to paste!")
	klog.Infoln("dst: ", dst)
	status, err := CloudDriveBufferToFile(task, bufferPath, dst, w, r, left, right)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	task.Transferred += diskSize
	return nil
}

func (rs *CloudDriveResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}

	metaInfo, err := GetCloudDriveFocusedMetaInfos(nil, src, w, r)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return nil, metaInfo.Size, 0755, metaInfo.IsDir, nil
}

func (rs *CloudDriveResourceService) MoveDelete(task *pool.Task, fileCache fileutils.FileCache, src string, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	_, status, err := ResourceDeleteCloudDrive(fileCache, src, w, r, true)
	if status != http.StatusOK && status != 0 {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *CloudDriveResourceService) GeneratePathList(db *gorm.DB, rootPath string, processor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error {
	if rootPath == "" {
		rootPath = "/"
	}

	processedPaths := make(map[string]bool)

	for bflName, cookie := range common.BflCookieCache {
		klog.Infof("Key: %s, Value: %s\n", bflName, cookie)

		var tempRequest = &http.Request{
			Header: make(http.Header),
		}
		tempRequest.Header.Set("Content-Type", "application/json")
		tempRequest.Header.Set("X-Bfl-User", bflName)
		tempRequest.Header.Set("Cookie", cookie)

		var storage = &storage.CloudStorage{
			Owner:   "bflName",
			Request: tempRequest,
		}

		// /drive/accounts logic is as same as google drive, but a little different from cloud drives. It is used only once, so call GoogleDriveCall here.
		res, err := storage.QueryAccount()
		if err != nil {
			klog.Errorf("GoogleDriveCall failed: %v\n", err)
			return err
		}

		result := res.(*model.AccountResponse)

		for _, datum := range result.Data {
			klog.Infof("datum=%v", datum)

			if datum.Type == SrcTypeGoogle {
				continue
			}

			rootParam := &model.ListParam{
				Path:  rootPath,
				Drive: datum.Type,
				Name:  datum.Name,
			}

			res, err := storage.List(rootParam)
			if err != nil {
				klog.Errorf("fetch repo response failed: %v\n", err)
				return err
			}
			dirent := res.(*model.CloudListResponse)

			generator := walkCloudDriveDirentsGenerator(dirent, storage, datum)

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

	err := recordsStatusProcessor(db, processedPaths, []string{SrcTypeCloud, SrcTypeDropbox, SrcTypeAWSS3, SrcTypeTencent}, 1)
	if err != nil {
		klog.Errorf("records status processor failed: %v\n", err)
		return err
	}

	return nil
}

// just for complement, no need to use now
func (rs *CloudDriveResourceService) parsePathToURI(path string) (string, string) {
	return SrcTypeCloud, path
}

func (rs *CloudDriveResourceService) GetFileCount(fs afero.Fs, src, countType string, w http.ResponseWriter, r *http.Request) (int64, error) {
	var count int64

	metaInfo, err := GetCloudDriveFocusedMetaInfos(nil, src, w, r)
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

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	srcDrive, srcName, pathId := ParseCloudDrivePath(src)
	queue := []string{pathId}

	for len(queue) > 0 {
		currentPath := queue[0]
		queue = queue[1:]

		param := &model.ListParam{
			Path:  currentPath,
			Drive: srcDrive,
			Name:  srcName,
		}

		res, err := storage.List(param)
		if err != nil {
			return 0, err
		}

		listResult := res.(*model.CloudListResponse)

		for _, item := range listResult.Data {
			if item.IsDir {
				normalizedPath := CloudDriveNormalizationPath(item.Path, "cloud", true, true)
				queue = append(queue, normalizedPath)
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

func (rs *CloudDriveResourceService) GetTaskFileInfo(fs afero.Fs, src string, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error) {
	metaInfo, err := GetCloudDriveFocusedMetaInfos(nil, src, w, r)
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

func ResourceDeleteCloudDriveFileParam(fileCache fileutils.FileCache, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request, returnResp bool) (*model.CloudResponse, int, error) {
	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.DeleteParam{
		Path:  fileParam.Path,
		Drive: fileParam.FileType, // "my_drive",
		Name:  fileParam.Extend,   // "file_name",
	}

	// del thumbnails for Cloud Drive
	err := delThumbsCloudDriveFileParam(r.Context(), fileCache, fileParam, w, r)
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	res, err := storage.Delete(param)
	if err != nil {
		klog.Errorln("Error calling drive/delete:", err)
		return nil, common.ErrToStatus(err), err
	}
	result := res.(*model.CloudResponse)
	return result, 0, nil
}

func ResourceDeleteCloudDrive(fileCache fileutils.FileCache, src string, w http.ResponseWriter, r *http.Request, returnResp bool) (*model.CloudResponse, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	klog.Infoln("src Path:", src)

	var err error
	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.DeleteParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// del thumbnails for Cloud Drive
	err = delThumbsCloudDrive(r.Context(), fileCache, src, w, r)
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	res, err := storage.Delete(param)
	if err != nil {
		klog.Errorln("Error calling drive/delete:", err)
		return nil, common.ErrToStatus(err), err
	}
	result := res.(*model.CloudResponse)
	return result, 0, nil
}

func ResourcePostCloudDriveFileParam(fileParam *models.FileParam, w http.ResponseWriter, r *http.Request, returnResp bool) (*model.CloudResponse, int, error) {
	path, newName := path.Split(fileParam.Path)

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.PostParam{
		ParentPath: path,
		FolderName: newName,
		Drive:      fileParam.FileType, // "my_drive",
		Name:       fileParam.Extend,   // "file_name",
	}

	res, err := storage.CreateFolder(param)
	if err != nil {
		klog.Errorln("Error calling drive/create_folder:", err)
		return nil, common.ErrToStatus(err), err
	}

	result := res.(*model.CloudResponse)
	return result, 0, nil
}

func ResourcePostCloudDrive(src string, w http.ResponseWriter, r *http.Request, returnResp bool) (*model.CloudResponse, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	klog.Infoln("src Path:", src)

	srcDrive, srcName, srcPath := ParseCloudDrivePath(src)
	klog.Infoln("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)
	srcPath = CloudDriveNormalizationPath(srcPath, srcDrive, true, false)
	path, newName := path.Split(srcPath)

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.PostParam{
		ParentPath: path,
		FolderName: newName,
		Drive:      srcDrive, // "my_drive",
		Name:       srcName,  // "file_name",
	}

	res, err := storage.CreateFolder(param)
	if err != nil {
		klog.Errorln("Error calling drive/create_folder:", err)
		return nil, common.ErrToStatus(err), err
	}

	result := res.(*model.CloudResponse)
	return result, 0, nil
}

func ResourcePatchCloudDrive(fileCache fileutils.FileCache, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (int, error) {
	dst := r.URL.Query().Get("destination")
	dstFilename, err := common.UnescapeURLIfEscaped(dst)
	if err != nil {
		return http.StatusBadRequest, err
	}

	var storage = &storage.CloudStorage{
		Owner:          r.Header.Get("X-Bfl-User"),
		ResponseWriter: w,
		Request:        r,
	}

	param := &model.PatchParam{
		Path:        fileParam.Path,
		NewFileName: dstFilename,
		Drive:       fileParam.FileType, // "my_drive",
		Name:        fileParam.Extend,   // "file_name",
	}

	// del thumbnails for Cloud Drive
	err = delThumbsCloudDriveFileParam(r.Context(), fileCache, fileParam, w, r)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	if _, err = storage.Rename(param); err != nil {
		klog.Errorln("Error calling drive/rename:", err)
		return common.ErrToStatus(err), err
	}

	return common.RenderSuccess(w, r)
}

func setContentDispositionCloudDrive(w http.ResponseWriter, r *http.Request, fileName string) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(fileName))
	}
}

func ParseTimeString(s *string) time.Time {
	if s == nil || *s == "" {
		return time.Unix(0, 0)
	}
	parsed, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return time.Unix(0, 0)
	}
	return parsed
}

func previewCacheKeyCloudDrive(f *model.CloudResponseData, previewSize preview.PreviewSize) string {
	return fmt.Sprintf("%x%x%x", f.Path, ParseTimeString(f.Modified).Unix(), previewSize)
}

func createPreviewCloudDrive(w http.ResponseWriter, r *http.Request, src string, imgSvc preview.ImgService, fileCache fileutils.FileCache,
	file *model.CloudResponseData, previewSize preview.PreviewSize, bflName string) ([]byte, error) {
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
	bufferPath := filepath.Join(bufferFilePath, file.Name)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(nil, bufferPath, SrcTypeCloud)
	}()

	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return nil, err
	}
	err = CloudDriveFileToBuffer(nil, src, bufferFilePath, w, r, 0, 0)
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
		cacheKey := previewCacheKeyCloudDrive(file, previewSize)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			klog.Errorf("failed to cache resized image: %v", err)
		}
	}()

	return buf.Bytes(), nil
}

func RawFileHandlerCloudDrive(src string, w http.ResponseWriter, r *http.Request, file *model.CloudResponseData, bflName string) (int, error) {
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
	bufferPath := filepath.Join(bufferFilePath, file.Name)
	klog.Infoln("Buffer file path: ", bufferFilePath)
	klog.Infoln("Buffer path: ", bufferPath)

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(nil, bufferPath, SrcTypeCloud)
	}()

	err = MakeDiskBuffer(bufferPath, diskSize, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	err = CloudDriveFileToBuffer(nil, src, bufferFilePath, w, r, 0, 0)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	fd, err := os.Open(bufferPath)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	defer fd.Close()

	setContentDispositionCloudDrive(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, ParseTimeString(file.Modified), fd)

	return 0, nil
}

func handleImagePreviewCloudDrive(
	w http.ResponseWriter,
	r *http.Request,
	src string,
	imgSvc preview.ImgService,
	fileCache fileutils.FileCache,
	file *model.CloudResponseData,
	previewSize preview.PreviewSize,
	enableThumbnails, resizePreview bool,
) (int, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return common.ErrToStatus(os.ErrPermission), os.ErrPermission
	}

	if (previewSize == preview.PreviewSizeBig && !resizePreview) ||
		(previewSize == preview.PreviewSizeThumb && !enableThumbnails) {
		return RawFileHandlerCloudDrive(src, w, r, file, bflName) // + preview
	}

	format, err := imgSvc.FormatFromExtension(path.Ext(file.Name))
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return RawFileHandlerCloudDrive(src, w, r, file, bflName) // + preview
	}
	if err != nil {
		return common.ErrToStatus(err), err
	}

	cacheKey := previewCacheKeyCloudDrive(file, previewSize)
	klog.Infoln("cacheKey:", cacheKey)
	klog.Infoln("f.RealPath:", file.Path)
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	if !ok {
		resizedImage, err = createPreviewCloudDrive(w, r, src, imgSvc, fileCache, file, previewSize, bflName) // + preview
		if err != nil {
			return common.ErrToStatus(err), err
		}
	}

	err = redisutils.UpdateFileAccessTimeToRedis(redisutils.GetFileName(cacheKey))
	if err != nil {
		return common.ErrToStatus(err), err
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, ParseTimeString(file.Modified), bytes.NewReader(resizedImage))

	return 0, nil
}

func PreviewGetCloudDrive(w http.ResponseWriter, r *http.Request, previewSize preview.PreviewSize, path string,
	imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) (int, error) {
	src := path

	metaData, err := GetCloudDriveMetadata(src, w, r)
	if err != nil {
		klog.Error(err)
		return common.ErrToStatus(err), err
	}

	setContentDispositionCloudDrive(w, r, metaData.Name) // + preview

	fileType := parser.MimeTypeByExtension(metaData.Name)
	if strings.HasPrefix(fileType, "image") {
		return handleImagePreviewCloudDrive(w, r, src, imgSvc, fileCache, metaData, previewSize, enableThumbnails, resizePreview) // + preview
	} else {
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", fileType)
	}
}

func delThumbsCloudDriveFileParam(ctx context.Context, fileCache fileutils.FileCache, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) error {
	metaData, err := GetCloudDriveMetadataFileParam(fileParam, w, r)
	if err != nil {
		klog.Errorln("Error calling drive/get_file_meta_data:", err)
		return err
	}

	for _, previewSizeName := range preview.PreviewSizeNames() {
		size, _ := preview.ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKeyCloudDrive(metaData, size)
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

func delThumbsCloudDrive(ctx context.Context, fileCache fileutils.FileCache, src string, w http.ResponseWriter, r *http.Request) error {
	metaData, err := GetCloudDriveMetadata(src, w, r)
	if err != nil {
		klog.Errorln("Error calling drive/get_file_meta_data:", err)
		return err
	}

	for _, previewSizeName := range preview.PreviewSizeNames() {
		size, _ := preview.ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKeyCloudDrive(metaData, size)
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

func walkCloudDriveDirentsGenerator(files *model.CloudListResponse, storage *storage.CloudStorage, datum *model.AccounResponseItem) <-chan DirentGeneratedEntry {
	ch := make(chan DirentGeneratedEntry)
	go func() {
		defer close(ch)

		queue := make([]*model.CloudResponseData, 0)
		files.Lock()
		queue = append(queue, files.Data...)
		files.Unlock()

		for len(queue) > 0 {
			firstItem := queue[0]
			queue = queue[1:]

			if firstItem.IsDir {
				fullPath := filepath.Join(datum.Type, datum.Name, firstItem.Path) + "/"
				var parsedTime time.Time
				var err error
				if firstItem.Modified != nil && *firstItem.Modified != "" {
					parsedTime, err = time.Parse(time.RFC3339, *firstItem.Modified)
					if err != nil {
						klog.Errorln("Parse time failed:", err)
						continue
					}
					klog.Infoln("Parsed Time:", parsedTime)
				} else {
					klog.Warningln("Modified is empty")
					parsedTime = time.Now()
				}
				entry := DirentGeneratedEntry{
					Drive: datum.Type,
					Path:  fullPath,
					Mtime: parsedTime,
				}
				ch <- entry

				paramPath := strings.TrimSuffix(firstItem.Path, "/") + "/"
				firstParam := &model.ListParam{
					Path:  paramPath,
					Drive: datum.Type,
					Name:  datum.Name,
				}

				res, err := storage.List(firstParam)
				if err != nil {
					klog.Error(err)
					continue
				}

				var filesResult = res.(*model.CloudListResponse)
				queue = append(queue, filesResult.Data...)
			}
		}
	}()
	return ch
}
