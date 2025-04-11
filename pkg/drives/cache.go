package drives

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/parser"
	"files/pkg/pool"
	"fmt"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type CacheResourceService struct {
	BaseResourceService
}

func ExecuteCacheSameTask(task *pool.Task, r *http.Request) error {
	// 创建一个ticker用于定期轮询
	ticker := time.NewTicker(1 * time.Second) // 1秒轮询一次，可根据需要调整
	defer ticker.Stop()

	// 用于记录上一次的日志长度，避免重复发送
	lastLogLength := len(task.Log)

	// 创建一个HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for {
		select {
		case <-task.Ctx.Done():
			// 上下文被取消，退出循环
			return nil
		case <-ticker.C:
			done := false // 标志变量，用于控制循环退出

			// 将HTTP请求和响应处理的逻辑包装在一个匿名函数中
			func() {
				// 构造请求URL
				taskUrl := fmt.Sprintf("http://127.0.0.1:80/api/cache/%s/task?task_id=%s", task.RelationNode, task.RelationTaskID)

				// 发送HTTP请求
				req, err := http.NewRequestWithContext(task.Ctx, "GET", taskUrl, nil)
				if err != nil {
					TaskLog(task, "error", fmt.Sprintf("failed to create request: %v", err))
					//task.ErrChan <- fmt.Errorf("failed to create request: %v", err)
					return
				}
				req.Header = r.Header

				resp, err := client.Do(req)
				if err != nil {
					TaskLog(task, "error", fmt.Sprintf("failed to query task status: %v", err))
					//task.ErrChan <- fmt.Errorf("failed to query task status: %v", err)
					return
				}
				defer resp.Body.Close() // 确保响应体在函数返回时关闭

				if resp.StatusCode != http.StatusOK {
					TaskLog(task, "error", fmt.Sprintf("failed to query task status: %s", resp.Status))
					//task.ErrChan <- fmt.Errorf("failed to query task status: %s", resp.Status)
					return
				}

				// 读取响应体
				body, err := io.ReadAll(SuitableResponseReader(resp))
				if err != nil {
					TaskLog(task, "error", fmt.Sprintf("failed to read response body: %v", err))
					//task.ErrChan <- fmt.Errorf("failed to read response body: %v", err)
					return
				}

				// 解析JSON响应
				var apiResponse struct {
					Code int        `json:"code"`
					Msg  string     `json:"msg"`
					Task *pool.Task `json:"task"`
				}
				if err := json.Unmarshal(body, &apiResponse); err != nil {
					//klog.Infof("~~~Debug Log: failed to unmarshal response body: %v", body)
					//task.ErrChan <- fmt.Errorf("failed to unmarshal response: %v", err)
					TaskLog(task, "error", fmt.Sprintf("failed to unmarshal response: %v", err))
					return
				}

				if apiResponse.Code != 0 {
					TaskLog(task, "error", fmt.Sprintf("API returned error code: %d, msg: %s", apiResponse.Code, apiResponse.Msg))
					//task.ErrChan <- fmt.Errorf("API returned error code: %d, msg: %s", apiResponse.Code, apiResponse.Msg)
					return
				}

				klog.Infof("~~~Temp Log: apiResponse: %+v", apiResponse)

				// 加锁保护共享数据
				//task.Mu.Lock()
				//defer task.Mu.Unlock()

				// 更新任务状态
				//task.Status = apiResponse.Task.Status
				//task.Progress = apiResponse.Task.Progress

				// 发送进度更新
				select {
				case task.ProgressChan <- apiResponse.Task.Progress:
					klog.Infof("~~~Temp Log: send progress %d", apiResponse.Task.Progress)
				default:
				}

				// 处理日志更新
				newLogs := make([]string, 0, len(apiResponse.Task.Log))
				for i := lastLogLength; i < len(apiResponse.Task.Log); i++ {
					if apiResponse.Task.Log[i] != "" {
						newLogs = append(newLogs, apiResponse.Task.Log[i])
					}
				}

				if len(newLogs) > 0 {
					// 发送新日志
					for _, log := range newLogs {
						task.Logging(log)
						//select {
						//case task.LogChan <- log:
						//	klog.Infof("~~~Temp Log: send log %s", log)
						//default:
						//}
					}

					// 更新最后日志长度
					lastLogLength = len(apiResponse.Task.Log)
				}

				if apiResponse.Task.FailedReason != "" {
					task.Mu.Lock()
					task.FailedReason = apiResponse.Task.FailedReason
					task.Mu.Unlock()
					//select {
					//case task.ErrChan <- fmt.Errorf(apiResponse.Task.FailedReason):
					//	klog.Infof("~~~Temp Log: send fail reason %s", apiResponse.Task.FailedReason)
					//default:
					//}
				}

				if apiResponse.Task.Status == "failed" {
					pool.FailTask(task.ID)
					done = true
					return
				}

				// 检查任务是否已完成
				if apiResponse.Task.Status == "completed" && apiResponse.Task.Progress == 100 {
					// 确认任务完成，再检查一次确保状态稳定
					// 先解锁，避免死锁
					//task.Mu.Unlock()

					// 再检查一次
					finalResp, err := client.Do(req.Clone(task.Ctx))
					if err != nil {
						TaskLog(task, "error", fmt.Sprintf("final check failed: %v", err))
						//task.ErrChan <- fmt.Errorf("final check failed: %v", err)
						return
					}
					defer finalResp.Body.Close()

					finalBody, err := io.ReadAll(SuitableResponseReader(finalResp))
					if err != nil {
						TaskLog(task, "error", fmt.Sprintf("final read failed: %v", err))
						//task.ErrChan <- fmt.Errorf("final read failed: %v", err)
						return
					}

					var finalApiResponse struct {
						Code int        `json:"code"`
						Msg  string     `json:"msg"`
						Task *pool.Task `json:"task"`
					}
					if err := json.Unmarshal(finalBody, &finalApiResponse); err != nil {
						TaskLog(task, "error", fmt.Sprintf("final unmarshal failed: %v", err))
						//task.ErrChan <- fmt.Errorf("final unmarshal failed: %v", err)
						return
					}

					if finalApiResponse.Code != 0 ||
						finalApiResponse.Task.Status != "completed" ||
						finalApiResponse.Task.Progress != 100 {
						// 状态不稳定，继续轮询
						return
					}

					// 状态稳定，任务完成
					//task.Mu.Lock()
					//task.Status = "completed"
					//task.Progress = 100
					//task.Mu.Unlock()
					pool.CompleteTask(task.ID)

					// 发送最终进度
					//select {
					//case task.ProgressChan <- 100:
					//default:
					//}

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

func (*CacheResourceService) PasteSame(task *pool.Task, action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	patchUrl := "http://127.0.0.1:80/api/resources/" + common.EscapeURLWithSpace(strings.TrimLeft(src, "/")) + "?action=" + action + "&destination=" + common.EscapeURLWithSpace(dst) + "&rename=" + strconv.FormatBool(rename) + "&task=1"
	method := "PATCH"
	payload := []byte(``)
	klog.Infoln(patchUrl)

	client := &http.Client{}
	req, err := http.NewRequest(method, patchUrl, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header = r.Header.Clone()

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response struct {
		TaskID string `json:"task_id"`
	}

	if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to parse task_id: %v", err)
	}

	//_, err = ioutil.ReadAll(res.Body)
	//if err != nil {
	//	return err
	//}

	task.Mu.Lock()
	task.RelationTaskID = response.TaskID
	xTerminusNode := r.Header.Get("X-Terminus-Node")
	task.RelationNode = xTerminusNode
	task.Mu.Unlock()

	go func() {
		err = ExecuteCacheSameTask(task, r)
		if err != nil {
			// 如果 ExecuteRsyncWithContext 返回错误，直接打印并返回
			fmt.Printf("Failed to initialize rsync: %v\n", err)
			return
		}
	}()

	return nil
}

func (rs *CacheResourceService) PasteDirFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
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

	type Item struct {
		Path      string `json:"path"`
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		Extension string `json:"extension"`
		Modified  string `json:"modified"`
		Mode      uint32 `json:"mode"`
		IsDir     bool   `json:"isDir"`
		IsSymlink bool   `json:"isSymlink"`
		Type      string `json:"type"`
	}

	type ResponseData struct {
		Items    []Item `json:"items"`
		NumDirs  int    `json:"numDirs"`
		NumFiles int    `json:"numFiles"`
		Sorting  struct {
			By  string `json:"by"`
			Asc bool   `json:"asc"`
		} `json:"sorting"`
		Path      string `json:"path"`
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		Extension string `json:"extension"`
		Modified  string `json:"modified"`
		Mode      uint32 `json:"mode"`
		IsDir     bool   `json:"isDir"`
		IsSymlink bool   `json:"isSymlink"`
		Type      string `json:"type"`
	}

	infoURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(src)

	client := &http.Client{}
	request, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		klog.Errorf("create request failed: %v\n", err)
		return err
	}

	request.Header = r.Header.Clone()

	response, err := client.Do(request)
	if err != nil {
		klog.Errorf("request failed: %v\n", err)
		return err
	}
	defer response.Body.Close()

	var bodyReader io.Reader = response.Body

	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			klog.Errorf("unzip response failed: %v\n", err)
			return err
		}
		defer gzipReader.Close()

		bodyReader = gzipReader
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		klog.Errorf("read response failed: %v\n", err)
		return err
	}

	var data ResponseData
	err = json.Unmarshal(body, &data)
	if err != nil {
		return err
	}

	for _, item := range data.Items {
		fsrc := filepath.Join(src, item.Name)
		fdst := filepath.Join(fdstBase, item.Name)

		if item.IsDir {
			err := rs.PasteDirFrom(task, fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), fileCount, w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			err := rs.PasteFileFrom(task, fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), item.Size, fileCount, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *CacheResourceService) PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string, fileMode os.FileMode, fileCount int64, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	if err := CacheMkdirAll(dst, fileMode, r); err != nil {
		return err
	}
	return nil
}

func (rs *CacheResourceService) PasteFileFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
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
	klog.Info("~~~Debug Log: left=", left, "mid=", mid, "right=", right)

	err = CacheFileToBuffer(task, src, bufferPath, left, mid)
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

func (rs *CacheResourceService) PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, left, right int, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
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

func (rs *CacheResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}

	infoURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(src)

	client := &http.Client{}
	request, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		klog.Errorf("create request failed: %v\n", err)
		return nil, 0, 0, false, err
	}

	request.Header = r.Header.Clone()

	response, err := client.Do(request)
	if err != nil {
		klog.Errorf("request failed: %v\n", err)
		return nil, 0, 0, false, err
	}
	defer response.Body.Close()

	var bodyReader io.Reader = response.Body

	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			klog.Errorf("unzip response failed: %v\n", err)
			return nil, 0, 0, false, err
		}
		defer gzipReader.Close()

		bodyReader = gzipReader
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		klog.Errorf("read response failed: %v\n", err)
		return nil, 0, 0, false, err
	}

	var fileInfo struct {
		Size  int64       `json:"size"`
		Mode  os.FileMode `json:"mode"`
		IsDir bool        `json:"isDir"`
		Path  string      `json:"path"`
		Name  string      `json:"name"`
		Type  string      `json:"type"`
	}

	err = json.Unmarshal(body, &fileInfo)
	if err != nil {
		klog.Errorf("parse response failed: %v\n", err)
		return nil, 0, 0, false, err
	}

	return nil, fileInfo.Size, fileInfo.Mode, fileInfo.IsDir, nil
}

func (rs *CacheResourceService) MoveDelete(task *pool.Task, fileCache fileutils.FileCache, src string, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	status, err := ResourceCacheDelete(fileCache, src, task.Ctx, d, r)
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

func (rs *CacheResourceService) GetFileCount(fs afero.Fs, src, countType string, w http.ResponseWriter, r *http.Request) (int64, error) {
	newSrc := strings.Replace(src, "AppData/", "appcache/", 1)
	klog.Infoln(newSrc)
	//srcinfo, err := fs.Stat(newSrc)
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

func (rs *CacheResourceService) GetTaskFileInfo(fs afero.Fs, src string, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error) {
	newSrc := strings.Replace(src, "AppData/", "appcache/", 1)
	klog.Infoln(newSrc)
	//srcinfo, err := fs.Stat(newSrc)
	srcinfo, err := os.Stat(newSrc)
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

	//err = fileutils.IoCopyFileWithBufferOs(newPath, bufferFilePath, 8*1024*1024)
	err = fileutils.ExecuteRsync(task, newPath, bufferFilePath, left, right)
	if err != nil {
		fmt.Printf("Failed to initialize rsync: %v\n", err)
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
	//err = fileutils.IoCopyFileWithBufferOs(bufferFilePath, newTargetPath, 8*1024*1024)
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

	//newTargetPath := strings.Replace(path, "AppData/", "appcache/", 1)
	//err := os.RemoveAll(newTargetPath)
	//
	//if err != nil {
	//	return common.ErrToStatus(err), err
	//}

	infoURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(path)

	client := &http.Client{}
	request, err := http.NewRequest("DELETE", infoURL, nil)
	if err != nil {
		klog.Errorf("delete request failed: %v\n", err)
		return common.ErrToStatus(err), err
	}

	request.Header = r.Header

	response, err := client.Do(request)
	if err != nil {
		klog.Errorf("request failed: %v\n", err)
		return common.ErrToStatus(err), err
	}
	defer response.Body.Close()

	return http.StatusOK, nil
}
