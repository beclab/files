package upload

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"
	"mime/multipart"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

var (
	InfoSyncMap sync.Map
)

type FileMetaData struct {
	FileRelativePath string `json:"file_relative_path" form:"file_relative_path" binding:"required"`
	FileType         string `json:"file_type" form:"file_type" binding:"required"`
	FileSize         int64  `json:"file_size" form:"file_size" binding:"required"`
	StoragePath      string `json:"storage_path" form:"storage_path" binding:"required"`
	FullPath         string `json:"full_path"` // storage_path(must exist) + file_relative_path
}

type FileInfo struct {
	ID             string    `json:"id"`
	Offset         int64     `json:"offset"`
	LastUpdateTime time.Time `json:"-"`
	FileMetaData
}

type ResumableInfo struct {
	ResumableChunkNumber      int                   `json:"resumableChunkNumber" form:"resumableChunkNumber"`
	ResumableChunkSize        int64                 `json:"resumableChunkSize" form:"resumableChunkSize"`
	ResumableCurrentChunkSize int64                 `json:"resumableCurrentChunkSize" form:"resumableCurrentChunkSize"`
	ResumableTotalSize        int64                 `json:"resumableTotalSize" form:"resumableTotalSize"`
	ResumableType             string                `json:"resumableType" form:"resumableType"`
	ResumableIdentifier       string                `json:"resumableIdentifier" form:"resumableIdentifier"`
	ResumableFilename         string                `json:"resumableFilename" form:"resumableFilename"`
	ResumableRelativePath     string                `json:"resumableRelativePath" form:"resumableRelativePath"`
	ResumableTotalChunks      int                   `json:"resumableTotalChunks" form:"resumableTotalChunks"`
	ParentDir                 string                `json:"parent_dir" form:"parent_dir"`
	MD5                       string                `json:"md5,omitempty" form:"md5"`
	File                      *multipart.FileHeader `json:"file" form:"file" binding:"required"`
}

type FileInfoMgr struct {
}

func NewFileInfoMgr() *FileInfoMgr {
	return &FileInfoMgr{}
}

func (m *FileInfoMgr) Init(c *cron.Cron) {
	m.cronDeleteOldInfo(c)
}

func (m *FileInfoMgr) cronDeleteOldInfo(c *cron.Cron) {
	needStart := false
	if c == nil {
		c = cron.New()
		needStart = true
	}

	_, err := c.AddFunc("30 * * * *", func() {
		m.DeleteOldInfos()
	})
	if err != nil {
		klog.Warningf("AddFunc DeleteOldInfos err:%v", err)
	}

	if needStart {
		c.Start()
	}
}

func (m *FileInfoMgr) DeleteOldInfos() {
	InfoSyncMap.Range(func(key, value interface{}) bool {
		v := value.(FileInfo)
		klog.Infof("Key: %v, Value: %v\n", key, v)
		if time.Since(v.LastUpdateTime) > expireTime {
			klog.Infof("id %s expire del in map, stack:%s", key, debug.Stack())
			InfoSyncMap.Delete(key)
			for _, uploadsFile := range UploadsFiles {
				RemoveTempFileAndInfoFile(filepath.Base(uploadsFile), filepath.Dir(uploadsFile))
			}
		}
		return true
	})
}

func (m *FileInfoMgr) AddFileInfo(id string, info FileInfo) error {
	if id != info.ID {
		klog.Errorf("id:%s diff from v:%v", id, info)
		return fmt.Errorf("id:%s diff from v:%v", id, info)
	}

	info.LastUpdateTime = time.Now()
	InfoSyncMap.Store(id, info)

	return nil
}

func (m *FileInfoMgr) UpdateInfo(id string, info FileInfo) {
	if id != info.ID {
		klog.Errorf("id:%s diff from v:%v", id, info)
		return
	}

	info.LastUpdateTime = time.Now()
	InfoSyncMap.Store(id, info)
}

func (m *FileInfoMgr) DelFileInfo(id, tmpName, uploadsDir string) {
	InfoSyncMap.Delete(id)
	RemoveTempFileAndInfoFile(tmpName, uploadsDir)
}

func (m *FileInfoMgr) ExistFileInfo(id string) (bool, FileInfo) {
	value, ok := InfoSyncMap.Load(id)
	if ok {
		return ok, value.(FileInfo)
	}

	return false, FileInfo{}
}

func (m *FileInfoMgr) CheckTempFile(id, uploadsDir string) (bool, int64) {
	return PathExistsAndGetLen(filepath.Join(uploadsDir, id))
}
