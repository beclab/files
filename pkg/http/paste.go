package http

import (
	"crypto/rand"
	"encoding/base64"
	e "errors"
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/pool"
	"fmt"
	"github.com/spf13/afero"
	"io/fs"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func resourcePasteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		var err error

		//src := r.URL.Path
		//dst := r.URL.Query().Get("destination")
		//srcType, err := drives.ParsePathType(r.URL.Path, r, false, true)
		//if err != nil {
		//	return http.StatusBadRequest, err
		//}
		//dstType, err := drives.ParsePathType(r.URL.Query().Get("destination"), r, true, true)
		//if err != nil {
		//	return http.StatusBadRequest, err
		//}
		//
		//if !drives.ValidSrcTypes[srcType] {
		//	klog.Infoln("Src type is invalid!")
		//	return http.StatusForbidden, nil
		//}
		//if !drives.ValidSrcTypes[dstType] {
		//	klog.Infoln("Dst type is invalid!")
		//	return http.StatusForbidden, nil
		//}
		srcFileParam, handler, err := UrlPrep(r, "")
		if err != nil {
			return http.StatusBadRequest, err
		}

		dstFileParam, _, err := UrlPrep(r, r.URL.Query().Get("destination"))
		if err != nil {
			return http.StatusBadRequest, err
		}

		detailSrcType := srcFileParam.FileType
		detailDstType := dstFileParam.FileType

		srcType := detailSrcType
		dstType := detailDstType

		if detailSrcType == drives.SrcTypeData || detailSrcType == drives.SrcTypeExternal {
			srcType = drives.SrcTypeDrive // In paste, data and external is dealt as same as drive
		}
		if detailDstType == drives.SrcTypeData || detailDstType == drives.SrcTypeExternal {
			dstType = drives.SrcTypeDrive // In paste, data and external is dealt as same as drive
		}

		if srcType == dstType {
			klog.Infoln("Src and dst are of same arch!")
		} else {
			klog.Infoln("Src and dst are of different arches!")
		}
		action := r.URL.Query().Get("action")

		//klog.Infoln("src:", src)
		//src, err = common.UnescapeURLIfEscaped(src)
		//klog.Infoln("src:", src, "err:", err)
		//klog.Infoln("dst:", dst)
		//dst, err = common.UnescapeURLIfEscaped(dst)
		//klog.Infoln("dst:", dst, "err:", err)
		//if err != nil {
		//	return common.ErrToStatus(err), err
		//}
		srcUri, err := srcFileParam.GetResourceUri()
		if err != nil {
			return http.StatusBadRequest, err
		}
		src := srcUri + srcFileParam.Path
		dstUri, err := dstFileParam.GetResourceUri()
		if err != nil {
			return http.StatusBadRequest, err
		}
		dst := dstUri + dstFileParam.Path

		if dstFileParam.Path == "/" || srcFileParam.Path == "/" {
			return http.StatusForbidden, nil
		}

		if dstType == drives.SrcTypeSync && strings.Contains(dstFileParam.Path, "\\") {
			response := map[string]interface{}{
				"code": -1,
				"msg":  "Sync does not support directory entries with backslashes in their names.",
			}
			return common.RenderJSON(w, r, response)
		}

		isDir := strings.HasSuffix(srcFileParam.Path, "/")
		if srcType == drives.SrcTypeGoogle && dstType != drives.SrcTypeGoogle {
			srcInfo, err := drives.GetGoogleDriveIdFocusedMetaInfosFileParam(nil, srcFileParam, w, r)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			srcName := srcInfo.Name
			formattedSrcName := common.RemoveSlash(srcName)
			dst = strings.ReplaceAll(dst, srcName, formattedSrcName)
			dstFileParam.Path = strings.ReplaceAll(dstFileParam.Path, srcName, formattedSrcName) // TODO no need to deal here
			isDir = srcInfo.IsDir

			if !srcInfo.CanDownload {
				if srcInfo.CanExport {
					dst += srcInfo.ExportSuffix
					dstFileParam.Path += srcInfo.ExportSuffix // TODO this is ugly
				} else {
					response := map[string]interface{}{
						"code": -1,
						"msg":  "Google drive cannot export this file.",
					}
					return common.RenderJSON(w, r, response)
				}
			}
		}
		if dstType != drives.SrcTypeGoogle {
			dst = drives.PasteAddVersionSuffix(dst, dstFileParam, isDir, files.DefaultFs, w, r)
		}

		//// dst changes huge, need to recreate dstFileParam
		//dstFileParam, _, err = UrlPrep(r, dst)
		//if err != nil {
		//	return http.StatusBadRequest, err
		//}

		var same = srcType == dstType
		// all cloud drives of two users must be seen as diff archs
		var srcName, dstName string
		if drives.IsThridPartyDrives(srcType) {
			srcName = srcFileParam.Extend
		}
		if drives.IsThridPartyDrives(dstType) {
			dstName = dstFileParam.Extend
		}
		if srcName != dstName {
			same = false
		}

		//handler, err := drives.GetResourceService(srcType)
		//if err != nil {
		//	return http.StatusBadRequest, err
		//}
		_, fileType, filename, err := handler.GetTaskFileInfo(files.DefaultFs, srcFileParam, w, r)

		taskID := fmt.Sprintf("task%d", time.Now().UnixNano())
		task := pool.NewTask(taskID, strings.TrimPrefix(src, "/data"), strings.TrimPrefix(dst, "/data"), detailSrcType, detailDstType, action, drives.TaskCancellable(srcType, dstType, same), drives.IsThridPartyDrives(srcType), isDir, fileType, filename)
		pool.TaskManager.Store(taskID, task)

		pool.WorkerPool.Submit(func() {
			klog.Infof("Task %s started", taskID)
			defer klog.Infof("Task %s exited", taskID)

			if loadedTask, ok := pool.TaskManager.Load(taskID); ok {
				if concreteTask, ok := loadedTask.(*pool.Task); ok {
					concreteTask.Status = "running"
					concreteTask.Progress = 0

					executePasteTask(concreteTask, same, action, srcType, dstType, srcFileParam, dstFileParam, d, fileCache, w, r)
				}
			}
		})

		return common.RenderJSON(w, r, map[string]string{"task_id": taskID})
	}
}

func checkDiskSpace(path string) (bool, error) {
	var stat syscall.Statfs_t

	err := syscall.Statfs(path, &stat)
	if err != nil {
		return false, fmt.Errorf("failed to get filesystem stats: %v", err)
	}

	// Calculate available space in bytes
	availableBytes := stat.Bavail * uint64(stat.Bsize)

	// Define a threshold for disk space (e.g., 100MB)
	const threshold = 100 * 1024 * 1024 // 100MB

	if availableBytes < uint64(threshold) {
		return true, nil // Disk is full or nearly full
	}

	return false, nil // Disk has sufficient space
}

func createAndRemoveTempFile(targetDir string) error {
	dir := fileutils.FindExistingDir(targetDir)
	if dir == "" {
		return fmt.Errorf("no writable directory found in path hierarchy of %q", targetDir)
	}

	// Check if disk is full before proceeding
	isFull, err := checkDiskSpace(dir)
	if err != nil {
		return fmt.Errorf("failed to check disk space: %v", err)
	}
	if isFull {
		return fmt.Errorf("disk full or nearly full in directory %q", dir)
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Errorf("failed to generate random bytes: %v", err)
	}
	randomStr := base64.URLEncoding.EncodeToString(randomBytes)[:8]
	filename := fmt.Sprintf("temp_%s_%s.testwriting", timestamp, randomStr)
	filePath := filepath.Join(dir, filename)

	defer func() {
		_ = os.Remove(filePath)
		klog.Infof("Cleaned up temporary file %s", filePath)
	}()

	klog.Infof("Creating temporary file %s", filePath)

	if err := os.WriteFile(filePath, []byte{0}, 0o644); err != nil {
		var pathErr *fs.PathError
		if e.As(err, &pathErr) {
			if pathErr.Err == syscall.EACCES || pathErr.Err == syscall.EPERM {
				return fmt.Errorf("permission denied: failed to create file: %v", err)
			} else if pathErr.Err == syscall.EROFS {
				return fmt.Errorf("read-only file system: failed to create file: %v", err)
			}
		}
		return fmt.Errorf("failed to create file: %v", err)
	}

	return nil
}

func executePasteTask(task *pool.Task, same bool, action, srcType, dstType string, srcFileParam, dstFileParam *models.FileParam,
	d *common.Data, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) {
	select {
	case <-task.Ctx.Done():
		return
	default:
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error = nil

		if task.DstType == drives.SrcTypeExternal {
			err = createAndRemoveTempFile(path.Dir(common.RootPrefix + task.Dest))
			if err != nil {
				task.ErrChan <- fmt.Errorf("writing test failed: %w", err)
				task.LogChan <- fmt.Sprintf("writing test failed: %v", err)
				pool.FailTask(task.ID)
			}
		}

		if err == nil {
			if same {
				err = pasteActionSameArch(task, action, srcFileParam, srcType, task.Source, dstFileParam, dstType, task.Dest, fileCache, w, r)
			} else {
				err = pasteActionDiffArch(task, action, srcFileParam, srcType, task.Source, dstFileParam, dstType, task.Dest, d, fileCache, w, r)
			}
			if common.ErrToStatus(err) == http.StatusRequestEntityTooLarge {
				fmt.Fprintln(w, err.Error())
			}
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
			klog.Errorf("[TASK EXECUTE ERROR]: %v", err)
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

func doPaste(task *pool.Task, fs afero.Fs, srcFileParam *models.FileParam, srcType, src string,
	dstFileParam *models.FileParam, dstType, dst string, d *common.Data, w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	// path.Clean, only operate on string level, so it fits every src/dst type.
	if srcType != drives.SrcTypeAWSS3 {
		if src = path.Clean("/" + src); src == "" {
			return os.ErrNotExist
		}
	}

	if dstType != drives.SrcTypeAWSS3 {
		if dst = path.Clean("/" + dst); dst == "" {
			return os.ErrNotExist
		}
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the virtual root directory.
		return os.ErrInvalid
	}

	// Only when URL and type are both the same, it is not OK.
	if (dst == src) && (dstType == srcType) {
		return os.ErrInvalid
	}

	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return err
	}

	_, size, mode, isDir, err := handler.GetStat(fs, srcFileParam, w, r)
	if err != nil {
		return err
	}

	var copyTempGoogleDrivePathIdCache = make(map[string]string)

	fileCount, err := handler.GetFileCount(fs, srcFileParam, "size", w, r)
	if err != nil {
		klog.Errorln(err)
		return err
	}
	task.TotalFileSize = fileCount
	if isDir {
		err = handler.PasteDirFrom(task, fs, srcFileParam, srcType, src, dstFileParam, dstType, dst, d, mode, fileCount, w, r, copyTempGoogleDrivePathIdCache)
	} else {
		err = handler.PasteFileFrom(task, fs, srcFileParam, srcType, src, dstFileParam, dstType, dst, d, mode, size, fileCount, w, r, copyTempGoogleDrivePathIdCache)
	}
	if err != nil {
		return err
	}
	return nil
}

func pasteActionSameArch(task *pool.Task, action string, srcFileParam *models.FileParam, srcType, src string,
	dstFileParam *models.FileParam, dstType, dst string, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	klog.Infoln("Now deal with ", action, " for same arch ", dstType)
	klog.Infoln("src: ", src, ", dst: ", dst)

	err := func() error {
		handler, err := drives.GetResourceService(srcType)
		if err != nil {
			return err
		}

		fileCount, err := handler.GetFileCount(files.DefaultFs, srcFileParam, "size", w, r)
		if err != nil {
			klog.Errorln(err)
			return err
		}
		task.TotalFileSize = fileCount

		err = handler.PasteSame(task, action, src, dst, srcFileParam, dstFileParam, fileCache, w, r)
		if err != nil {
			return err
		}
		pool.TaskManager.Store(task.ID, task)
		return nil
	}()

	select {
	case <-task.Ctx.Done():
		return err
	default:
		// doPaste always set progress to 99 at the end
		if err != nil {
			task.ErrChan <- err
			task.LogChan <- fmt.Sprintf("%s from %s to %s failed", action, src, dst)
			pool.FailTask(task.ID)
			return err
		} else {
			return nil
		}
	}
}

func pasteActionDiffArch(task *pool.Task, action string, srcFileParam *models.FileParam, srcType, src string,
	dstFileParam *models.FileParam, dstType, dst string, d *common.Data, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
	}

	// In this function, context if tied up to src, because src is in the URL
	xTerminusNode := r.Header.Get("X-Terminus-Node")
	task.RelationNode = xTerminusNode

	var err error
	switch action {
	case "copy":
		err = doPaste(task, files.DefaultFs, srcFileParam, srcType, src, dstFileParam, dstType, dst, d, w, r)
	case "move":
		err = doPaste(task, files.DefaultFs, srcFileParam, srcType, src, dstFileParam, dstType, dst, d, w, r)
		if err != nil {
			break
		}

		var handler drives.ResourceService
		handler, err = drives.GetResourceService(srcType)
		if err != nil {
			break
		}
		err = handler.MoveDelete(task, fileCache, srcFileParam, d, w, r)
		if err != nil {
			break
		}
	default:
		err = fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}

	select {
	case <-task.Ctx.Done():
		return err
	default:
		// doPaste always set progress to 99 at the end
		if err != nil {
			task.ErrChan <- err
			task.LogChan <- fmt.Sprintf("%s from %s to %s failed", action, src, dst)
			pool.FailTask(task.ID)
			return err
		} else {
			task.LogChan <- fmt.Sprintf("%s from %s to %s successfully", action, src, dst)
			pool.CompleteTask(task.ID)
			return nil
		}
	}
}
