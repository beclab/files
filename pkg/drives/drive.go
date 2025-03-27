package drives

import (
	"bytes"
	"context"
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
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
)

var (
	mountedData []files.DiskInfo = nil
	mu          sync.Mutex
)

// if cache logic is same as drive, it will be written in this file
type DriveResourceService struct {
	BaseResourceService
}

func (rs *DriveResourceService) PasteSame(action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	GetMountedData()
	srcExternalType := files.GetExternalType(src, mountedData)
	dstExternalType := files.GetExternalType(dst, mountedData)
	return common.PatchAction(r.Context(), action, src, dst, srcExternalType, dstExternalType, fileCache)
}

func (rs *DriveResourceService) PasteDirFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	srcinfo, err := fs.Stat(src)
	if err != nil {
		return err
	}
	mode := srcinfo.Mode()

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteDirTo(fs, src, dst, mode, w, r, d, driveIdCache)
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
			err = rs.PasteDirFrom(fs, srcType, fsrc, dstType, fdst, d, obj.Mode(), w, r, driveIdCache)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = rs.PasteFileFrom(fs, srcType, fsrc, dstType, fdst, d, obj.Mode(), obj.Size(), w, r, driveIdCache)
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

func (rs *DriveResourceService) PasteDirTo(fs afero.Fs, src, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	mode := fileMode
	if err := fileutils.MkdirAllWithChown(fs, dst, mode); err != nil {
		klog.Errorln(err)
		return err
	}
	return nil
}

func (rs *DriveResourceService) PasteFileFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
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

	err = MakeDiskBuffer(bufferPath, diskSize, false)
	if err != nil {
		return err
	}
	err = DriveFileToBuffer(fileInfo, bufferPath)
	if err != nil {
		return err
	}

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(bufferPath, srcType)
	}()

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteFileTo(fs, bufferPath, dst, mode, w, r, d, diskSize)
	if err != nil {
		return err
	}
	return nil
}

func (rs *DriveResourceService) PasteFileTo(fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	status, err := DriveBufferToFile(bufferPath, dst, fileMode, d)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
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

func (rs *DriveResourceService) MoveDelete(fileCache fileutils.FileCache, src string, ctx context.Context, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	status, err := ResourceDriveDelete(fileCache, src, ctx, d)
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
	GetMountedData()

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

func DriveFileToBuffer(file *files.FileInfo, bufferFilePath string) error {
	path, err := common.UnescapeURLIfEscaped(file.Path)
	if err != nil {
		return err
	}
	klog.Infoln("file.Path:", file.Path, ", path:", path)

	err = fileutils.IoCopyFileWithBufferOs("/data"+path, bufferFilePath, 8*1024*1024)
	if err != nil {
		return err
	}

	return nil
}

func DriveBufferToFile(bufferFilePath string, targetPath string, mode os.FileMode, d *common.Data) (int, error) {
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

	err = fileutils.IoCopyFileWithBufferOs(bufferFilePath, "/data"+targetPath, 8*1024*1024)

	if err != nil {
		_ = files.DefaultFs.RemoveAll(targetPath)
	}

	return common.ErrToStatus(err), err
}

func ResourceDriveDelete(fileCache fileutils.FileCache, path string, ctx context.Context, d *common.Data) (int, error) {
	if path == "/" {
		return http.StatusForbidden, nil
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       path,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})
	if err != nil {
		return common.ErrToStatus(err), err
	}

	// delete thumbnails
	err = preview.DelThumbs(ctx, fileCache, file)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	err = files.DefaultFs.RemoveAll(path)

	if err != nil {
		return common.ErrToStatus(err), err
	}

	return http.StatusOK, nil
}

//func GetMountedData() {
//	mu.Lock()
//	defer mu.Unlock()
//
//	url := "http://" + files.TerminusdHost + "/system/mounted-path-incluster"
//
//	headers := make(http.Header)
//	headers.Set("Content-Type", "application/json")
//	headers.Set("X-Signature", "temp_signature")
//
//	tempMountedData, err := files.FetchDiskInfo(url, headers)
//	if err != nil {
//		klog.Errorln(err)
//		return
//	}
//	mountedData = tempMountedData
//	return
//}

func ParseExternalPath(path string) string {
	for _, datum := range mountedData {
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
