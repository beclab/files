package drives

import (
	"context"
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/parser"
	"files/pkg/pool"
	"files/pkg/preview"
	"fmt"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CacheResourceService struct {
	BaseResourceService
}

func ExecuteCacheSameTask(task *pool.Task, r *http.Request) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastLogLength := len(task.Log)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for {
		select {
		case <-task.Ctx.Done():
			return nil
		case <-ticker.C:
			done := false

			func() {
				taskUrl := fmt.Sprintf("http://127.0.0.1:80/api/cache/%s/task?task_id=%s", task.RelationNode, task.RelationTaskID)

				req, err := http.NewRequestWithContext(task.Ctx, "GET", taskUrl, nil)
				if err != nil {
					TaskLog(task, "error", fmt.Sprintf("failed to create request: %v", err))
					return
				}
				req.Header = r.Header

				resp, err := client.Do(req)
				if err != nil {
					TaskLog(task, "error", fmt.Sprintf("failed to query task status: %v", err))
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					TaskLog(task, "error", fmt.Sprintf("failed to query task status: %s", resp.Status))
					return
				}

				body, err := io.ReadAll(SuitableResponseReader(resp))
				if err != nil {
					TaskLog(task, "error", fmt.Sprintf("failed to read response body: %v", err))
					return
				}

				var apiResponse struct {
					Code int        `json:"code"`
					Msg  string     `json:"msg"`
					Task *pool.Task `json:"task"`
				}
				if err := json.Unmarshal(body, &apiResponse); err != nil {
					TaskLog(task, "error", fmt.Sprintf("failed to unmarshal response: %v", err))
					return
				}

				if apiResponse.Code != 0 {
					TaskLog(task, "error", fmt.Sprintf("API returned error code: %d, msg: %s", apiResponse.Code, apiResponse.Msg))
					return
				}

				select {
				case task.ProgressChan <- apiResponse.Task.Progress:
					klog.Infof("send progress %d", apiResponse.Task.Progress)
				default:
				}

				newLogs := make([]string, 0, len(apiResponse.Task.Log))
				for i := lastLogLength; i < len(apiResponse.Task.Log); i++ {
					if apiResponse.Task.Log[i] != "" {
						newLogs = append(newLogs, apiResponse.Task.Log[i])
					}
				}

				if len(newLogs) > 0 {
					for _, log := range newLogs {
						task.Logging(log)
					}

					lastLogLength = len(apiResponse.Task.Log)
				}

				if apiResponse.Task.FailedReason != "" {
					task.FailedReason = apiResponse.Task.FailedReason
				}

				if apiResponse.Task.Status == "failed" {
					pool.FailTask(task.ID)
					done = true
					return
				}

				if apiResponse.Task.Status == "completed" && apiResponse.Task.Progress == 100 {
					finalResp, err := client.Do(req.Clone(task.Ctx))
					if err != nil {
						TaskLog(task, "error", fmt.Sprintf("final check failed: %v", err))
						return
					}
					defer finalResp.Body.Close()

					finalBody, err := io.ReadAll(SuitableResponseReader(finalResp))
					if err != nil {
						TaskLog(task, "error", fmt.Sprintf("final read failed: %v", err))
						return
					}

					var finalApiResponse struct {
						Code int        `json:"code"`
						Msg  string     `json:"msg"`
						Task *pool.Task `json:"task"`
					}
					if err := json.Unmarshal(finalBody, &finalApiResponse); err != nil {
						TaskLog(task, "error", fmt.Sprintf("final unmarshal failed: %v", err))
						return
					}

					if finalApiResponse.Code != 0 ||
						finalApiResponse.Task.Status != "completed" ||
						finalApiResponse.Task.Progress != 100 {
						return
					}

					pool.CompleteTask(task.ID)

					done = true
					return
				}

			}()

			if done {
				return nil
			}
		}
	}
}

func (*CacheResourceService) PasteSame(task *pool.Task, action, src, dst string, srcFileParam, dstFileParam *models.FileParam,
	fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	srcExternalType := srcFileParam.FileType
	dstExternalType := dstFileParam.FileType
	return common.PatchAction(task, task.Ctx, action, src, dst, srcExternalType, dstExternalType, fileCache)

	//patchUrl := "http://127.0.0.1:80/api/resources/" + common.EscapeURLWithSpace(strings.TrimLeft(src, "/")) + "?action=" + action + "&destination=" + common.EscapeURLWithSpace(dst) + "&task=1"
	//method := "PATCH"
	//payload := []byte(``)
	//klog.Infoln(patchUrl)
	//
	//client := &http.Client{}
	//req, err := http.NewRequest(method, patchUrl, bytes.NewBuffer(payload))
	//if err != nil {
	//	return err
	//}
	//
	//req.Header = r.Header.Clone()
	//
	//res, err := client.Do(req)
	//if err != nil {
	//	return err
	//}
	//defer res.Body.Close()
	//
	//var response struct {
	//	TaskID string `json:"task_id"`
	//}
	//
	//if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
	//	return fmt.Errorf("failed to parse task_id: %v", err)
	//}
	//
	//task.RelationTaskID = response.TaskID
	//xTerminusNode := r.Header.Get("X-Terminus-Node")
	//task.RelationNode = xTerminusNode
	//
	//go func() {
	//	err = ExecuteCacheSameTask(task, r)
	//	if err != nil {
	//		klog.Errorf("Failed to initialize rsync: %v\n", err)
	//		return
	//	}
	//}()
	//
	//return nil
}

func (rs *CacheResourceService) PasteDirFrom(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
	dstFileParam *models.FileParam, dstType, dst string, d *common.Data,
	fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	srcUri, err := srcFileParam.GetResourceUri()
	if err != nil {
		return err
	}
	srcUrlPath := srcUri + srcFileParam.Path

	dstUri, err := dstFileParam.GetResourceUri()
	if err != nil {
		return err
	}

	srcinfo, err := fs.Stat(strings.TrimPrefix(srcUrlPath, "/data"))
	if err != nil {
		return err
	}
	mode := srcinfo.Mode()

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteDirTo(task, fs, src, dst, srcFileParam, dstFileParam, mode, fileCount, w, r, d, driveIdCache)
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
		select {
		case <-task.Ctx.Done():
			return nil
		default:
		}

		fsrc := filepath.Join(src, obj.Name())
		fdst := filepath.Join(fdstBase, obj.Name())

		fsrcFileParam := &models.FileParam{
			Owner:    srcFileParam.Owner,
			FileType: srcFileParam.FileType,
			Extend:   srcFileParam.Extend,
			Path:     strings.TrimPrefix(fsrc, strings.TrimPrefix(srcUri, "/data")),
		}
		fdstFileParam := &models.FileParam{
			Owner:    dstFileParam.Owner,
			FileType: dstFileParam.FileType,
			Extend:   dstFileParam.Extend,
			Path:     strings.TrimPrefix(fdst, strings.TrimPrefix(dstUri, "/data")),
		}

		if obj.IsDir() {
			// Create sub-directories, recursively.
			err = rs.PasteDirFrom(task, fs, fsrcFileParam, srcType, fsrc, fdstFileParam, dstType, fdst, d, obj.Mode(), fileCount, w, r, driveIdCache)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = rs.PasteFileFrom(task, fs, fsrcFileParam, srcType, fsrc, fdstFileParam, dstType, fdst, d, obj.Mode(), obj.Size(), fileCount, w, r, driveIdCache)
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

	//mode := fileMode
	//
	//handler, err := GetResourceService(dstType)
	//if err != nil {
	//	return err
	//}
	//
	//err = handler.PasteDirTo(task, fs, src, dst, mode, fileCount, w, r, d, driveIdCache)
	//if err != nil {
	//	return err
	//}
	//
	//var fdstBase string = dst
	//if driveIdCache[src] != "" {
	//	fdstBase = filepath.Join(filepath.Dir(filepath.Dir(strings.TrimSuffix(dst, "/"))), driveIdCache[src])
	//}
	//
	//type Item struct {
	//	Path      string `json:"path"`
	//	Name      string `json:"name"`
	//	Size      int64  `json:"size"`
	//	Extension string `json:"extension"`
	//	Modified  string `json:"modified"`
	//	Mode      uint32 `json:"mode"`
	//	IsDir     bool   `json:"isDir"`
	//	IsSymlink bool   `json:"isSymlink"`
	//	Type      string `json:"type"`
	//}
	//
	//type ResponseData struct {
	//	Items    []Item `json:"items"`
	//	NumDirs  int    `json:"numDirs"`
	//	NumFiles int    `json:"numFiles"`
	//	Sorting  struct {
	//		By  string `json:"by"`
	//		Asc bool   `json:"asc"`
	//	} `json:"sorting"`
	//	Path      string `json:"path"`
	//	Name      string `json:"name"`
	//	Size      int64  `json:"size"`
	//	Extension string `json:"extension"`
	//	Modified  string `json:"modified"`
	//	Mode      uint32 `json:"mode"`
	//	IsDir     bool   `json:"isDir"`
	//	IsSymlink bool   `json:"isSymlink"`
	//	Type      string `json:"type"`
	//}
	//
	//infoURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(src)
	//
	//client := &http.Client{}
	//request, err := http.NewRequest("GET", infoURL, nil)
	//if err != nil {
	//	klog.Errorf("create request failed: %v\n", err)
	//	return err
	//}
	//
	//request.Header = r.Header.Clone()
	//
	//response, err := client.Do(request)
	//if err != nil {
	//	klog.Errorf("request failed: %v\n", err)
	//	return err
	//}
	//defer response.Body.Close()
	//
	//var bodyReader io.Reader = response.Body
	//
	//if response.Header.Get("Content-Encoding") == "gzip" {
	//	gzipReader, err := gzip.NewReader(response.Body)
	//	if err != nil {
	//		klog.Errorf("unzip response failed: %v\n", err)
	//		return err
	//	}
	//	defer gzipReader.Close()
	//
	//	bodyReader = gzipReader
	//}
	//
	//body, err := ioutil.ReadAll(bodyReader)
	//if err != nil {
	//	klog.Errorf("read response failed: %v\n", err)
	//	return err
	//}
	//
	//var data ResponseData
	//err = json.Unmarshal(body, &data)
	//if err != nil {
	//	return err
	//}
	//
	//for _, item := range data.Items {
	//	select {
	//	case <-task.Ctx.Done():
	//		return nil
	//	default:
	//	}
	//
	//	fsrc := filepath.Join(src, item.Name)
	//	fdst := filepath.Join(fdstBase, item.Name)
	//
	//	if item.IsDir {
	//		err := rs.PasteDirFrom(task, fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), fileCount, w, r, driveIdCache)
	//		if err != nil {
	//			return err
	//		}
	//	} else {
	//		err := rs.PasteFileFrom(task, fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), item.Size, fileCount, w, r, driveIdCache)
	//		if err != nil {
	//			return err
	//		}
	//	}
	//}
	//return nil
}

func (rs *CacheResourceService) PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string,
	srcFileParam, dstFileParam *models.FileParam, fileMode os.FileMode, fileCount int64, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	mode := fileMode
	if err := fileutils.MkdirAllWithChown(fs, dst, mode); err != nil {
		klog.Errorln(err)
		return err
	}
	//if err := CacheMkdirAll(dst, fileMode, r); err != nil {
	//	return err
	//}
	return nil
}

func (rs *CacheResourceService) PasteFileFrom(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
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

	extRemains := IsThridPartyDrives(dstType)
	var bufferPath string

	var err error
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

	err = CacheFileToBuffer(task, src, bufferPath, left, mid)
	if err != nil {
		return err
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

func (rs *CacheResourceService) PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string,
	srcFileParam, dstFileParam *models.FileParam, fileMode os.FileMode, left, right int, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	status, err := CacheBufferToFile(task, bufferPath, dst, fileMode, d, left, right)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	task.Transferred += diskSize
	return nil
}

func (rs *CacheResourceService) GetStat(fs afero.Fs, fileParam *models.FileParam, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, 0, 0, false, err
	}
	urlPath := uri + fileParam.Path

	info, err := fs.Stat(strings.TrimPrefix(urlPath, "/data"))
	if err != nil {
		return nil, 0, 0, false, err
	}
	return info, info.Size(), info.Mode(), info.IsDir(), nil
}

func (rs *CacheResourceService) MoveDelete(task *pool.Task, fileCache fileutils.FileCache, fileParam *models.FileParam, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		klog.Errorln(err)
		return err
	}

	dirent := strings.TrimPrefix(uri+fileParam.Path, "/data")
	klog.Infoln("~~~Debug log: dirent:", dirent)

	status, err := ResourceCacheDelete(fileCache, dirent, task.Ctx, d, r)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *CacheResourceService) GeneratePathList(db *gorm.DB, rootPath string, processor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error {
	if rootPath == "" {
		rootPath = "/appcache"
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

	err = recordsStatusProcessor(db, processedPaths, []string{SrcTypeCache}, 1)
	if err != nil {
		klog.Errorf("records status processor failed: %v\n", err)
		return err
	}
	return err
}

func (rs *CacheResourceService) parsePathToURI(path string) (string, string) {
	pathSplit := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(pathSplit) < 2 {
		return "unknown", path
	}
	if strings.HasPrefix(pathSplit[1], "pvc-appcache-") {
		if len(pathSplit) == 2 {
			return "unknown", path
		}
		return "cache", filepath.Join(pathSplit[1:]...)
	}
	return "error", path
}

func (rs *CacheResourceService) GetFileCount(fs afero.Fs, fileParam *models.FileParam, countType string, w http.ResponseWriter, r *http.Request) (int64, error) {
	//newSrc := strings.Replace(src, "AppData/", "appcache/", 1)
	//klog.Infoln(newSrc)
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return 0, err
	}
	newSrc := uri + fileParam.Path

	srcinfo, err := os.Stat(newSrc)
	if err != nil {
		return 0, err
	}

	var count int64 = 0

	if srcinfo.IsDir() {
		err = filepath.Walk(newSrc, func(path string, info os.FileInfo, err error) error {
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

func (rs *CacheResourceService) GetTaskFileInfo(fs afero.Fs, fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error) {
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return false, "", "", err
	}
	urlPath := uri + fileParam.Path

	srcinfo, err := os.Stat(urlPath)
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

func CacheMkdirAll(dst string, mode os.FileMode, r *http.Request) error {
	targetURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(dst) + "/?mode=" + mode.String()

	request, err := http.NewRequest("POST", targetURL, nil)
	if err != nil {
		return err
	}

	request.Header = r.Header.Clone()
	request.Header.Set("Content-Type", "application/octet-stream")

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("file upload failed with status: %d", response.StatusCode)
	}

	return nil
}

func CacheFileToBuffer(task *pool.Task, src string, bufferFilePath string, left, right int) error {
	newSrc := strings.Replace(src, "AppData/", "appcache/", 1)
	newPath, err := common.UnescapeURLIfEscaped(newSrc)
	if err != nil {
		return err
	}
	klog.Infoln("newSrc:", newSrc, ", newPath:", newPath)

	err = fileutils.ExecuteRsync(task, newPath, bufferFilePath, left, right)
	if err != nil {
		klog.Errorf("Failed to initialize rsync: %v\n", err)
		return err
	}

	return nil
}

func CacheBufferToFile(task *pool.Task, bufferFilePath string, targetPath string, mode os.FileMode, d *common.Data, left, right int) (int, error) {
	// Directories creation on POST.
	if strings.HasSuffix(targetPath, "/") {
		if err := fileutils.MkdirAllWithChown(files.DefaultFs, targetPath, mode); err != nil {
			klog.Errorln(err)
			return common.ErrToStatus(err), err
		}
	}

	_, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       targetPath,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})

	newTargetPath := strings.Replace(targetPath, "AppData/", "appcache/", 1)
	err = fileutils.ExecuteRsync(task, bufferFilePath, newTargetPath, left, right)

	if err != nil {
		err = os.RemoveAll(newTargetPath)
		if err == nil {
			klog.Errorln("Rollback Failed:", err)
		}
		klog.Infoln("Rollback success")
	}

	return common.ErrToStatus(err), err
}

func ResourceCacheDelete(fileCache fileutils.FileCache, path string, ctx context.Context, d *common.Data, r *http.Request) (int, error) {
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
	//if path == "/" {
	//	return http.StatusForbidden, nil
	//}
	//
	//infoURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(path)
	//
	//client := &http.Client{}
	//request, err := http.NewRequest("DELETE", infoURL, nil)
	//if err != nil {
	//	klog.Errorf("delete request failed: %v\n", err)
	//	return common.ErrToStatus(err), err
	//}
	//
	//request.Header = r.Header
	//
	//response, err := client.Do(request)
	//if err != nil {
	//	klog.Errorf("request failed: %v\n", err)
	//	return common.ErrToStatus(err), err
	//}
	//defer response.Body.Close()
	//
	//return http.StatusOK, nil
}
