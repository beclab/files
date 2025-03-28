package postgres

import (
	"files/pkg/drives"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/klog/v2"
	"time"
)

type PathList struct {
	Drive      string    `gorm:"type:varchar(20);not null;primaryKey"`
	Path       string    `gorm:"type:text;not null;primaryKey"`
	MTime      time.Time `gorm:"not null;type:timestamptz"`
	Status     int       `gorm:"not null"`
	CreateTime time.Time `gorm:"not null;type:timestamptz"`
	UpdateTime time.Time `gorm:"not null;type:timestamptz;autoUpdateTime"`
}

func createPathListTable() {
	// Migrate the schema, create the table if it does not exist
	err := DBServer.AutoMigrate(&PathList{})
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

func InitDrivePathList() {
	rs, err := drives.GetResourceService(drives.SrcTypeDrive)
	if err != nil {
		klog.Errorf("failed to get resource service: %v", err)
		return
	}

	err = rs.GeneratePathList(DBServer, ProcessDirectory)
	if err != nil {
		klog.Errorf("failed to generate drive path list: %v", err)
		return
	}

	if err = logPathList(); err != nil {
		fmt.Println("Error logging path list:", err)
	}
	return
}

func logPathList() error {
	var paths []PathList
	if err := DBServer.Find(&paths).Error; err != nil {
		return err
	}

	fmt.Println("Path List Entries:")
	for _, path := range paths {
		fmt.Printf("Drive: %s, Path: %s, MTime: %s, Status: %d, CreateTime: %s, UpdateTime: %s\n",
			path.Drive, path.Path, path.MTime, path.Status, path.CreateTime, path.UpdateTime)
	}

	return nil
}

func ProcessDirectory(db *gorm.DB, drive, path string, modTime time.Time) error {
	if drive == "Unknown" || drive == "error" || path == "" {
		// won't deal with these on purpose
		return nil
	}

	// Get the record from the database
	var record PathList
	if err := db.First(&record, "drive = ? AND path = ?", drive, path).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}

		// If record not found, insert a new one
		record = PathList{
			Drive:      drive,
			Path:       path,
			MTime:      modTime,
			Status:     0, // Set your default status
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
		}
		return db.Create(&record).Error
	}

	// If the record exists, check the modification time
	if record.MTime.After(modTime) {
		// Skip if the database time is after the file system time
		return nil
	}

	// Update the modification time in the database
	record.MTime = modTime
	record.UpdateTime = time.Now()
	return db.Save(&record).Error
}

func processFile(drive, path string, modTime time.Time) {
	// Implement file content indexing logic here
	fmt.Printf("Processing file: %s\n", path)
}

func batchUpdate(paths []PathList) error {
	// 使用 `clause.OnConflict` 来处理冲突
	return DBServer.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "drive"}, {Name: "path"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"mtime":       paths[0].MTime,
			"update_time": paths[0].UpdateTime,
		}),
	}).Create(&paths).Error
}
