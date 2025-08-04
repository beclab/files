package drives

import (
	"context"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/pool"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
type PathProcessor func(*gorm.DB, string, string, time.Time) (int, error)
type RecordsStatusProcessor func(db *gorm.DB, processedPaths map[string]bool, srcTypes []string, status int) error

type ResourceService interface {
	// resource handlers
	DeleteHandler(fileCache fileutils.FileCache) handleFunc // not used now
	PutHandler(fileParam *models.FileParam) handleFunc
	PatchHandler(fileCache fileutils.FileCache, fileParam *models.FileParam) handleFunc

	// paste funcs
	PasteSame(task *pool.Task, action, src, dst string, srcFileParam, dstFileParam *models.FileParam, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error
	PasteDirFrom(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
		dstFileParam *models.FileParam, dstType, dst string, d *common.Data, fileMode os.FileMode,
		fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error
	PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string, srcFileParam, dstFileParam *models.FileParam, fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request,
		d *common.Data, driveIdCache map[string]string) error
	PasteFileFrom(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
		dstFileParam *models.FileParam, dstType, dst string, d *common.Data, mode os.FileMode, diskSize int64,
		fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error
	PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string, srcFileParam, dstFileParam *models.FileParam, fileMode os.FileMode, left, right int, w http.ResponseWriter, r *http.Request,
		d *common.Data, diskSize int64) error
	GetStat(fs afero.Fs, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error)
	MoveDelete(task *pool.Task, fileCache fileutils.FileCache, fileParam *models.FileParam, d *common.Data, w http.ResponseWriter, r *http.Request) error
	GetFileCount(fs afero.Fs, fileParam *models.FileParam, countType string, w http.ResponseWriter, r *http.Request) (int64, error)
	GetTaskFileInfo(fs afero.Fs, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error)

	// path list funcs
	GeneratePathList(db *gorm.DB, rootPath string, pathProcessor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error // won't use
	parsePathToURI(path string) (string, string)                                                                                     // won't use
}

var (
	BaseService  = &BaseResourceService{}
	DriveService = &DriveResourceService{}
	CacheService = &CacheResourceService{}
	SyncService  = &SyncResourceService{}
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
	SrcTypeInternal = "internal"
	SrcTypeUsb      = "usb"
	SrcTypeSmb      = "smb"
	SrcTypeHdd      = "hdd"
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
	SrcTypeInternal: true,
	SrcTypeUsb:      true,
	SrcTypeSmb:      true,
	SrcTypeHdd:      true,
}

func GetResourceService(srcType string) (ResourceService, error) {
	switch srcType {
	case SrcTypeDrive, SrcTypeData, SrcTypeExternal, SrcTypeInternal, SrcTypeUsb, SrcTypeSmb, SrcTypeHdd:
		return DriveService, nil
	case SrcTypeCache:
		return CacheService, nil
	case SrcTypeSync:
		return SyncService, nil
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

func (rs *BaseResourceService) PutHandler(fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		// Only allow PUT for files.
		if strings.HasSuffix(fileParam.Path, "/") {
			return http.StatusMethodNotAllowed, nil
		}

		uri, err := fileParam.GetResourceUri()
		if err != nil {
			return http.StatusBadRequest, err
		}
		urlPath := strings.TrimPrefix(uri+fileParam.Path, "/data")

		exists, err := afero.Exists(files.DefaultFs, urlPath)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !exists {
			return http.StatusNotFound, nil
		}

		info, err := fileutils.WriteFile(files.DefaultFs, urlPath, r.Body)
		etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
		w.Header().Set("ETag", etag)

		return common.ErrToStatus(err), err
	}
}

func (rs *BaseResourceService) PatchHandler(fileCache fileutils.FileCache, fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		action := "rename" // only this for PATCH /api/resources

		uri, err := fileParam.GetResourceUri()
		if err != nil {
			return http.StatusBadRequest, err
		}

		src := strings.TrimPrefix(uri+fileParam.Path, "/data")
		dst := r.URL.Query().Get("destination") // only a name in PATCH /api/resources now
		dst, err = common.UnescapeURLIfEscaped(dst)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		source := r.URL.Path
		destination := filepath.Join(filepath.Dir(strings.TrimSuffix(source, "/")), dst)
		if strings.HasSuffix(source, "/") {
			destination += "/"
		}

		dst = filepath.Join(filepath.Dir(strings.TrimSuffix(src, "/")), dst)
		if strings.HasSuffix(src, "/") {
			dst += "/"
		}

		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		err = common.CheckParent(src, dst)
		if err != nil {
			return http.StatusBadRequest, err
		}

		dst = common.AddVersionSuffix(dst, files.DefaultFs, strings.HasSuffix(src, "/"))

		srcExternalType := files.GetExternalType(src, MountedData)
		dstExternalType := files.GetExternalType(dst, MountedData)

		klog.Infoln("Before patch action:", src, dst)

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
			isDir, fileType, filename, err := handler.GetTaskFileInfo(files.DefaultFs, fileParam, w, r)

			taskID := fmt.Sprintf("task%d", time.Now().UnixNano())
			task = pool.NewTask(taskID, source, destination, src, dst, SrcTypeCache, SrcTypeCache, action, true, false, isDir, fileType, filename)
			pool.TaskManager.Store(taskID, task)
			pool.WorkerPool.Submit(func() {
				klog.Infof("Task %s started", taskID)
				defer klog.Infof("Task %s exited", taskID)

				if loadedTask, ok := pool.TaskManager.Load(taskID); ok {
					if concreteTask, ok := loadedTask.(*pool.Task); ok {
						concreteTask.Status = "running"
						concreteTask.Progress = 0

						executePatchTask(concreteTask, action, SrcTypeCache, SrcTypeCache, d, fileCache, w, r)
					}
				}
			})
			return common.RenderJSON(w, r, map[string]string{"task_id": taskID})
		}

		err = common.PatchAction(nil, r.Context(), action, src, dst, srcExternalType, dstExternalType, fileCache)
		return common.ErrToStatus(err), err
	}
}

func (rs *BaseResourceService) PasteSame(task *pool.Task, action, src, dst string, srcFileParam, dstFileParam *models.FileParam, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteDirFrom(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
	dstFileParam *models.FileParam, dstType, dst string, d *common.Data,
	fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string,
	srcFileParam, dstFileParam *models.FileParam, fileMode os.FileMode, fileCount int64, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteFileFrom(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
	dstFileParam *models.FileParam, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string,
	srcFileParam, dstFileParam *models.FileParam, fileMode os.FileMode, left, right int, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GetStat(fs afero.Fs, fileParam *models.FileParam, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	return nil, 0, 0, false, fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) MoveDelete(task *pool.Task, fileCache fileutils.FileCache, fileParam *models.FileParam, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GetFileCount(fs afero.Fs, fileParam *models.FileParam, countType string, w http.ResponseWriter, r *http.Request) (int64, error) {
	return 0, fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GetTaskFileInfo(fs afero.Fs, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error) {
	return false, "", "", fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GeneratePathList(db *gorm.DB, rootPath string, pathProcessor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) parsePathToURI(path string) (string, string) {
	return "error", ""
}

func executePatchTask(task *pool.Task, action, srcType, dstType string,
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
