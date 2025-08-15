package handlers

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"io"
	"math"
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
	klog.Infof("DownloadFromCloud - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, common.ToJson(c.src), common.ToJson(c.dst))

	return c.cloudPaste()

	// todo If the operation fails or the task is canceled, the target file needs to be deleted;
	// todo if it is a paste operation and the copy is successful, the source file needs to be deleted.
}

func (c *Handler) DownloadFromSync() error {
	totalSize, err := c.GetFromSyncFileCount("size") // file and dir can both use this
	if err != nil {
		klog.Errorf("DownloadFromSync - GetFromSyncFileCount - %v", err)
		return err
	}
	c.UpdateTotalSize(totalSize)

	_, isFile := c.src.IsFile()
	if isFile {
		err = c.DownloadFileFromSync(nil, nil)
		if err != nil {
			return err
		}
	} else {
		err = c.DownloadDirFromSync(nil, nil)
		if err != nil {
			return err
		}
	}
	if c.action == "move" {
		err = seahub.HandleDelete(c.src)
		if err != nil {
			return err
		}
	}
	_, _, transferred, _ := c.GetProgress()
	c.UpdateProgress(100, transferred)
	return nil
}

func (c *Handler) GetFromSyncFileCount(countType string) (int64, error) {
	var count int64
	repoId := c.src.Extend
	parentDir, filename := filepath.Split(c.src.Path)
	if !strings.HasSuffix(parentDir, "/") {
		parentDir += "/"
	}

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

		curDirInfoRes, err := seahub.HandleGetRepoDir(curFileParam)
		if err != nil || curDirInfoRes == nil {
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
					} else {
						count++
					}
				}
			}
		}
	}

	return count, nil
}

func (c *Handler) DownloadDirFromSync(src, dst *models.FileParam) error {
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

	dirInfoRes, err := seahub.HandleGetRepoDir(src)
	if err != nil || dirInfoRes == nil {
		return errors.New("folder not found")
	}
	var dirInfo map[string]interface{}
	if err = json.Unmarshal(dirInfoRes, &dirInfo); err != nil {
		return err
	}

	dstFullPath = AddVersionSuffix(dstFullPath, dst, true)

	mode := seahub.SyncPermToMode(dirInfo["user_perm"].(string))
	if err = files.MkdirAllWithChown(nil, dstFullPath, mode); err != nil {
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
			err = c.DownloadDirFromSync(fsrcFileParam, fdstFileParam)
			if err != nil {
				return err
			}
		} else {
			err = c.DownloadFileFromSync(fsrcFileParam, fdstFileParam)
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
			if isDir {
				dirInfoRes, err := seahub.HandleGetRepoDir(fileParam)
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

func (c *Handler) DownloadFileFromSync(src, dst *models.FileParam) error {
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

	dlUrlRaw, err := seahub.ViewLibFile(src, "dl")
	if err != nil {
		return err
	}
	dlUrl := "http://127.0.0.1:80/" + string(dlUrlRaw)

	request, err := http.NewRequestWithContext(c.ctx, "GET", dlUrl, nil)
	if err != nil {
		return err
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

	dstFullPath = AddVersionSuffix(dstFullPath, dst, false)

	if err = files.MkdirAllWithChown(nil, filepath.Dir(dstFullPath), 0755); err != nil {
		klog.Errorln(err)
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
				klog.Info("[" + time.Now().Format("2006-01-02 15:04:05") + "]" + fmt.Sprintf("downloaded from seafile %d/%d with progress %d", totalRead, diskSize, progress))
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
