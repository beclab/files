package handlers

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/files"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

func (c *Handler) DownloadFromFiles() error {
	return nil
}

func (c *Handler) DownloadFromCloud() error {
	klog.Infof("DownloadFromCloud - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	return c.cloudPaste()

	// todo If the operation fails or the task is canceled, the target file needs to be deleted;
	// todo if it is a paste operation and the copy is successful, the source file needs to be deleted.
}

func (c *Handler) DownloadFromSync() error {
	klog.Infof("~~~Copy Debug log: Download from sync begins!")
	header := make(http.Header)
	header.Add("X-Bfl-User", c.owner)

	totalSize, err := c.GetFromSyncFileCount(header, "size") // file and dir can both use this
	if err != nil {
		klog.Errorf("DownloadFromSync - GetFromSyncFileCount - %v", err)
		return err
	}
	klog.Infof("~~~Copy Debug log: DownloadFromSync - GetFromSyncFileCount - totalSize: %d", totalSize)
	c.UpdateTotalSize(totalSize)

	_, isFile := c.src.IsFile()
	if isFile {
		err = c.DownloadFileFromSync(header, nil, nil)
		if err != nil {
			return err
		}
	} else {
		err = c.DownloadDirFromSync(header, nil, nil)
		if err != nil {
			return err
		}
	}
	if c.action == "move" {
		err = seahub.HandleDelete(header, c.src)
		if err != nil {
			return err
		}
	}
	_, _, transferred, _ := c.GetProgress()
	c.UpdateProgress(100, transferred)
	return nil
}

func (c *Handler) GetFromSyncFileCount(header http.Header, countType string) (int64, error) {
	klog.Infof("～～～Copy Debug Log: Start GetFromSyncFileCount with repoId: %s, path: %s, countType: %s",
		c.src.Extend, c.src.Path, countType)

	var count int64
	repoId := c.src.Extend
	parentDir, filename := filepath.Split(c.src.Path)
	if !strings.HasSuffix(parentDir, "/") {
		parentDir += "/"
	}
	klog.Infof("～～～Copy Debug Log: Process path - parentDir: %s, filename: %s", parentDir, filename)

	firstFileParam := &models.FileParam{
		Owner:    c.src.Owner,
		FileType: c.src.FileType,
		Extend:   repoId,
		Path:     parentDir,
	}

	queue := []*models.FileParam{firstFileParam}

	for len(queue) > 0 {
		curFileParam := queue[0]
		queue = queue[1:]
		klog.Infof("～～～Copy Debug Log: Processing directory: %s", curFileParam.Path)

		curDirInfoRes, err := seahub.HandleGetRepoDir(header, curFileParam)
		if err != nil || curDirInfoRes == nil {
			klog.Errorf("～～～Copy Debug Log: Folder not found at path: %s, error: %v", curFileParam.Path, err)
			return 0, errors.New("folder not found")
		}

		var curDirInfo map[string]interface{}
		if err = json.Unmarshal(curDirInfoRes, &curDirInfo); err != nil {
			klog.Errorf("～～～Copy Debug Log: JSON unmarshal failed for path: %s, error: %v", curFileParam.Path, err)
			return 0, err
		}
		klog.Infof("~~~Copy Debug Log: curDirInfo: %v", curDirInfo)

		direntInterfaceList, ok := curDirInfo["dirent_list"].([]interface{})
		klog.Infof("~~~Copy Debug Log: direntInterfaceList: %v, ok: %v", direntInterfaceList, ok)
		if !ok {
			klog.Errorf("Invalid dirent_list format at path: %s", curFileParam.Path)
			return 0, fmt.Errorf("invalid directory format")
		}

		direntList := make([]map[string]interface{}, 0)
		for _, item := range direntInterfaceList {
			if dirent, ok := item.(map[string]interface{}); ok {
				klog.Infof("~~~Copy Debug Log: dirent: %v, ok: %v", dirent, ok)
				direntList = append(direntList, dirent)
			} else {
				klog.Errorf("Invalid dirent item type at path: %s", curFileParam.Path)
				return 0, fmt.Errorf("invalid directory item type")
			}
		}
		klog.Infof("~~~Copy Debug Log: len(direntList)=%d", len(direntList))

		for _, dirent := range direntList {
			klog.Infof("~~~Copy Debug Log: dirent: %v", dirent)
			name, _ := dirent["name"].(string)
			objType, _ := dirent["type"].(string)
			klog.Infof("~~~Copy Debug Log: name: %s, objType: %s, size: %v", name, objType, dirent["size"])

			if filename != "" && name == filename {
				klog.Infof("～～～Copy Debug Log: Found target file: %s, type: %s", name, objType)
				if countType == "size" {
					size, _ := dirent["size"].(float64)
					count += int64(size)
					klog.Infof("～～～Copy Debug Log: Add file size: %d", size)
				} else {
					count++
				}
				klog.Infof("～～～Copy Debug Log: Returning early with count: %d", count)
				return count, nil
			} else if filename == "" {
				if objType == "dir" {
					dirPath, _ := dirent["path"].(string)
					if dirPath != "/" {
						dirPath += "/"
					}
					klog.Infof("～～～Copy Debug Log: Enqueue subdirectory: %s", dirPath)
					appendFileParam := &models.FileParam{
						Owner:    c.src.Owner,
						FileType: c.src.FileType,
						Extend:   repoId,
						Path:     dirPath,
					}
					queue = append(queue, appendFileParam)
				} else {
					if countType == "size" {
						size, _ := dirent["size"].(float64)
						count += int64(size)
						klog.Infof("～～～Copy Debug Log: Add file size: %d for file: %s", size, name)
					} else {
						count++
						klog.Infof("～～～Copy Debug Log: Increment file count for: %s", name)
					}
				}
			}
		}
	}

	klog.Infof("～～～Copy Debug Log: Final count result: %d", count)
	return count, nil
}

func (c *Handler) DownloadDirFromSync(header http.Header, src, dst *models.FileParam) error {
	klog.Infof("~~~Copy Debug log: Download dir from sync begins!")
	select {
	case <-c.ctx.Done():
		return nil
	default:
	}

	if src == nil {
		src = c.src
	}
	if dst == nil {
		dst = c.dst
	}

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return err
	}
	dstFullPath := dstUri + dst.Path

	dirInfoRes, err := seahub.HandleGetRepoDir(header, src)
	if err != nil || dirInfoRes == nil {
		return errors.New("folder not found")
	}
	var dirInfo map[string]interface{}
	if err = json.Unmarshal(dirInfoRes, &dirInfo); err != nil {
		return err
	}

	dstFullPath = AddVersionSuffix(dstFullPath, dst, true)
	klog.Infof("~~~Debug log: dstFullPath after addversionsuffix=%s", dstFullPath)

	mode := seahub.SyncPermToMode(dirInfo["user_perm"].(string))
	if err = files.MkdirAllWithChown(files.DefaultFs, dstFullPath, mode); err != nil {
		klog.Errorln(err)
		return err
	}

	var fdstBase string = strings.TrimPrefix(dstFullPath, dstUri)

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
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		fsrc := filepath.Join(src.Path, item["name"].(string))
		fdst := filepath.Join(fdstBase, item["name"].(string))

		klog.Infof("~~~Debug log: fsrc=%s", fsrc)
		fsrcFileParam := &models.FileParam{
			Owner:    src.Owner,
			FileType: src.FileType,
			Extend:   src.Extend,
			Path:     fsrc,
		}
		klog.Infof("~~~Debug log: fdst=%s", fdst)
		fdstFileParam := &models.FileParam{
			Owner:    dst.Owner,
			FileType: dst.FileType,
			Extend:   dst.Extend,
			Path:     fdst,
		}

		if item["type"].(string) == "dir" {
			err = c.DownloadDirFromSync(header, fsrcFileParam, fdstFileParam)
			if err != nil {
				return err
			}
		} else {
			err = c.DownloadFileFromSync(header, fsrcFileParam, fdstFileParam)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func AddVersionSuffix(source string, fileParam *models.FileParam, isDir bool) string {
	if strings.HasSuffix(source, "/") {
		source = strings.TrimSuffix(source, "/")
	}

	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
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
			var header = http.Header{
				utils.REQUEST_HEADER_OWNER: []string{fileParam.Owner},
			}
			if isDir {
				dirInfoRes, err := seahub.HandleGetRepoDir(header, fileParam)
				if err != nil || dirInfoRes == nil {
					break
				}
			} else {
				fileInfo := seahub.GetFileInfo(fileParam.Extend, fileParam.Path)
				if fileInfo == nil {
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

func (c *Handler) DownloadFileFromSync(header http.Header, src, dst *models.FileParam) error {
	klog.Infof("~~~Copy Debug log: Download file from sync begins!")
	select {
	case <-c.ctx.Done():
		return nil
	default:
	}

	if src == nil {
		src = c.src
	}
	if dst == nil {
		dst = c.dst
	}

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return err
	}
	dstFullPath := dstUri + dst.Path

	fileInfo := seahub.GetFileInfo(src.Extend, src.Path)
	diskSize := fileInfo["size"].(int64)

	left, _, right := c.CalculateSyncProgressRange(diskSize) // mid may used for sync <-> cloud, reserved but not used here

	dlUrlRaw, err := seahub.ViewLibFile(header, src, "dl")
	if err != nil {
		return err
	}
	dlUrl := "http://127.0.0.1:80/" + string(dlUrlRaw)
	klog.Infof("~~~Debug log: dlURL=%s", dlUrl)

	request, err := http.NewRequestWithContext(c.ctx, "GET", dlUrl, nil)
	if err != nil {
		return err
	}

	request.Header = header.Clone()

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

	_, params, err := mime.ParseMediaType(contentDisposition)
	if err != nil {
		return err
	}
	filename := params["filename"]
	klog.Infof("~~~Debug log: filename=%s", filename)

	dstFullPath = AddVersionSuffix(dstFullPath, dst, false)
	klog.Infof("~~~Debug log: dstFullPath after addversionsuffix=%s", dstFullPath)

	if err := os.MkdirAll(filepath.Dir(dstFullPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %v", err)
	}

	dstFile, err := os.OpenFile(dstFullPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	var reader io.Reader = response.Body
	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	buf := make([]byte, 32*1024)
	var totalRead int64
	lastProgress := 0
	for {
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		nr, er := reader.Read(buf)
		if nr > 0 {
			if _, err := dstFile.Write(buf[:nr]); err != nil {
				return err
			}
			_, _, transferred, _ := c.GetProgress()
			totalRead += int64(nr)

			progress := int(float64(totalRead) / float64(diskSize) * 100)
			if progress > 100 {
				progress = 100
			}

			mappedProgress := left + (progress*(right-left))/100

			if mappedProgress < left {
				mappedProgress = left
			} else if mappedProgress > right {
				mappedProgress = right
			}

			if lastProgress != progress {
				klog.Info("~~~Debug log: [" + time.Now().Format("2006-01-02 15:04:05") + "]" + fmt.Sprintf("downloaded from seafile %d/%d with progress %d", totalRead, diskSize, progress))
				lastProgress = progress
			}
			c.UpdateProgress(mappedProgress, transferred+int64(nr))
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

func (c *Handler) CalculateSyncProgressRange(currentFileSize int64) (left, mid, right int) {
	_, progress, transferred, totalFileSize := c.GetProgress()

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
