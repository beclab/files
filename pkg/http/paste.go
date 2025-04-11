package http

import (
	"context"
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/pool"
	"fmt"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

func resourcePasteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		var err error

		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		//srcType := r.URL.Query().Get("src_type")
		//if srcType == "" {
		//	srcType = drives.SrcTypeDrive
		//}
		srcType, err := drives.ParsePathType(r.URL.Path, r, false, true)
		if err != nil {
			return http.StatusBadRequest, err
		}
		//dstType := r.URL.Query().Get("dst_type")
		//if dstType == "" {
		//	dstType = drives.SrcTypeDrive
		//}
		dstType, err := drives.ParsePathType(r.URL.Query().Get("destination"), r, true, true)
		if err != nil {
			return http.StatusBadRequest, err
		}

		if !drives.ValidSrcTypes[srcType] {
			klog.Infoln("Src type is invalid!")
			return http.StatusForbidden, nil
		}
		if !drives.ValidSrcTypes[dstType] {
			klog.Infoln("Dst type is invalid!")
			return http.StatusForbidden, nil
		}

		if srcType == drives.SrcTypeData || srcType == drives.SrcTypeExternal {
			srcType = drives.SrcTypeDrive // In paste, data and external is dealt as same as drive
		}
		if dstType == drives.SrcTypeData || srcType == drives.SrcTypeExternal {
			dstType = drives.SrcTypeDrive // In paste, data and external is dealt as same as drive
		}

		if srcType == dstType {
			klog.Infoln("Src and dst are of same arch!")
		} else {
			klog.Infoln("Src and dst are of different arches!")
		}
		action := r.URL.Query().Get("action")

		klog.Infoln("src:", src)
		src, err = common.UnescapeURLIfEscaped(src)
		klog.Infoln("src:", src, "err:", err)
		klog.Infoln("dst:", dst)
		dst, err = common.UnescapeURLIfEscaped(dst)
		klog.Infoln("dst:", dst, "err:", err)
		if err != nil {
			return common.ErrToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		if dstType == drives.SrcTypeSync && strings.Contains(dst, "\\") {
			response := map[string]interface{}{
				"code": -1,
				"msg":  "Sync does not support directory entries with backslashes in their names.",
			}
			return common.RenderJSON(w, r, response)
		}

		rename := r.URL.Query().Get("rename") == "true"
		if !rename {
			if _, err := files.DefaultFs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		isDir := strings.HasSuffix(src, "/")
		if srcType == drives.SrcTypeGoogle && dstType != drives.SrcTypeGoogle {
			srcInfo, err := drives.GetGoogleDriveIdFocusedMetaInfos(src, w, r)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			srcName := srcInfo.Name
			formattedSrcName := common.RemoveSlash(srcName)
			dst = strings.ReplaceAll(dst, srcName, formattedSrcName)
			isDir = srcInfo.IsDir

			if !srcInfo.CanDownload {
				if srcInfo.CanExport {
					dst += srcInfo.ExportSuffix
				} else {
					response := map[string]interface{}{
						"code": -1,
						"msg":  "Google drive cannot export this file.",
					}
					return common.RenderJSON(w, r, response)
				}
			}
		}
		if rename && dstType != drives.SrcTypeGoogle {
			dst = drives.PasteAddVersionSuffix(dst, dstType, isDir, files.DefaultFs, w, r)
		}
		var same = srcType == dstType
		// all cloud drives of two users must be seen as diff archs
		var srcName, dstName string
		if srcType == drives.SrcTypeGoogle {
			_, srcName, _, _ = drives.ParseGoogleDrivePath(src)
		} else if drives.IsCloudDrives(srcType) {
			_, srcName, _ = drives.ParseCloudDrivePath(src)
		}
		if dstType == drives.SrcTypeGoogle {
			_, dstName, _, _ = drives.ParseGoogleDrivePath(dst)
		} else if drives.IsCloudDrives(srcType) {
			_, dstName, _ = drives.ParseCloudDrivePath(dst)
		}
		if srcName != dstName {
			same = false
		}

		//if same {
		//	err = pasteActionSameArch(action, srcType, src, dstType, dst, rename, fileCache, w, r)
		//} else {
		//	err = pasteActionDiffArch(r.Context(), action, srcType, src, dstType, dst, d, fileCache, w, r)
		//}
		//if common.ErrToStatus(err) == http.StatusRequestEntityTooLarge {
		//	fmt.Fprintln(w, err.Error())
		//}
		//return common.ErrToStatus(err), err

		taskID := fmt.Sprintf("task%d", time.Now().UnixNano())
		task := &pool.Task{
			ID:     taskID,
			Source: src,
			Dest:   dst,
			Status: "pending",
		}
		pool.TaskManager.Store(taskID, task)

		// 提交任务到任务队列
		pool.WorkerPool.Submit(func() {
			ctx, cancel := context.WithCancel(context.Background())
			pool.TaskManager.Store(taskID, &pool.Task{ID: taskID, Status: "running", Progress: 0})
			executePasteTask(ctx, &pool.Task{ID: taskID, Source: src, Dest: dst, Log: task.Log},
				same, action, srcType, dstType, rename, d, fileCache, w, r)
			cancel()
		})

		//w.Header().Set("Content-Type", "application/json")
		//json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
		return common.RenderJSON(w, r, map[string]string{"task_id": taskID})
	}
}

func executePasteTask(ctx context.Context, task *pool.Task, same bool, action, srcType, dstType string, rename bool,
	d *common.Data, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) {
	logChan := make(chan string, 100)
	defer close(logChan)

	go func() {
		var err error
		if same {
			err = pasteActionSameArch(action, srcType, task.Source, dstType, task.Dest, rename, fileCache, w, r)
		} else {
			err = pasteActionDiffArch(r.Context(), action, srcType, task.Source, dstType, task.Dest, d, fileCache, w, r)
		}
		if common.ErrToStatus(err) == http.StatusRequestEntityTooLarge {
			fmt.Fprintln(w, err.Error())
		}
		if err != nil {
			klog.Errorln(err)
		}
		return

		//// 子任务 1：本地文件复制
		//err := rsyncCopy(ctx, task.Source, task.Dest, logChan)
		//if err != nil {
		//	taskManager.Store(task.ID, &Task{ID: task.ID, Status: "failed", Log: []string{err.Error()}})
		//	return
		//}
		//
		//// 更新进度
		//taskManager.Store(task.ID, &Task{ID: task.ID, Status: "running", Progress: 50, Log: append(task.Log, <-logChan)})
		//
		//// 子任务 2：调用第三方接口上传
		//err = uploadToThirdParty(ctx, task.Dest, logChan)
		//if err != nil {
		//	taskManager.Store(task.ID, &Task{ID: task.ID, Status: "failed", Log: []string{err.Error()}})
		//	return
		//}
		//
		//// 更新进度
		//taskManager.Store(task.ID, &Task{ID: task.ID, Status: "completed", Progress: 100, Log: append(task.Log, <-logChan)})
	}()

	// 收集日志
	for log := range logChan {
		if t, ok := pool.TaskManager.Load(task.ID); ok {
			if existingTask, ok := t.(*pool.Task); ok {
				newTask := *existingTask
				newTask.Log = append(newTask.Log, log)
				pool.TaskManager.Store(task.ID, &newTask)
			}
		}
	}
	return
}

func doPaste(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, w http.ResponseWriter, r *http.Request) error {
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

	_, size, mode, isDir, err := handler.GetStat(fs, src, w, r)
	if err != nil {
		return err
	}

	var copyTempGoogleDrivePathIdCache = make(map[string]string)

	if isDir {
		err = handler.PasteDirFrom(fs, srcType, src, dstType, dst, d, mode, w, r, copyTempGoogleDrivePathIdCache)
	} else {
		err = handler.PasteFileFrom(fs, srcType, src, dstType, dst, d, mode, size, w, r, copyTempGoogleDrivePathIdCache)
	}
	if err != nil {
		return err
	}
	return nil
}

func pasteActionSameArch(action, srcType, src, dstType, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	klog.Infoln("Now deal with ", action, " for same arch ", dstType)
	klog.Infoln("src: ", src, ", dst: ", dst)

	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return err
	}

	return handler.PasteSame(action, src, dst, rename, fileCache, w, r)
}

func pasteActionDiffArch(ctx context.Context, action, srcType, src, dstType, dst string, d *common.Data, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	// In this function, context if tied up to src, because src is in the URL
	switch action {
	case "copy":
		return doPaste(files.DefaultFs, srcType, src, dstType, dst, d, w, r)
	case "rename":
		err := doPaste(files.DefaultFs, srcType, src, dstType, dst, d, w, r)
		if err != nil {
			return err
		}

		handler, err := drives.GetResourceService(srcType)
		if err != nil {
			return err
		}
		err = handler.MoveDelete(fileCache, src, ctx, d, w, r)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
	return nil
}
