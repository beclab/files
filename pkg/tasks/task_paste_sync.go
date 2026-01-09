package tasks

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/drivers/sync/seahub/searpc"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

/**
 * ~ DownloadFromSync
 */
func (t *Task) DownloadFromSync() error {
	// sync > posix; sync > cloud

	var cmd = rclone.Command
	var user = t.param.Owner
	var action = t.param.Action
	var src = t.param.Src
	var share = t.param.Share
	var srcOwner = t.param.SrcOwner
	var dstOwner = t.param.DstOwner

	var tempParam *models.FileParam
	var dst *models.FileParam

	if t.param.Dst.IsCloud() { // sync->cloud, if phase 1 complete
		if t.pausedPhase == t.totalPhases {
			klog.Infof("[Task] Id: %s, resume phase: %d", t.id, t.pausedPhase)
			return nil
		}
	}

	if t.wasPaused && t.pausedParam == nil {
		klog.Errorf("[Task] Id: %s, paused param invalid", t.id)
		return errors.New("pause param invalid")
	}

	if !t.param.Dst.IsCloud() && t.wasPaused {
		t.param.Dst = t.pausedParam
		t.pausedParam = nil
	}

	if t.param.Dst.IsCloud() {
		if !t.wasPaused {
			srcName, isFile := files.GetFileNameFromPath(src.Path)
			srcPath := srcName
			if !isFile {
				srcPath += "/"
			}

			tempParam = &models.FileParam{
				Owner:    user,
				FileType: common.Cache,
				Extend:   global.CurrentNodeName,
				Path:     common.DefaultSyncUploadToCloudTempPath + "/" + t.id + "/" + srcPath,
			}
			dst = tempParam
		} else {
			dst = t.pausedParam
			t.pausedParam = nil
		}
	} else {
		dst = t.param.Dst
	}

	if share == 1 {
		src.Owner = srcOwner
		dst.Owner = dstOwner
	}

	klog.Infof("[Task] Id: %s, start, downloadFormSync, phase: %d/%d, user: %s, action: %s, src: %s, dst: %s, psrc: %s, pdst: %s", t.id, t.currentPhase, t.totalPhases, user, action, common.ToJson(src), common.ToJson(dst), common.ToJson(t.param.Src), common.ToJson(t.param.Dst))

	// check local free space
	srcTotalSize, err := t.GetFromSyncFileCount("size") // file and dir can both use this
	if err != nil {
		klog.Errorf("[Task] Id: %s, getFromSyncFileCount error: %v", t.id, err)
		return err
	}

	dstSize, err := cmd.GetSpaceSize(dst)
	if err != nil {
		klog.Errorf("get posix free space size error: %v", err)
		return err
	}

	klog.Infof("[Task] Id: %s, check posix space, syncSize: %d, posixSize: %d", t.id, srcTotalSize, dstSize)

	requiredSpace, ok := common.IsDiskSpaceEnough(dstSize, srcTotalSize)
	if ok {
		return fmt.Errorf("not enough free space on disk, required: %s, available: %s", common.FormatBytes(requiredSpace), common.FormatBytes(dstSize))
	}

	t.resetProgressZero()
	t.updateTotalSize(srcTotalSize)

	defer func() {
		klog.Infof("[Task] Id: %s, defer, error: %v", t.id, err)
	}()

	_, isFile := t.param.Src.IsFile()
	if isFile {
		err = t.DownloadFileFromSync(src, dst, true)
		if err != nil {
			klog.Errorf("[Task] Id: %s, downloadFileFromSync error: %v", t.id, err)
		}
	} else {
		err = t.DownloadDirFromSync(src, dst, true)

		if err != nil {
			klog.Errorf("[Task] Id: %s, downloadDirFromSync error: %v", t.id, err)
		}
	}

	if err != nil {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return searpc.SyncConnectionFailedError(err)
	}

	if !t.param.Dst.IsCloud() && t.param.Action == common.ActionMove {
		if err = seahub.HandleDelete(t.param.Src); err != nil {
			return err
		}
	}

	t.updateProgress(100, 0)

	if t.param.Dst.IsCloud() {
		klog.Infof("[Task] Id: %s, download from sync done! temp: %s", t.id, common.ToJson(t.param.Temp))
	}

	return nil
}

/**
 * ~ UploadToSync
 */
func (t *Task) UploadToSync() error {
	// posix > sync; cloud > sync
	var cmd = rclone.Command
	var user = t.param.Owner
	var action = t.param.Action
	var share = t.param.Share
	var srcOwner = t.param.SrcOwner
	var dstOwner = t.param.DstOwner

	var src *models.FileParam
	var dst *models.FileParam

	if t.param.Src.IsCloud() {
		if t.param.Temp == nil {
			return fmt.Errorf("[Task] Id: %s, param invalid", t.id)
		} else {
			src = t.param.Temp
		}
	} else {
		src = t.param.Src
	}

	if t.wasPaused && t.pausedPhase == t.totalPhases {
		if t.pausedParam == nil {
			return fmt.Errorf("[Task] Id: %s, pause param invalid", t.id)
		}
		dst = t.pausedParam
	} else {
		dst = t.param.Dst
	}

	if share == 1 {
		src.Owner = srcOwner
		dst.Owner = dstOwner
	}

	t.resetProgressZero()

	posixSize, err := cmd.GetFilesSize(src)
	if err != nil {
		klog.Errorf("get posix size error: %v", err)
		return err
	}

	t.updateTotalSize(posixSize)

	_, isFile := src.IsFile()

	klog.Infof("[Task] Id: %s, start, uploadToSync, phase: %d/%d, user: %s, action: %s, src: %s, dst: %s", t.id, t.currentPhase, t.totalPhases, user, action, common.ToJson(src), common.ToJson(dst))

	if isFile {
		err = t.UploadFileToSync(src, dst)
		if err != nil {
			klog.Errorf("[Task] Id: %s, uploadFileToSync error: %v", t.id, err)
		}
	} else {
		err = t.UploadDirToSync(src, dst, true)
		if err != nil {
			klog.Errorf("[Task] Id: %s, uploadDirToSync error: %v", t.id, err)
		}
	}

	if err != nil {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		t.pausedSyncMkdir = true
		return err
	}

	klog.Infof("[Task] Id: %s done! error: %v", t.id, err)

	if t.param.Src.IsCloud() {
		if e := cmd.ClearTaskCaches(src, t.id); e != nil {
			klog.Errorf("[Task] Id: %s, clear src error: %v", t.id, e)
		}
	}

	if t.param.Action == common.ActionMove {
		srcUri := ""
		srcUri, err = t.param.Src.GetResourceUri()
		if err != nil {
			return err
		}
		srcFullPath := srcUri + t.param.Src.Path
		err = os.RemoveAll(srcFullPath)
		if err != nil {
			return err
		}
	}

	t.updateProgress(100, 0)

	return nil
}

/**
 * ~ SyncCopy
 */
func (t *Task) SyncCopy() error {
	var user = t.param.Owner
	var action = t.param.Action
	var src = t.param.Src
	var dst = t.param.Dst
	var share = t.param.Share
	var srcOwner = t.param.SrcOwner
	var dstOwner = t.param.DstOwner

	if share == 1 {
		src.Owner = srcOwner
		dst.Owner = dstOwner
	}

	klog.Infof("[Task] Id: %s, start, sync, user: %s, action: %s, src: %s, dst: %s",
		t.id, user, action, common.ToJson(src), common.ToJson(dst))

	totalSize, err := t.GetFromSyncFileCount("size") // file and dir can both use this
	if err != nil {
		klog.Errorf("DownloadFromSync - GetFromSyncFileCount - %v", err)
		return err
	}

	klog.Infof("[Task] Id: %s, start, syncCopy, phase: %d/%d, user: %s, action: %s, src: %s, dst: %s", t.id, t.currentPhase, t.totalPhases, user, action, common.ToJson(t.param.Src), common.ToJson(t.param.Dst))

	t.updateTotalSize(totalSize)

	newDstPath := AddVersionSuffix(dst.Path, dst, strings.HasSuffix(dst.Path, "/"), "")
	dst.Path = newDstPath

	err = t.DoSyncCopy(src, dst, true)
	if err != nil {
		klog.Errorf("DownloadFromSync - DoSyncCopy - %v", err)
		return err
	}
	return nil
}

func (t *Task) GetFromSyncFileCount(countType string) (int64, error) {
	var count int64
	repoId := t.param.Src.Extend
	parentDir, filename := filepath.Split(t.param.Src.Path)
	if !strings.HasSuffix(parentDir, "/") {
		parentDir += "/"
	}

	firstFileParam := &models.FileParam{
		Owner:    t.param.Src.Owner,
		FileType: t.param.Src.FileType,
		Extend:   repoId,
		Path:     parentDir,
	}

	queue := []*models.FileParam{firstFileParam}

	for len(queue) > 0 {
		curFileParam := queue[0]
		queue = queue[1:]

		curDirInfoRes, err := seahub.HandleGetRepoDir(curFileParam)
		if err != nil {
			return 0, err
		}
		if curDirInfoRes == nil {
			return 0, errors.New("folder not found")
		}

		var curDirInfo map[string]interface{}
		if err = json.Unmarshal(curDirInfoRes, &curDirInfo); err != nil {
			return 0, err
		}

		direntInterfaceList, ok := curDirInfo["dirent_list"].([]interface{})
		if !ok {
			klog.Errorf("Invalid dirent_list format at path: %s", curFileParam.Path)
			return 0, fmt.Errorf("invalid directory format")
		}

		direntList := make([]map[string]interface{}, 0)
		for _, item := range direntInterfaceList {
			if dirent, ok := item.(map[string]interface{}); ok {
				direntList = append(direntList, dirent)
			} else {
				klog.Errorf("Invalid dirent item type at path: %s", curFileParam.Path)
				return 0, fmt.Errorf("invalid directory item type")
			}
		}

		for _, dirent := range direntList {
			name, _ := dirent["name"].(string)
			objType, _ := dirent["type"].(string)

			if filename != "" && name == filename {
				if countType == "size" {
					size, _ := dirent["size"].(float64)
					count += int64(size)
				} else {
					count++
				}
				return count, nil
			} else if filename == "" {
				if objType == "dir" {
					dirPath, _ := dirent["path"].(string)
					if dirPath != "/" {
						dirPath += "/"
					}
					appendFileParam := &models.FileParam{
						Owner:    t.param.Src.Owner,
						FileType: t.param.Src.FileType,
						Extend:   repoId,
						Path:     dirPath,
					}
					queue = append(queue, appendFileParam)
				} else {
					if countType == "size" {
						size, _ := dirent["size"].(float64)
						count += int64(size)
					} else {
						count++
					}
				}
			}
		}
	}

	return count, nil
}

func (t *Task) DownloadDirFromSync(src, dst *models.FileParam, root bool) error {
	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		return ctxErr
	}

	klog.Infof("[Task] Id: %s, dir, src: %s, dst: %s", t.id, common.ToJson(src), common.ToJson(dst))

	var downloadPath string

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return err
	}

	downloadPath = dstUri + dst.Path

	dirInfoRes, err := seahub.HandleGetRepoDir(src)
	if err != nil {
		return err
	}
	if dirInfoRes == nil {
		return errors.New("folder not found")
	}
	var dirInfo map[string]interface{}
	if err = json.Unmarshal(dirInfoRes, &dirInfo); err != nil {
		return err
	}

	if root {
		if !t.param.Dst.IsCloud() {
			if !t.wasPaused {
				downloadPath = AddVersionSuffix(downloadPath, dst, true, "")
			}

		}
	}

	if !common.PathExists(downloadPath) {
		mode := seahub.SyncPermToMode(dirInfo["user_perm"].(string))
		if err = files.MkdirAllWithChown(nil, downloadPath, mode, true, 1000, 1000); err != nil {
			klog.Errorf("[Task] Id: %s, mkdir %s failed, error: %v", t.id, downloadPath, err)
			return err
		}
	}

	var fdstBase = strings.TrimPrefix(downloadPath, dstUri)

	if root {
		t.param.Temp = &models.FileParam{
			Owner:    dst.Owner,
			FileType: dst.FileType,
			Extend:   dst.Extend,
			Path:     fdstBase,
		}
	}

	klog.Infof("[Task] Id: %s, dstFullPath: %s, fdstBase: %s, dstUri: %s", t.id, downloadPath, fdstBase, dstUri)

	direntInterfaceList, ok := dirInfo["dirent_list"].([]interface{})
	if !ok {
		klog.Errorf("Invalid dirent_list format at path: %s", src.Path)
		return fmt.Errorf("invalid directory format")
	}

	direntList := make([]map[string]interface{}, 0)
	for _, item := range direntInterfaceList {
		if dirent, ok := item.(map[string]interface{}); ok {
			direntList = append(direntList, dirent)
		} else {
			klog.Errorf("Invalid dirent item type at path: %s", src.Path)
			return fmt.Errorf("invalid directory item type")
		}
	}

	for _, item := range direntList {
		if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
			return ctxErr
		}

		fsrc := filepath.Join(src.Path, item["name"].(string))
		fdst := filepath.Join(fdstBase, item["name"].(string))
		fsrcFileParam := &models.FileParam{
			Owner:    src.Owner,
			FileType: src.FileType,
			Extend:   src.Extend,
			Path:     fsrc,
		}
		fdstFileParam := &models.FileParam{
			Owner:    dst.Owner,
			FileType: dst.FileType,
			Extend:   dst.Extend,
			Path:     fdst,
		}

		if item["type"].(string) == "dir" {
			err = t.DownloadDirFromSync(fsrcFileParam, fdstFileParam, false)
			if err != nil {
				return err
			}
		} else {
			err = t.DownloadFileFromSync(fsrcFileParam, fdstFileParam, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Task) DownloadFileFromSync(src, dst *models.FileParam, root bool) error {
	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		return ctxErr
	}

	klog.Infof("[Task] Id: %s, download dst: %s", t.id, common.ToJson(dst))

	var downloadPath, downloadFilePath string

	srcFileName, _ := files.GetFileNameFromPath(src.Path)
	_ = srcFileName
	dstFileName, _ := files.GetFileNameFromPath(dst.Path)

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return err
	}

	downloadPath = dstUri + filepath.Dir(dst.Path)

	fileInfo := seahub.GetFileInfo(src.Extend, src.Path)
	fileSize := fileInfo["size"].(int64)

	dlUrlRaw, err := seahub.ViewLibFile(src, "dl")
	if err != nil {
		return err
	}
	dlUrl := "http://127.0.0.1:80/" + string(dlUrlRaw)

	request, err := http.NewRequestWithContext(t.ctx, "GET", dlUrl, nil)
	if err != nil {
		return fmt.Errorf("url: %s, error: %v", dlUrl, err)
	}

	client := http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed，status code：%d", response.StatusCode)
	}

	contentDisposition := response.Header.Get("Content-Disposition")
	if contentDisposition == "" {
		return fmt.Errorf("unrecognizable response format")
	}

	if !t.wasPaused {
		downloadFilePath = AddVersionSuffix(downloadPath+"/"+dstFileName, dst, false, "")
	} else {
		downloadFilePath = filepath.Join(downloadPath, dstFileName)
	}

	if !common.PathExists(downloadPath) {
		if err = files.MkdirAllWithChown(nil, filepath.Dir(downloadFilePath), 0755, true, 1000, 1000); err != nil {
			klog.Errorf("[Task] Id: %s, mkdir %s error: %v", t.id, downloadPath, err)
			return fmt.Errorf("failed to create parent directories: %v", err)
		}
	}

	var downloadFilePathShortName = t.manager.formatFilePathWithoutTask(downloadFilePath, t.id)

	dstFileStat, err := os.Stat(downloadFilePath)
	if dstFileStat != nil && dstFileStat.Size() == fileSize {
		var p = int(float64(t.transfer+fileSize) / float64(t.totalSize) * 100)
		t.updateProgress(p, fileSize)
		klog.Infof("[Task] Id: %s, %s exists, skip", t.id, downloadFilePathShortName)
		return nil
	}

	klog.Infof("[Task] Id: %s, download %s (%s)", t.id, downloadFilePathShortName, common.FormatBytes(fileSize))

	dstFile, err := os.OpenFile(downloadFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		klog.Errorf("[Task] Id: %s, open file error: %v", t.id, err)
		return err
	}
	defer dstFile.Close()

	if err = files.Chown(nil, downloadFilePath, 1000, 1000); err != nil {
		klog.Errorf("[Task] Id: %s, chown file error: %v", t.id, err)
		return err
	}

	if root {

		t.param.Temp = &models.FileParam{
			Owner:    dst.Owner,
			FileType: dst.FileType,
			Extend:   dst.Extend,
			Path:     common.DefaultSyncUploadToCloudTempPath + "/" + t.id + strings.TrimPrefix(downloadFilePath, downloadPath),
		}
	}

	var reader io.Reader = response.Body
	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			klog.Errorf("[Task] Id: %s, gzipReader error: %v", t.id, err)
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	var buf = make([]byte, 32*1024)
	var transferred int64
	var ticker = time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			klog.Infof("[Task] Id: %s, canceled", t.id)
			return t.ctx.Err()
		case <-ticker.C:
			klog.Infof("[Task] Id: %s, download progress %d (%s/%s)", t.id, t.progress, common.FormatBytes(t.transfer), common.FormatBytes(t.totalSize))
		default:
		}

		nr, er := reader.Read(buf)
		if nr > 0 {
			if _, err := dstFile.Write(buf[:nr]); err != nil {
				return err
			}
			transferred += int64(nr)
			progress := int(float64(t.transfer+int64(nr)) / float64(t.totalSize) * 100)

			if progress > 100 {
				progress = 100
			}

			t.updateProgress(progress, int64(nr))
		}

		if er != nil {
			if er == io.EOF {
				break
			}
			return er
		}
	}

	return nil
}

func (t *Task) GetToSyncFileCount(countType string) (int64, error) {
	uri, err := t.param.Src.GetResourceUri()
	if err != nil {
		return 0, err
	}
	newSrc := uri + t.param.Src.Path

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

func (t *Task) UploadDirToSync(src, dst *models.FileParam, root bool) error {
	srcUri, err := src.GetResourceUri()
	if err != nil {
		return err
	}
	srcFullPath := srcUri + src.Path

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return err
	}
	dstFullPath := dstUri + dst.Path

	var fdstBase = strings.TrimPrefix(dstFullPath, dstUri)

	if root {
		if !t.wasPaused || !t.pausedSyncMkdir {
			dstFullPath = AddVersionSuffix(dstFullPath, dst, true, "")
			fdstBase = strings.TrimPrefix(dstFullPath, dstUri)
		}
	}

	var created bool = true
	var getUploadDirParam = &models.FileParam{
		Owner:    dst.Owner,
		FileType: dst.FileType,
		Extend:   dst.Extend,
		Path:     fdstBase,
	}
	dirId, _ := seahub.GetUploadDir(getUploadDirParam)
	if dirId != "" {
		created = false
	}

	if created {
		res, err := seahub.HandleDirOperation(src.Owner, dst.Extend, fdstBase, "", "mkdir", true)
		if err != nil {
			klog.Errorf("Sync create error: %v, path: %s", err, dst.Path)
			return err
		}
		klog.Infof("Sync create success, result: %s, path: %s", string(res), dst.Path)
	}

	dir, _ := os.Open(srcFullPath)
	obs, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var errs []error

	for _, obj := range obs {
		if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
			return ctxErr
		}

		fsrc := filepath.Join(src.Path, obj.Name())
		fdst := filepath.Join(fdstBase, obj.Name())

		fsrcFileParam := &models.FileParam{
			Owner:    src.Owner,
			FileType: src.FileType,
			Extend:   src.Extend,
			Path:     fsrc,
		}
		fdstFileParam := &models.FileParam{
			Owner:    dst.Owner,
			FileType: dst.FileType,
			Extend:   dst.Extend,
			Path:     fdst,
		}

		if obj.IsDir() {
			// Create sub-directories, recursively.
			err = t.UploadDirToSync(fsrcFileParam, fdstFileParam, false)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = t.UploadFileToSync(fsrcFileParam, fdstFileParam)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	var errString string
	for _, err = range errs {
		errString += err.Error() + "\n"
	}

	if errString != "" {
		return errors.New(errString)
	}
	return nil
}

func (t *Task) UploadFileToSync(src, dst *models.FileParam) error {
	srcUri, err := src.GetResourceUri()
	if err != nil {
		return err
	}
	srcFullPath := srcUri + src.Path

	srcinfo, err := os.Stat(srcFullPath)
	if err != nil {
		return err
	}
	diskSize := srcinfo.Size()

	_, isUploadFile := t.param.Src.IsFile()

	if isUploadFile && !t.wasPaused {
		newDstPath, err := t.manager.GetSyncDupName(t.id, src, dst, t.param.Src, t.param.Dst)
		if err != nil {
			return err
		}
		dst.Path = newDstPath

		klog.Infof("[Task] Id: %s, generate dup name: %s, dst: %s", t.id, newDstPath, common.ToJson(dst))
	}

	if t.wasPaused {

		getFileId, _ := seahub.GetUploadFile(dst)
		if getFileId != "" {
			klog.Infof("[Task] Id: %s, upload file %s exists, skip", t.id, dst.Path)
			_, _, right := t.CalculateSyncProgressRange(diskSize)
			t.updateProgress(right, 0)
			return nil
		}
	}

	left, _, right := t.CalculateSyncProgressRange(diskSize)

	prefix, filename := filepath.Split(dst.Path)
	prefix = strings.TrimPrefix(prefix, "/")

	extension := path.Ext(filename)
	mimeType := "application/octet-stream"
	if extension != "" {
		mimeType = mime.TypeByExtension(extension)
	}

	uploadParam := &models.FileParam{
		Owner:    dst.Owner,
		FileType: dst.FileType,
		Extend:   dst.Extend,
		Path:     filepath.Dir(dst.Path),
	}
	uploadLink, err := seahub.GetUploadLink(uploadParam, "api", false, false)
	if err != nil {
		return err
	}
	uploadLink = strings.Trim(uploadLink, "\"")

	targetURL := "http://127.0.0.1:80" + uploadLink + "?ret-json=1"
	klog.Infoln(targetURL)

	srcFile, err := os.Open(srcFullPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	chunkSize := int64(8 * 1024 * 1024) // 8MB
	totalChunks := (diskSize + chunkSize - 1) / chunkSize
	identifier := seahub.GenerateUniqueIdentifier(common.EscapeAndJoin(filename, "/"))

	var chunkStart int64 = 0
	for chunkNumber := int64(1); chunkNumber <= totalChunks; chunkNumber++ {
		status, _, _, _ := t.GetProgress()
		if status != "running" && status != "pending" {
			return nil
		}

		percent := (chunkNumber * 100) / totalChunks
		rangeSize := right - left
		mappedProgress := left + int((percent*int64(rangeSize))/100)
		finalProgress := mappedProgress
		if finalProgress < left {
			finalProgress = left
		} else if finalProgress > right {
			finalProgress = right
		}
		klog.Infof("finalProgress:%d", finalProgress)

		offset := (chunkNumber - 1) * chunkSize
		chunkData := make([]byte, chunkSize)
		bytesRead, err := srcFile.ReadAt(chunkData, offset)
		if err != nil && err != io.EOF {
			return err
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		writer.WriteField("resumableChunkNumber", strconv.FormatInt(chunkNumber, 10))
		writer.WriteField("resumableChunkSize", strconv.FormatInt(chunkSize, 10))
		writer.WriteField("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		writer.WriteField("resumableTotalSize", strconv.FormatInt(diskSize, 10))
		writer.WriteField("resumableType", mimeType)
		writer.WriteField("resumableIdentifier", identifier)
		writer.WriteField("resumableFilename", filename)
		writer.WriteField("resumableRelativePath", filename)
		writer.WriteField("resumableTotalChunks", strconv.FormatInt(totalChunks, 10))
		writer.WriteField("parent_dir", "/"+prefix)

		part, err := writer.CreateFormFile("file", common.EscapeAndJoin(filename, "/"))
		if err != nil {
			klog.Errorln("Create Form File error: ", err)
			return err
		}

		_, err = part.Write(chunkData[:bytesRead])
		if err != nil {
			klog.Errorln("Write Chunk Data error: ", err)
			return err
		}

		err = writer.Close()
		if err != nil {
			klog.Errorln("Write Close error: ", err)
			return err
		}

		request, err := http.NewRequest("POST", targetURL, body)
		if err != nil {
			klog.Errorln("New Request error: ", err)
			return err
		}

		request.Header = make(http.Header)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Content-Disposition", "attachment; filename=\""+common.EscapeAndJoin(filename, "/")+"\"")
		request.Header.Set("Content-Range", "bytes "+strconv.FormatInt(chunkStart, 10)+"-"+strconv.FormatInt(chunkStart+int64(bytesRead)-1, 10)+"/"+strconv.FormatInt(diskSize, 10))
		chunkStart += int64(bytesRead)

		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		maxRetries := 3
		var response *http.Response
		special := false

		for retry := 0; retry < maxRetries; retry++ {
			var req *http.Request
			var err error

			if retry == 0 {
				req, err = http.NewRequest(request.Method, request.URL.String(), request.Body)
				if err != nil {
					klog.Warningf("create request error: %v", err)
					continue
				}
				req.Header = make(http.Header)
				for k, s := range request.Header {
					req.Header[k] = s
				}
			} else {
				// newBody begin
				offset = (chunkNumber - 1) * chunkSize
				chunkData = make([]byte, chunkSize)
				bytesRead, err = srcFile.ReadAt(chunkData, offset)
				if err != nil && err != io.EOF {
					return err
				}

				newBody := &bytes.Buffer{}
				writer = multipart.NewWriter(newBody)

				writer.WriteField("resumableChunkNumber", strconv.FormatInt(chunkNumber, 10))
				writer.WriteField("resumableChunkSize", strconv.FormatInt(chunkSize, 10))
				writer.WriteField("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
				writer.WriteField("resumableTotalSize", strconv.FormatInt(diskSize, 10))
				writer.WriteField("resumableType", mimeType)
				writer.WriteField("resumableIdentifier", identifier)
				writer.WriteField("resumableFilename", filename)
				writer.WriteField("resumableRelativePath", filename)
				writer.WriteField("resumableTotalChunks", strconv.FormatInt(totalChunks, 10))
				writer.WriteField("parent_dir", "/"+prefix)

				part, err = writer.CreateFormFile("file", common.EscapeAndJoin(filename, "/"))
				if err != nil {
					klog.Errorln("Create Form File error: ", err)
					return err
				}

				_, err = part.Write(chunkData[:bytesRead])
				if err != nil {
					klog.Errorln("Write Chunk Data error: ", err)
					return err
				}

				err = writer.Close()
				if err != nil {
					klog.Errorln("Write Close error: ", err)
					return err
				}

				if err != nil {
					klog.Warningf("generate body error: %v", err)
					continue
				}
				// newBody end

				req, err = http.NewRequest(request.Method, request.URL.String(), newBody)
				if err != nil {
					klog.Warningf("create request error: %v", err)
					continue
				}
				req.Header = make(http.Header)
				for k, s := range request.Header {
					req.Header[k] = s
				}
			}

			response, err = client.Do(req)
			klog.Infoln("Do Request (attempt", retry+1, ")")

			if err != nil {
				klog.Warningf("request error (attempt %d): %v", retry+1, err)

				if chunkNumber == totalChunks {
					if strings.Contains(err.Error(), "context deadline exceeded (Client.Timeout exceeded while awaiting headers)") {
						const gb = 1024 * 1024 * 1024
						additionalBlocks := diskSize / (10 * gb)
						totalBubble := 15*time.Second + time.Duration(additionalBlocks)*15*time.Second
						klog.Infof("Waiting %ds for seafile to complete", int(totalBubble.Seconds()))
						time.Sleep(totalBubble)
						special = true
						if response != nil && response.Body != nil {
							response.Body.Close()
						}
						klog.Infof("Waiting for seafile to complete huge file done!")
						break
					}
				}

				if response != nil && response.Body != nil {
					bodyBytes, err := io.ReadAll(response.Body)
					if err != nil {
						klog.Warningf("read body error: %v", err)
					} else {
						bodyString := string(bodyBytes)
						klog.Infof("error response: %s", bodyString)

						response.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					}
				} else {
					klog.Infof("got an empty error response")
				}

				if retry < maxRetries-1 {
					waitTime := time.Duration(1<<uint(retry)) * time.Second
					klog.Warningf("Retrying in %v...", waitTime)
					time.Sleep(waitTime)
				}
				continue
			}

			if response.StatusCode == http.StatusOK {
				break
			}

			klog.Warningf("non-200 status: %s (attempt %d)", response.Status, retry+1)

			if response.Body != nil {
				response.Body.Close()
			}

			if retry < maxRetries-1 {
				waitTime := time.Duration(1<<uint(retry)) * time.Second
				klog.Warningf("Retrying in %v...", waitTime)
				time.Sleep(waitTime)
			}
		}

		if !special {
			if response == nil || response.StatusCode != http.StatusOK {
				statusCode := http.StatusInternalServerError
				statusMsg := "request failed after retries"

				if response != nil {
					statusCode = response.StatusCode
					statusMsg = response.Status
					if response.Body != nil {
						defer response.Body.Close()
					}
				}

				klog.Warningf("%d, %s after %d attempts", statusCode, statusMsg, maxRetries)
				return searpc.SyncConnectionFailedError(fmt.Errorf("%d, %s after %d attempts", statusCode, statusMsg, maxRetries))
			}
			defer response.Body.Close()

			// Read the response body as a string
			postBody, err := io.ReadAll(response.Body)
			klog.Infoln("ReadAll")
			if err != nil {
				klog.Errorln("ReadAll error: ", err)
				return err
			}

			klog.Infoln("Status Code: ", response.StatusCode)
			if response.StatusCode != http.StatusOK {
				klog.Infoln(string(postBody))
				return fmt.Errorf("file upload failed, status code: %d", response.StatusCode)
			}
		}

		klog.Infof("Chunk %d/%d from of bytes %d-%d/%d successfully transferred.", chunkNumber, totalChunks, chunkStart, chunkStart+int64(bytesRead)-1, diskSize)
		t.updateProgress(finalProgress, int64(bytesRead))

		if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
			return ctxErr
		}

		time.Sleep(150 * time.Millisecond)
	}
	klog.Infoln("upload file to sync success!")

	t.updateProgress(right, 0)

	return nil
}

func (t *Task) CalculateSyncProgressRange(currentFileSize int64) (left, mid, right int) {
	_, progress, transferred, totalFileSize := t.GetProgress()

	klog.Infof("taskProgress=%d, currentFileSize=%d, transferred=%d, totalFileSize=%d",
		progress, currentFileSize, transferred, totalFileSize)

	if totalFileSize <= 0 {
		return 0, 0, 0
	}
	if progress >= 99 {
		return 99, 99, 99
	}

	sum := transferred + currentFileSize
	if sum > totalFileSize {
		sum = totalFileSize
	}

	right = int(math.Floor((float64(sum) / float64(totalFileSize)) * 100))

	if right > 99 {
		right = 99
	}

	left = progress
	if left > right {
		left = right
	}

	mid = (left + right) / 2

	return left, mid, right
}

func (t *Task) DoSyncCopy(src, dst *models.FileParam, root bool) error {
	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return ctxErr
	}

	go t.SimulateProgress(0, 99, 50000000)

	var err error
	srcParentDir := filepath.Dir(strings.TrimSuffix(src.Path, "/"))
	srcDirents := []string{filepath.Base(strings.TrimSuffix(src.Path, "/"))}
	dstParentDir := filepath.Dir(strings.TrimSuffix(dst.Path, "/"))
	if t.param.Action == common.ActionCopy {
		_, err = seahub.HandleBatchCopy(src.Owner, src.Extend, srcParentDir, srcDirents, dst.Extend, dstParentDir)
		if err != nil {
			// return err
			klog.Errorf("[Task] Id: %s, batch copy error: %v", t.id, err)
		}
	} else {
		_, err = seahub.HandleBatchMove(src.Owner, src.Extend, srcParentDir, srcDirents, dst.Extend, dstParentDir)
		if err != nil {
			klog.Errorf("[Task] Id: %s, batch move error: %v", t.id, err)
		}
	}

	if err != nil {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return err
	}

	_, _, _, size := t.GetProgress()
	t.updateProgress(100, size)
	return err
}

func (t *Task) SimulateProgress(left, right int, speed int64) {
	startTime := time.Now()
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Simulate progress update
			usedTime := int(time.Now().Sub(startTime).Seconds())
			status, _, _, size := t.GetProgress()
			progress := MapProgressByTime(left, right, size, speed, usedTime)

			if status == "running" {
				t.updateProgress(progress, 0)
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func AddVersionSuffix(source string, fileParam *models.FileParam, isDir bool, uploadedFilename string) string {
	if strings.HasSuffix(source, "/") {
		source = strings.TrimSuffix(source, "/")
	}

	counter := 1
	dir, name := path.Split(source)
	_, ext := common.SplitNameExt(name)
	base := strings.TrimSuffix(name, ext)
	renamed := ""
	bubble := ""
	if fileParam.FileType == "sync" {
		bubble = " "
	}

	var err error
	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return ""
	}

	for {
		if fileParam.FileType == "sync" {
			if isDir {
				dirInfoRes, err := seahub.HandleGetRepoDir(fileParam)
				if err != nil {
					klog.Error(err)
					break
				}
				if dirInfoRes == nil {
					break
				}
			} else {
				fileInfo := seahub.GetFileInfo(fileParam.Extend, fileParam.Path)
				if fileInfo == nil {
					break
				}
				var objId = fileInfo["obj_id"].(string)
				if objId == "" {
					break
				}
			}
		} else {
			if _, err = os.Stat(source); err != nil {
				break
			}
		}
		if !isDir {
			renamed = fmt.Sprintf("%s%s(%d)%s", base, bubble, counter, ext)
		} else {
			renamed = fmt.Sprintf("%s%s(%d)", name, bubble, counter)
		}
		source = path.Join(dir, renamed)
		fileParam.Path = strings.TrimPrefix(source, uri)
		counter++
	}

	if isDir {
		source += "/"
	}

	fileParam.Path = strings.TrimPrefix(source, uri)

	return source
}

func MapProgressByTime(left, right int, size, speed int64, usedTime int) int {
	transferredBytes := int64(usedTime) * speed

	var progressPercentage int64
	if size > 0 {
		progress := transferredBytes * 10000 / size
		progressPercentage = progress / 100 // Keep all calculations in int64
	} else {
		progressPercentage = 0
	}

	if progressPercentage < 0 {
		progressPercentage = 0
	} else if progressPercentage > 100 {
		progressPercentage = 100
	}

	// Convert progressPercentage to int for the final mapping
	mappedProgress := left + (right-left)*int(progressPercentage)/100

	if mappedProgress < left {
		mappedProgress = left
	} else if mappedProgress > right {
		mappedProgress = right
	}

	return mappedProgress
}
