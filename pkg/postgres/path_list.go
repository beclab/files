package postgres

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PathList struct {
	Drive      string    `gorm:"type:varchar(10);not null;primaryKey"`
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

func parseDrive(path string) (string, string) {
	pathSplit := strings.Split(path, "/")
	if len(pathSplit) < 3 {
		return "Unknown drive", path
	}
	if strings.HasPrefix(pathSplit[1], "pvc-userspace-") {
		if pathSplit[2] == "Data" {
			return "data", filepath.Join(pathSplit[1:]...)
		} else if pathSplit[2] == "Home" {
			return "drive", filepath.Join(pathSplit[1:]...)
		}
	}
	if pathSplit[1] == "appcache" {
		return "cache", filepath.Join(pathSplit[2:]...)
	}
	if pathSplit[1] == "External" {
		return "External", filepath.Join(pathSplit[2:]...) // TODO: External types
	}
	return "Parse Error", path
}

func InitDrivePathList() {
	rootPath := "/data"

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Process directory
			drive, parsedPath := parseDrive(path)
			return processDirectory(drive, parsedPath, info.ModTime())
		}

		// Process file (if needed)
		// Uncomment the following line if you need to process files
		// processFile(db, drive, path, info.ModTime())

		return nil
	})

	if err != nil {
		fmt.Println("Error walking the path:", err)
	}

	if err = logPathList(); err != nil {
		fmt.Println("Error logging path list:", err)
	}
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

func processDirectory(drive, path string, modTime time.Time) error {
	// Get the record from the database
	var record PathList
	if err := DBServer.First(&record, "drive = ? AND path = ?", drive, path).Error; err != nil {
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
		return DBServer.Create(&record).Error
	}

	// If the record exists, check the modification time
	if record.MTime.After(modTime) {
		// Skip if the database time is after the file system time
		return nil
	}

	// Update the modification time in the database
	record.MTime = modTime
	record.UpdateTime = time.Now()
	return DBServer.Save(&record).Error
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
