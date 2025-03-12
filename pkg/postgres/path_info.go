package postgres

import (
	"k8s.io/klog/v2"
	"time"
)

var STATUS_PRIVATE = 0
var STATUS_PUBLIC = 1
var STATUS_DELETED = 2

// PathInfo represents the structure of the file table in the database
type PathInfo struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	Path       string    `gorm:"type:text;not null;index:idx_path_infos_path"`
	SrcType    string    `gorm:"type:varchar(10);not null"`
	MD5        string    `gorm:"type:varchar(32);not null;uniqueIndex:idx_path_infos_md5"`
	OwnerID    string    `gorm:"type:text;not null"`
	OwnerName  string    `gorm:"type:text;not null"`
	Status     int       `gorm:"not null"` // 0 = private, 1 = public, 2 = deleted
	CreateTime time.Time `gorm:"not null;type:timestamptz"`
	UpdateTime time.Time `gorm:"not null;type:timestamptz;autoUpdateTime"`
}

func createPathInfoTable() {
	// Migrate the schema, create the table if it does not exist
	err := DBServer.AutoMigrate(&PathInfo{})
	if err != nil {
		klog.Errorf("failed to migrate database: %v", err)
	} else {
		klog.Infoln("migrated database table")
	}

	// Optionally, you can create the index for the MD5 field explicitly,
	// but GORM should handle the uniqueIndex directive automatically.
	// If you need to create a functional index (which PostgreSQL supports),
	// you would need to do it manually via raw SQL, as GORM does not support
	// functional indexes directly.

	klog.Infoln("Database migration succeeded")
}

func QueryPathInfos(status *int, path string, md5 string, page, limit *int) ([]PathInfo, error) {
	klog.Infoln("status=", status, ", path=", path, ", path == empty?", path == "", ", md5=", md5, ", page=", page, ", limit=", limit)

	var pathInfos []PathInfo
	query := DBServer.Model(&PathInfo{})

	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if path != "" {
		query = query.Where("path = ?", path)
	}
	if md5 != "" {
		query = query.Where("md5 = ?", md5)
	}

	if limit != nil {
		var offset int
		if page != nil {
			offset = (*page - 1) * *limit
		} else {
			offset = 0
		}
		query = query.Offset(offset).Limit(*limit)
	}

	if err := query.Find(&pathInfos).Error; err != nil {
		return nil, err
	}

	return pathInfos, nil
}

func GetPathMapFromDB() (map[string]PathInfo, error) {
	var pathInfos []PathInfo
	if err := DBServer.Find(&pathInfos).Error; err != nil {
		return nil, err
	}

	pathMap := make(map[string]PathInfo)
	for _, info := range pathInfos {
		pathMap[info.Path] = info
	}

	return pathMap, nil
}
