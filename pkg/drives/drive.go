package drives

import (
	"bytes"
	"context"
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/parser"
	"files/pkg/pool"
	"files/pkg/preview"
	"fmt"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	MountedData   []files.DiskInfo = nil
	mu            sync.Mutex
	MountedTicker = time.NewTicker(5 * time.Minute)
)

// if cache logic is same as drive, it will be written in this file
type DriveResourceService struct {
	BaseResourceService
}

func (rs *DriveResourceService) PasteSame(task *pool.Task, action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	srcExternalType := files.GetExternalType(src, MountedData)
	dstExternalType := files.GetExternalType(dst, MountedData)
	return common.PatchAction(task, task.Ctx, action, src, dst, srcExternalType, dstExternalType, fileCache)
}

func (rs *DriveResourceService) PasteDirFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	srcinfo, err := fs.Stat(src)
	if err != nil {
		return err
	}
	mode := srcinfo.Mode()

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

	dir, _ := fs.Open(src)
	obs, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var errs []error

	for _, obj := range obs {
		fsrc := filepath.Join(src, obj.Name())
		fdst := filepath.Join(fdstBase, obj.Name())

		if obj.IsDir() {
			// Create sub-directories, recursively.
			err = rs.PasteDirFrom(task, fs, srcType, fsrc, dstType, fdst, d, obj.Mode(), fileCount, w, r, driveIdCache)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = rs.PasteFileFrom(task, fs, srcType, fsrc, dstType, fdst, d, obj.Mode(), obj.Size(), fileCount, w, r, driveIdCache)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	var errString string
	for _, err := range errs {
		errString += err.Error() + "\n"
	}

	if errString != "" {
		return e.New(errString)
	}
	return nil
}

func (rs *DriveResourceService) PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string, fileMode os.FileMode, fileCount int64, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	mode := fileMode
	if err := fileutils.MkdirAllWithChown(fs, dst, mode); err != nil {
		klog.Errorln(err)
		return err
	}
	return nil
}

func (rs *DriveResourceService) PasteFileFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	extRemains := IsThridPartyDrives(dstType)
	var bufferPath string
	fileInfo, status, err := ResourceDriveGetInfo(src, r, d)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	diskSize = fileInfo.Size
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return err
	}
	bufferPath, err = GenerateBufferFileName(src, bflName, extRemains)
	if err != nil {
		return err
	}
	task.AddBuffer(bufferPath)

	defer func() {
		logMsg := fmt.Sprintf("Remove copy buffer")
		TaskLog(task, "info", logMsg)
		RemoveDiskBuffer(task, bufferPath, srcType)
	}()

	err = MakeDiskBuffer(bufferPath, diskSize, false)
	if err != nil {
		return err
	}

	left, mid, right := CalculateProgressRange(task, diskSize)

	err = DriveFileToBuffer(task, fileInfo, bufferPath, left, mid)
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

func (rs *DriveResourceService) PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string, fileMode os.FileMode,
	left, right int, w http.ResponseWriter, r *http.Request, d *common.Data, diskSize int64) error {
	status, err := DriveBufferToFile(task, bufferPath, dst, fileMode, d, left, right)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	task.Mu.Lock()
	task.Transferred += diskSize
	task.Mu.Unlock()
	return nil
}

func (rs *DriveResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}

	info, err := fs.Stat(src)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return info, info.Size(), info.Mode(), info.IsDir(), nil
}

func (rs *DriveResourceService) MoveDelete(task *pool.Task, fileCache fileutils.FileCache, src string, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	status, err := ResourceDriveDelete(fileCache, src, task.Ctx, d)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *DriveResourceService) GeneratePathList(db *gorm.DB, rootPath string, processor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error {
	if rootPath == "" {
		rootPath = "/data"
	}

	processedPaths := make(map[string]bool)
	processedPathEntries := make(map[string]ProcessedPathsEntry)
	var sendS3Files = []os.FileInfo{}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			klog.Errorf("Access error: %v\n", err)
			return nil
		}

		if info.IsDir() {
			if info.Mode()&os.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			// Process directory
			drive, parsedPath := rs.parsePathToURI(path)

			key := fmt.Sprintf("%s:%s", drive, parsedPath)
			processedPaths[key] = true

			op, err := processor(db, drive, parsedPath, info.ModTime())
			processedPathEntries[key] = ProcessedPathsEntry{
				Drive: drive,
				Path:  parsedPath,
				Mtime: info.ModTime(),
				Op:    op,
			}
			return err
		} else {
			fileDir := filepath.Dir(path)
			drive, parsedPath := rs.parsePathToURI(fileDir)

			key := fmt.Sprintf("%s:%s", drive, parsedPath)

			if entry, exists := processedPathEntries[key]; exists {
				if !info.ModTime().Before(entry.Mtime) || entry.Op == 1 { // create need to send to S3
					sendS3Files = append(sendS3Files, info)

					if len(sendS3Files) == 100 {
						callSendS3MultiFiles(sendS3Files) // TODO: Just take this position now
						sendS3Files = sendS3Files[:0]
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		klog.Errorln("Error walking the path:", err)
	}

	if len(sendS3Files) > 0 {
		callSendS3MultiFiles(sendS3Files) // TODO: Just take this position now
		sendS3Files = sendS3Files[:0]
	}

	err = recordsStatusProcessor(db, processedPaths, []string{SrcTypeDrive, SrcTypeData, SrcTypeExternal}, 1)
	if err != nil {
		klog.Errorf("records status processor failed: %v\n", err)
		return err
	}
	return err
}

func (rs *DriveResourceService) parsePathToURI(path string) (string, string) {
	pathSplit := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(pathSplit) < 2 {
		return "unknown", path
	}
	if strings.HasPrefix(pathSplit[1], "pvc-userspace-") {
		if len(pathSplit) == 2 {
			return "unknown", path
		}
		if pathSplit[2] == "Data" {
			return "data", filepath.Join(pathSplit[1:]...)
		} else if pathSplit[2] == "Home" {
			return "drive", filepath.Join(pathSplit[1:]...)
		}
	}
	if pathSplit[1] == "External" {
		externalPath := ParseExternalPath(filepath.Join(pathSplit[2:]...))
		return "external", externalPath
	}
	return "error", path
}

func (rs *DriveResourceService) GetFileCount(fs afero.Fs, src, countType string, w http.ResponseWriter, r *http.Request) (int64, error) {
	srcinfo, err := fs.Stat(src)
	if err != nil {
		return 0, err
	}

	var count int64 = 0

	if srcinfo.IsDir() {
		err = afero.Walk(fs, src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				if countType == "size" {
					count += info.Size()
				} else {
					count++
				}
			}
			return nil
		})

		if err != nil {
			klog.Infoln("Error walking the directory:", err)
			return 0, err
		}
		klog.Infoln("Directory traversal completed.")
	} else {
		if countType == "size" {
			count = srcinfo.Size()
		} else {
			count = 1
		}
	}
	return count, nil
}

func (rs *DriveResourceService) GetTaskFileInfo(fs afero.Fs, src string, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error) {
	srcinfo, err := fs.Stat(src)
	if err != nil {
		return false, "", "", err
	}
	isDir = srcinfo.IsDir()
	filename = srcinfo.Name()
	fileType = ""
	if !isDir {
		fileType = parser.MimeTypeByExtension(filename)
	}

	return isDir, fileType, filename, nil
}

func generateListingData(listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string, d *common.Data, mountedData []files.DiskInfo) {
	defer close(dataChan)

	var A []*files.FileInfo
	listing.Lock()
	A = append(A, listing.Items...)
	listing.Unlock()

	for len(A) > 0 {
		firstItem := A[0]

		if firstItem.IsDir {
			var file *files.FileInfo
			var err error
			if mountedData != nil {
				file, err = files.NewFileInfoWithDiskInfo(files.FileOptions{
					Fs:         files.DefaultFs,
					Path:       firstItem.Path,
					Modify:     true,
					Expand:     true,
					ReadHeader: d.Server.TypeDetectionByHeader,
					Content:    true,
				}, mountedData)
			} else {
				file, err = files.NewFileInfo(files.FileOptions{
					Fs:         files.DefaultFs,
					Path:       firstItem.Path,
					Modify:     true,
					Expand:     true,
					ReadHeader: d.Server.TypeDetectionByHeader,
					Content:    true,
				})
			}
			if err != nil {
				klog.Error(err)
				return
			}

			var nestedItems []*files.FileInfo
			if file.Listing != nil {
				nestedItems = append(nestedItems, file.Listing.Items...)
			}
			A = append(nestedItems, A[1:]...)
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

func formatSSEvent(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("data: %s\n\n", jsonData)
}

func streamListingItems(w http.ResponseWriter, r *http.Request, listing *files.Listing, d *common.Data, mountedData []files.DiskInfo) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateListingData(listing, stopChan, dataChan, d, mountedData)

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

func ResourceDriveGetInfo(path string, r *http.Request, d *common.Data) (*files.FileInfo, int, error) {
	xBflUser := r.Header.Get("X-Bfl-User")
	klog.Infoln("X-Bfl-User: ", xBflUser)

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       path,
		Modify:     true,
		Expand:     true,
		ReadHeader: d.Server.TypeDetectionByHeader,
		Content:    true,
	})
	if err != nil {
		return file, common.ErrToStatus(err), err
	}

	if file.IsDir {
		file.Listing.Sorting = files.DefaultSorting
		file.Listing.ApplySort()
		return file, http.StatusOK, nil
	}

	if file.Type == "video" {
		osSystemServer := "system-server.user-system-" + xBflUser

		httpposturl := fmt.Sprintf("http://%s/legacy/v1alpha1/api.intent/v1/server/intent/send", osSystemServer)

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

		body, _ := ioutil.ReadAll(response.Body)
		klog.Infoln("response Body:", string(body))
	}

	return file, http.StatusOK, nil
}

func DriveFileToBuffer(task *pool.Task, file *files.FileInfo, bufferFilePath string, left, right int) error {
	path, err := common.UnescapeURLIfEscaped(file.Path)
	if err != nil {
		return err
	}
	klog.Infoln("file.Path:", file.Path, ", path:", path)

	err = fileutils.ExecuteRsync(task, "/data"+path, bufferFilePath, left, right)
	if err != nil {
		klog.Errorf("Failed to initialize rsync: %v\n", err)
		return err
	}

	return nil
}

func DriveBufferToFile(task *pool.Task, bufferFilePath string, targetPath string, mode os.FileMode, d *common.Data, left, right int) (int, error) {
	klog.Infoln("***DriveBufferToFile!")
	klog.Infoln("*** bufferFilePath:", bufferFilePath)
	klog.Infoln("*** targetPath:", targetPath)

	var err error
	targetPath, err = common.UnescapeURLIfEscaped(targetPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Directories creation on POST.
	if strings.HasSuffix(targetPath, "/") {
		if err = fileutils.MkdirAllWithChown(files.DefaultFs, targetPath, mode); err != nil {
			klog.Errorln(err)
			return common.ErrToStatus(err), err
		}
	}

	_, err = files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       targetPath,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})

	err = fileutils.ExecuteRsync(task, bufferFilePath, "/data"+targetPath, left, right)

	if err != nil {
		_ = files.DefaultFs.RemoveAll(targetPath)
	}

	return common.ErrToStatus(err), err
}

func ResourceDriveDelete(fileCache fileutils.FileCache, path string, ctx context.Context, d *common.Data) (int, error) {
	if path == "/" {
		return http.StatusForbidden, nil
	}

	srcinfo, err := files.DefaultFs.Stat(path)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	if srcinfo.IsDir() {
		// first recursively delete all thumbs
		err = filepath.Walk("/data"+path, func(subPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				file, err := files.NewFileInfo(files.FileOptions{
					Fs:         files.DefaultFs,
					Path:       subPath,
					Modify:     true,
					Expand:     false,
					ReadHeader: false,
				})
				if err != nil {
					return err
				}

				// delete thumbnails
				err = preview.DelThumbs(ctx, fileCache, file)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			klog.Infoln("Error walking the directory:", err)
		} else {
			klog.Infoln("Directory traversal completed.")
		}
	} else {
		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       path,
			Modify:     true,
			Expand:     false,
			ReadHeader: false,
		})
		if err != nil {
			return common.ErrToStatus(err), err
		}

		// delete thumbnails
		err = preview.DelThumbs(ctx, fileCache, file)
		if err != nil {
			return common.ErrToStatus(err), err
		}
	}

	err = files.DefaultFs.RemoveAll(path)

	if err != nil {
		return common.ErrToStatus(err), err
	}

	return http.StatusOK, nil
}

func ParseExternalPath(path string) string {
	for _, datum := range MountedData {
		if strings.HasPrefix(path, datum.Path) {
			idSerial := datum.IDSerial
			if idSerial == "" {
				idSerial = datum.Type + "_device"
			}
			partationUUID := datum.PartitionUUID
			if partationUUID == "" {
				partationUUID = datum.Type + "_partition"
			}
			return filepath.Join(datum.Type, idSerial, partationUUID, path)
		}
	}
	return ""
}
