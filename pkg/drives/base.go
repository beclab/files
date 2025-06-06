package drives

import (
	"bytes"
	"context"
	"files/pkg/common"
	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/pool"
	"files/pkg/preview"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
type PathProcessor func(*gorm.DB, string, string, time.Time) (int, error)
type RecordsStatusProcessor func(db *gorm.DB, processedPaths map[string]bool, srcTypes []string, status int) error

type ResourceService interface {
	// resource handlers
	GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	DeleteHandler(fileCache fileutils.FileCache) handleFunc
	PostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	PutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	PatchHandler(fileCache fileutils.FileCache) handleFunc
	BatchDeleteHandler(fileCache fileutils.FileCache, dirents []string) handleFunc

	// raw handler
	RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)

	// preview handler
	PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc

	// paste funcs
	PasteSame(task *pool.Task, action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error
	PasteDirFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, fileMode os.FileMode,
		fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error
	PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string, fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request,
		d *common.Data, driveIdCache map[string]string) error
	PasteFileFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, mode os.FileMode, diskSize int64,
		fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error
	PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, left, right int, w http.ResponseWriter, r *http.Request,
		d *common.Data, diskSize int64) error
	GetStat(fs afero.Fs, src string, w http.ResponseWriter, r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error)
	MoveDelete(task *pool.Task, fileCache fileutils.FileCache, src string, d *common.Data, w http.ResponseWriter, r *http.Request) error
	GetFileCount(fs afero.Fs, src, countType string, w http.ResponseWriter, r *http.Request) (int64, error)
	GetTaskFileInfo(fs afero.Fs, src string, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error)

	// path list funcs
	GeneratePathList(db *gorm.DB, rootPath string, pathProcessor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error
	parsePathToURI(path string) (string, string)
}

var (
	BaseService        = &BaseResourceService{}
	DriveService       = &DriveResourceService{}
	CacheService       = &CacheResourceService{}
	GoogleDriveService = &GoogleDriveResourceService{}
	SyncService        = &SyncResourceService{}
	CloudDriveService  = &CloudDriveResourceService{}
)

const (
	SrcTypeDrive    = "drive"
	SrcTypeData     = "data"
	SrcTypeExternal = "external"
	SrcTypeCache    = "cache"
	SrcTypeSync     = "sync"
	SrcTypeGoogle   = "google"
	SrcTypeCloud    = "cloud"
	SrcTypeAWSS3    = "awss3"
	SrcTypeTencent  = "tencent"
	SrcTypeDropbox  = "dropbox"
)

var ValidSrcTypes = map[string]bool{
	SrcTypeDrive:    true,
	SrcTypeData:     true,
	SrcTypeExternal: true,
	SrcTypeCache:    true,
	SrcTypeSync:     true,
	SrcTypeGoogle:   true,
	SrcTypeCloud:    true,
	SrcTypeAWSS3:    true,
	SrcTypeTencent:  true,
	SrcTypeDropbox:  true,
}

func GetResourceService(srcType string) (ResourceService, error) {
	switch srcType {
	case SrcTypeDrive, SrcTypeData, SrcTypeExternal:
		return DriveService, nil
	case SrcTypeCache:
		return CacheService, nil
	case SrcTypeSync:
		return SyncService, nil
	case SrcTypeGoogle:
		return GoogleDriveService, nil
	case SrcTypeCloud, SrcTypeAWSS3, SrcTypeTencent, SrcTypeDropbox:
		return CloudDriveService, nil
	default:
		return BaseService, nil
	}
}

func IsThridPartyDrives(dstType string) bool {
	switch dstType {
	case SrcTypeDrive, SrcTypeData, SrcTypeExternal, SrcTypeCache, SrcTypeSync:
		return false
	case SrcTypeGoogle, SrcTypeCloud, SrcTypeAWSS3, SrcTypeTencent, SrcTypeDropbox:
		return true
	default:
		return false
	}
}

func IsBaseDrives(dstType string) bool {
	switch dstType {
	case SrcTypeDrive, SrcTypeData, SrcTypeExternal, SrcTypeCache:
		return true
	default:
		return false
	}
}

func IsCloudDrives(dstType string) bool {
	switch dstType {
	case SrcTypeCloud, SrcTypeAWSS3, SrcTypeTencent, SrcTypeDropbox:
		return true
	default:
		return false
	}
}

func GetMountedData(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	var err error = nil
	if files.TerminusdHost != "" {
		// for 1.12: path-incluster URL exists, won't err in normal condition
		// for 1.11: path-incluster URL may not exist, if err, use usb-incluster and hdd-incluster for system functional
		url := "http://" + files.TerminusdHost + "/system/mounted-path-incluster"

		headers := make(http.Header)
		headers.Set("Content-Type", "application/json")
		headers.Set("X-Signature", "temp_signature")

		MountedData, err = files.FetchDiskInfo(url, headers)
		if err != nil {
			klog.Infof("Failed to fetch data from %s: %v", url, err)
			usbUrl := "http://" + files.TerminusdHost + "/system/mounted-usb-incluster"

			usbHeaders := headers.Clone()
			usbHeaders.Set("Content-Type", "application/json")
			usbHeaders.Set("X-Signature", "temp_signature")

			usbData, err := files.FetchDiskInfo(usbUrl, usbHeaders)
			if err != nil {
				klog.Infof("Failed to fetch data from %s: %v", usbUrl, err)
			}

			klog.Infoln("USB Data:", usbData)

			hddUrl := "http://" + files.TerminusdHost + "/system/mounted-hdd-incluster"

			hddHeaders := headers.Clone()
			hddHeaders.Set("Content-Type", "application/json")
			hddHeaders.Set("X-Signature", "temp_signature")

			hddData, err := files.FetchDiskInfo(hddUrl, hddHeaders)
			if err != nil {
				klog.Infof("Failed to fetch data from %s: %v", hddUrl, err)
			}

			klog.Infoln("HDD Data:", hddData)

			for _, item := range usbData {
				item.Type = "usb"
				MountedData = append(MountedData, item)
			}

			for _, item := range hddData {
				item.Type = "hdd"
				MountedData = append(MountedData, item)
			}
		}
		klog.Infoln("Mounted Data:", MountedData)
	}
	MountedTicker.Reset(5 * time.Minute)
	return
}

type BaseResourceService struct{}

func (rs *BaseResourceService) GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	xBflUser := r.Header.Get("X-Bfl-User")
	klog.Infoln("X-Bfl-User: ", xBflUser)

	streamStr := r.URL.Query().Get("stream")
	stream := 0
	var err error = nil
	if streamStr != "" {
		stream, err = strconv.Atoi(streamStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	var file *files.FileInfo
	if MountedData != nil {
		file, err = files.NewFileInfoWithDiskInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.Server.TypeDetectionByHeader,
			Content:    true,
		}, MountedData)
	} else {
		file, err = files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.Server.TypeDetectionByHeader,
			Content:    true,
		})
	}
	if err != nil {
		if common.ErrToStatus(err) == http.StatusNotFound && r.URL.Path == "/External/" {
			listing := &files.Listing{
				Items:         []*files.FileInfo{},
				NumDirs:       0,
				NumFiles:      0,
				NumTotalFiles: 0,
				Size:          0,
				FileSize:      0,
			}
			file = &files.FileInfo{
				Path:         "/External/",
				Name:         "External",
				Size:         0,
				FileSize:     0,
				Extension:    "",
				ModTime:      time.Now(),
				Mode:         os.FileMode(2147484141),
				IsDir:        true,
				IsSymlink:    false,
				Type:         "",
				ExternalType: "others",
				Subtitles:    []string{},
				Checksums:    make(map[string]string),
				Listing:      listing,
				Fs:           nil,
			}

			return common.RenderJSON(w, r, file)
		}
		return common.ErrToStatus(err), err
	}

	if file.IsDir {
		if files.CheckPath(file.Path, files.ExternalPrefix, "/") {
			files.GetExternalExtraInfos(file, MountedData, 1)
		}
		file.Listing.Sorting = files.DefaultSorting
		file.Listing.ApplySort()
		if stream == 1 {
			streamListingItems(w, r, file.Listing, d, MountedData)
			return 0, nil
		} else {
			return common.RenderJSON(w, r, file)
		}
	}

	if checksum := r.URL.Query().Get("checksum"); checksum != "" {
		err := file.Checksum(checksum)
		if err == errors.ErrInvalidOption {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}

		// do not waste bandwidth if we just want the checksum
		file.Content = ""
	}

	if file.Type == "video" {
		osSystemServer := "system-server.user-system-" + xBflUser

		httpposturl := fmt.Sprintf("http://%s/legacy/v1alpha1/api.intent/v1/server/intent/send", osSystemServer)

		klog.Infoln("HTTP JSON POST URL:", httpposturl)

		var jsonData = []byte(`{
			"action": "view",
			"category": "video",
			"data": {
				"name": "` + file.Name + `",
				"path": "` + file.Path + `",
				"extention": "` + file.Extension + `"
			}
		}`)
		request, error := http.NewRequest("POST", httpposturl, bytes.NewBuffer(jsonData))
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")

		client := &http.Client{}
		response, error := client.Do(request)
		if error != nil {
			panic(error)
		}
		defer response.Body.Close()

		klog.Infoln("response Status:", response.Status)
		klog.Infoln("response Headers:", response.Header)
		body, _ := ioutil.ReadAll(response.Body)
		klog.Infoln("response Body:", string(body))
	}
	return common.RenderJSON(w, r, file)
}

func (rs *BaseResourceService) DeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		if r.URL.Path == "/" {
			return http.StatusForbidden, nil
		}

		status, err := ResourceDriveDelete(fileCache, r.URL.Path, r.Context(), d)
		if status != http.StatusOK {
			return status, os.ErrInvalid
		}
		if err != nil {
			return common.ErrToStatus(err), err
		}
		return 0, nil
	}
}

func (rs *BaseResourceService) PostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	modeParam := r.URL.Query().Get("mode")

	mode, err := strconv.ParseUint(modeParam, 8, 32)
	if err != nil || modeParam == "" {
		mode = 0775
	}

	fileMode := os.FileMode(mode)

	// Directories creation on POST.
	if strings.HasSuffix(r.URL.Path, "/") {
		if err = fileutils.MkdirAllWithChown(files.DefaultFs, r.URL.Path, fileMode); err != nil {
			klog.Errorln(err)
			return common.ErrToStatus(err), err
		}
		return http.StatusOK, nil
	}
	return http.StatusBadRequest, fmt.Errorf("%s is not a valid directory path", r.URL.Path)
}

func (rs *BaseResourceService) PutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	// Only allow PUT for files.
	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	info, err := fileutils.WriteFile(files.DefaultFs, r.URL.Path, r.Body)
	etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
	w.Header().Set("ETag", etag)

	return common.ErrToStatus(err), err
}

func (rs *BaseResourceService) PatchHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		action := r.URL.Query().Get("action")
		dst, err := common.UnescapeURLIfEscaped(dst)

		if err != nil {
			return common.ErrToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		err = common.CheckParent(src, dst)
		if err != nil {
			return http.StatusBadRequest, err
		}

		rename := r.URL.Query().Get("rename") == "true"
		if !rename {
			if _, err = files.DefaultFs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		if rename {
			dst = common.AddVersionSuffix(dst, files.DefaultFs, strings.HasSuffix(src, "/"))
		}

		srcExternalType := files.GetExternalType(src, MountedData)
		dstExternalType := files.GetExternalType(dst, MountedData)

		klog.Infoln("Before patch action:", src, dst, action, rename)

		needTaskStr := r.URL.Query().Get("task")
		needTask := 0
		if needTaskStr != "" {
			needTask, err = strconv.Atoi(needTaskStr)
			if err != nil {
				klog.Errorln(err)
				needTask = 0
			}
		}

		var task *pool.Task = nil
		if needTask != 0 {
			// only for cache now
			handler, err := GetResourceService(SrcTypeCache)
			if err != nil {
				return http.StatusBadRequest, err
			}
			isDir, fileType, filename, err := handler.GetTaskFileInfo(files.DefaultFs, src, w, r)

			taskID := fmt.Sprintf("task%d", time.Now().UnixNano())
			task = pool.NewTask(taskID, src, dst, SrcTypeCache, SrcTypeCache, action, true, false, isDir, fileType, filename)
			pool.TaskManager.Store(taskID, task)
			pool.WorkerPool.Submit(func() {
				klog.Infof("Task %s started", taskID)
				defer klog.Infof("Task %s exited", taskID)

				if loadedTask, ok := pool.TaskManager.Load(taskID); ok {
					if concreteTask, ok := loadedTask.(*pool.Task); ok {
						concreteTask.Status = "running"
						concreteTask.Progress = 0

						executePatchTask(concreteTask, action, SrcTypeCache, SrcTypeCache, rename, d, fileCache, w, r)
					}
				}
			})
			return common.RenderJSON(w, r, map[string]string{"task_id": taskID})
		}

		err = common.PatchAction(nil, r.Context(), action, src, dst, srcExternalType, dstExternalType, fileCache)
		return common.ErrToStatus(err), err
	}
}

func (rs *BaseResourceService) BatchDeleteHandler(fileCache fileutils.FileCache, dirents []string) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		failDirents := []string{}
		for _, dirent := range dirents {
			if dirent == "/" {
				failDirents = append(failDirents, dirent)
				continue
			}

			status, err := ResourceDriveDelete(fileCache, dirent, r.Context(), d)
			if status != http.StatusOK || err != nil {
				failDirents = append(failDirents, dirent)
			}
		}

		if len(failDirents) > 0 {
			return http.StatusInternalServerError, fmt.Errorf("delete %s failed", strings.Join(failDirents, "; "))
		}
		return common.RenderJSON(w, r, map[string]interface{}{"msg": "all dirents deleted"})
	}
}

func (rs *BaseResourceService) RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       r.URL.Path,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})
	if err != nil {
		return common.ErrToStatus(err), err
	}

	if files.IsNamedPipe(file.Mode) {
		SetContentDisposition(w, r, file)
		return 0, nil
	}

	if !file.IsDir {
		return RawFileHandler(w, r, file)
	}

	return RawDirHandler(w, r, d, file)
}

func (rs *BaseResourceService) PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		vars := mux.Vars(r)

		previewSize, err := preview.ParsePreviewSize(vars["size"])
		if err != nil {
			return http.StatusBadRequest, err
		}
		path := "/" + vars["path"]

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.Server.TypeDetectionByHeader,
		})
		if err != nil {
			return common.ErrToStatus(err), err
		}

		SetContentDisposition(w, r, file)

		switch file.Type {
		case "image":
			return HandleImagePreview(w, r, imgSvc, fileCache, file, previewSize, enableThumbnails, resizePreview)
		default:
			return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", file.Type)
		}
	}
}

func (rs *BaseResourceService) PasteSame(task *pool.Task, action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteDirFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string, fileMode os.FileMode, fileCount int64, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteFileFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, left, right int, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	return nil, 0, 0, false, fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) MoveDelete(task *pool.Task, fileCache fileutils.FileCache, src string, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GetFileCount(fs afero.Fs, src, countType string, w http.ResponseWriter, r *http.Request) (int64, error) {
	return 0, fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GetTaskFileInfo(fs afero.Fs, src string, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error) {
	return false, "", "", fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GeneratePathList(db *gorm.DB, rootPath string, pathProcessor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) parsePathToURI(path string) (string, string) {
	return "error", ""
}

func executePatchTask(task *pool.Task, action, srcType, dstType string, rename bool,
	d *common.Data, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) {
	select {
	case <-task.Ctx.Done():
		return
	default:
	}

	// only for cache

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		err = common.PatchAction(task, task.Ctx, action, task.Source, task.Dest, "", "", fileCache)

		if common.ErrToStatus(err) == http.StatusRequestEntityTooLarge {
			fmt.Fprintln(w, err.Error())
		}
		if err != nil {
			klog.Errorln(err)
		}
		return
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		task.UpdateProgress()
	}()

	select {
	case err := <-task.ErrChan:
		if err != nil {
			task.LoggingError(fmt.Sprintf("%v", err))
			klog.Errorf("[ERROR]: %v", err)
			return
		}
	case <-time.After(5 * time.Second):
		fmt.Println("ExecuteRsyncWithContext took too long to start, proceeding assuming no initial error.")
	case <-task.Ctx.Done():
		return
	}

	if task.ProgressChan == nil {
		klog.Error("progressChan is nil")
		return
	}

	wg.Wait()
}
