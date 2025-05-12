package postgres

import (
	"time"

	"k8s.io/klog/v2"
)

var STATUS_INACTIVE = 0
var STATUS_ACTIVE = 1

var PERMISSION_VIEWABLE = 1
var PERMISSION_DOWNLOADABLE = 2
var PERMISSION_UPLOADABLE = 4
var PERMISSION_DELETEABLE = 8
var PERMISSION_EDITABLE = 16
var PERMISSION_PERMISSIONS_EDITABLE = 32

var PERMISSION_READONLY = PERMISSION_VIEWABLE + PERMISSION_DOWNLOADABLE
var PERMISSION_READWRITE = PERMISSION_READONLY + PERMISSION_UPLOADABLE
var PERMISSION_MANAGEABLE = PERMISSION_READWRITE + PERMISSION_DELETEABLE + PERMISSION_EDITABLE + PERMISSION_PERMISSIONS_EDITABLE

// ShareLink represents the structure of the share_link table in the database
type ShareLink struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	LinkURL    string    `gorm:"type:text;not null;index:idx_share_links_link_url"`
	PathID     uint64    `gorm:"type:bigint;not null;index:idx_share_links_path_id"`
	Path       string    `gorm:"type:text;not null;index:idx_share_links_path"`
	PathMD5    string    `gorm:"type:varchar(32);not null;index:idx_share_links_path_md5"`
	Password   string    `gorm:"type:varchar(32);not null;index:idx_share_links_password"`
	OwnerID    string    `gorm:"type:text;not null"`
	OwnerName  string    `gorm:"type:text;not null"`
	Permission int       `gorm:"not null"`
	ExpireIn   uint64    `gorm:"not null"`
	ExpireTime time.Time `gorm:"not null;type:timestamptz"`
	Count      uint64    `gorm:"not null;default:0"`
	Status     int       `gorm:"not null"`
	CreateTime time.Time `gorm:"not null;type:timestamptz"`
	UpdateTime time.Time `gorm:"not null;type:timestamptz;autoUpdateTime"`
}

func createShareLinkTable() {
	// Automatically migrate the schema and create the table if it does not exist
	err := DBServer.AutoMigrate(&ShareLink{})
	if err != nil {
		klog.Errorf("Failed to migrate the database: %v", err)
	}

	// Optionally, you can create the index manually if you prefer
	// db.Model(&ShareLink{}).AddIndex("idx_link_url", "link_url")

	klog.Infoln("Database migration succeeded.")
}

func QueryShareLinks(path, ownerID string, status, page, limit *int) ([]ShareLink, error) {
	var shareLinks []ShareLink
	query := DBServer.Model(&ShareLink{}).Where("owner_id = ?", ownerID)

	if path != "" {
		query = query.Where("path = ?", path)
	}

	if status != nil {
		query = query.Where("status = ?", *status)
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

	if err := query.Find(&shareLinks).Error; err != nil {
		return nil, err
	}

	return shareLinks, nil
}
