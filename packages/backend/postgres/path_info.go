package postgres

import (
	"fmt"
	"log"
	"time"
)

// PathInfo represents the structure of the file table in the database
type PathInfo struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
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
		log.Fatalf("failed to migrate database: %v", err)
	} else {
		log.Println("migrated database table")
	}

	// Optionally, you can create the index for the MD5 field explicitly,
	// but GORM should handle the uniqueIndex directive automatically.
	// If you need to create a functional index (which PostgreSQL supports),
	// you would need to do it manually via raw SQL, as GORM does not support
	// functional indexes directly.

	log.Println("Database migration succeeded")
}

func QueryPathInfos(status *int, path *string, md5 *string, page, limit *int) ([]PathInfo, error) {
	fmt.Println("status=", status, ", path=", *path, ", path == empty?", *path == "", ", md5=", *md5, ", page=", page, ", limit=", limit)

	var pathInfos []PathInfo
	query := DBServer.Model(&PathInfo{}).Where("true")

	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if path != nil && *path != "" {
		query = query.Where("path = ?", *path)
	}
	if md5 != nil {
		query = query.Where("md5 = ?", *md5)
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
