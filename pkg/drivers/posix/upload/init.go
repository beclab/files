package upload

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"
	"os"
	"strconv"
	"strings"
	"time"
)

var FileInfoManager *FileInfoMgr = nil
var supportedFileTypes map[string]bool
var allowAllFileType bool
var limitedSize int64

var (
	UploadFileType    = "UPLOAD_FILE_TYPE"
	UploadLimitedSize = "UPLOAD_LIMITED_SIZE"
)

func Init(c *cron.Cron) {
	FileInfoManager = NewFileInfoMgr()
	FileInfoManager.Init(c)

	UploadsFiles.Range(func(_, value any) bool {
		cronDeleteOldFile(value.(string), c)
		return true
	})

	getEnvInfo()

}

// cronDeleteOldFile registers an hourly job that removes the upload-buffer
// file once it has aged past expireTime. The caller MUST pass a non-nil
// cron instance whose lifecycle is owned by InitCrontabs; otherwise the job
// would run on a private scheduler that no shutdown hook can stop.
func cronDeleteOldFile(filePath string, c *cron.Cron) {
	if c == nil {
		klog.Errorf("cronDeleteOldFile: nil cron passed for %s; refusing to spawn an orphan scheduler", filePath)
		return
	}

	_, err := c.AddFunc("30 * * * *", func() {
		subErr := DeleteIfFileExpired(filePath)
		if subErr != nil {
			klog.Warningf("DeleteOldFile %s, err:%v", filePath, subErr)
		}
	})
	if err != nil {
		klog.Warningf("AddFunc DeleteOldSubfolders err:%v", err)
	}
}

func DeleteIfFileExpired(filePath string) error {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %s", err.Error())
	}

	if !fileInfo.IsDir() {
		modTime := fileInfo.ModTime()
		if time.Since(modTime) >= expireTime {
			err := os.Remove(filePath)
			if err != nil {
				return fmt.Errorf("failed to delete file: %s", err.Error())
			}
			klog.Infof("Deleted file: %s\n", filePath)
		} else {
			klog.Infof("File %s is not expired (modified %v ago)\n", filePath, time.Since(modTime))
		}
	} else {
		return fmt.Errorf("provided path is a directory, not a file: %s", filePath)
	}

	return nil
}

func getEnvInfo() {
	var uploadFileType, uploadLimitedSize string

	uploadFileType = os.Getenv(UploadFileType)
	supportedFileTypes = make(map[string]bool)
	if uploadFileType == "" {
		allowAllFileType = true
	} else {
		fileTypes := strings.Split(uploadFileType, ",")
		for _, ft := range fileTypes {
			if ft == "*" {
				allowAllFileType = true
			}
			supportedFileTypes[ft] = true
		}
	}

	uploadLimitedSize = os.Getenv(UploadLimitedSize)

	size, err := strconv.ParseInt(uploadLimitedSize, 10, 64)
	if err != nil {
		klog.Errorf("uploadLimitedSize:%s parse int err:%v", uploadLimitedSize, err)
	}
	limitedSize = size
	if limitedSize <= 0 {
		limitedSize = DefaultMaxFileSize
	}

	klog.Infof("uploadFileType:%s, uploadLimitedSize:%s", uploadFileType, uploadLimitedSize)
	klog.Infof("allowAllFileType:%t supportedFileTypes:%v, limitedSize:%d", allowAllFileType, supportedFileTypes, limitedSize)
}
