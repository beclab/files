package postgres

import (
	"context"
	"files/pkg/common"
	"files/pkg/drives"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PathList struct {
	Drive      string    `gorm:"type:varchar(20);not null;primaryKey"`
	Path       string    `gorm:"type:text;not null;primaryKey"`
	MTime      time.Time `gorm:"not null;type:timestamptz"`
	ParseDoc   bool      `gorm:"type:boolean;not null;default:false"`
	Status     int       `gorm:"not null;default:0"`
	CreateTime time.Time `gorm:"not null;type:timestamptz;autoCreateTime"`
	UpdateTime time.Time `gorm:"not null;type:timestamptz;autoUpdateTime"`
}

var pathListInited bool = false

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
	var srcTypeList = []string{
		drives.SrcTypeDrive,
		drives.SrcTypeCache,
	}

	for _, srcType := range srcTypeList {
		rs, err := drives.GetResourceService(srcType)
		if err != nil {
			klog.Errorf("failed to get resource service: %v", err)
			return
		}

		err = rs.GeneratePathList(DBServer, ProcessDirectory)
		if err != nil {
			klog.Errorf("failed to generate drive path list: %v", err)
			return
		}
	}

	pathListInited = true

	if err := logPathList(); err != nil {
		fmt.Println("Error logging path list:", err)
	}
	return
}

func GenerateOtherPathList(ctx context.Context) {
	go func() {
		var mu sync.Mutex
		cond := sync.NewCond(&mu)

		go func() {
			for {
				time.Sleep(1 * time.Second)
				mu.Lock()
				if len(common.BflCookieCache) > 0 {
					//klog.Info("~~~Temp log: cookie has come")
					cond.Broadcast()
				} else {
					//klog.Info("~~~Temp log: cookie hasn't come")
				}
				mu.Unlock()

				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}()

		for {
			mu.Lock()
			for len(common.BflCookieCache) == 0 {
				//klog.Info("~~~Temp log: waiting for cookie")
				done := ctx.Done()
				if done != nil {
					select {
					case <-done:
						mu.Unlock()
						return
					default:
						cond.Wait()
					}
				} else {
					cond.Wait()
				}
			}

			var srcTypeList = []string{
				drives.SrcTypeSync,
			}

			for _, srcType := range srcTypeList {
				rs, err := drives.GetResourceService(srcType)
				if err != nil {
					klog.Errorf("failed to get resource service: %v", err)
					return
				}

				err = rs.GeneratePathList(DBServer, ProcessDirectory)
				if err != nil {
					klog.Errorf("failed to generate drive path list: %v", err)
					return
				}
			}

			if err := logPathList(); err != nil {
				fmt.Println("Error logging path list:", err)
			}

			mu.Unlock()
			return
		}
	}()
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
	if drive == "unknown" || drive == "error" || path == "" {
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

func CheckAndUpdateStatus(ctx context.Context) {
	if !pathListInited {
		return
	}

	var pathEntries []PathList

	if err := DBServer.WithContext(ctx).Where("drive IN (?, ?, ?, ?)", "drive", "data", "cache", "external").Find(&pathEntries).Error; err != nil {
		klog.Errorf("failed to query drive path list: %v", err)
		return
	}

	for _, entry := range pathEntries {
		var fullPath string
		var exists bool
		var err error

		switch entry.Drive {
		case "drive":
			fullPath = "/data/" + entry.Path
		case "data":
			fullPath = "/data/" + entry.Path
		case "cache":
			fullPath = "/appcache/" + entry.Path
		case "external":
			pathSplit := strings.Split(entry.Path, "/")
			fullPath = "/data/External/" + filepath.Join(pathSplit[3:]...)
		default:
			continue
		}

		exists, err = pathExists(fullPath)
		if err != nil {
			klog.Errorf("failed to check if path exists: %v", err)
			return
		}

		var newStatus int
		if exists {
			newStatus = 0
		} else {
			newStatus = 1
		}

		if entry.Status == newStatus {
			continue
		}

		if err := DBServer.WithContext(ctx).Model(&PathList{}).Where("drive = ? AND path = ?", entry.Drive, entry.Path).Update("status", newStatus).Error; err != nil {
			klog.Errorf("failed to update drive path status: %v", err)
			return
		}
	}

	return
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
