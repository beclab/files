package main

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
	"os"
	"strings"
	"time"
)

var (
	DB_HOST     = os.Getenv("DB_HOST")
	DB_PORT     = os.Getenv("DB_PORT")
	DB_USER     = os.Getenv("DB_USER")
	DB_PASSWORD = os.Getenv("DB_PASSWORD")
	DB_NAME1    = os.Getenv("DB_NAME1")
	DB_NAME2    = os.Getenv("DB_NAME2")
	DB_NAME3    = os.Getenv("DB_NAME3")
)

type Profile struct {
	ContactEmail string `gorm:"column:contact_email"`
	User         string `gorm:"column:user"`
}

func logTableStructure(db *gorm.DB, tableName, dbName string) {
	var tables []string
	if tableName == "" {
		err := db.Raw(`
			SELECT table_name 
			FROM information_schema.tables 
			WHERE table_catalog = ? AND table_schema = 'public'`, dbName). // 添加 schema 过滤
			Pluck("table_name", &tables).Error

		klog.Infof("Found %d tables in database %s", len(tables), dbName)

		if err != nil {
			klog.Errorf("Failed to fetch table list: %v", err)
			return
		}
	} else {
		tables = []string{tableName}
	}

	for _, table := range tables {
		klog.Infof("=== Table Structure [%s.%s] ===", dbName, table)

		rows, err := db.Raw(`
			SELECT column_name, data_type, is_nullable 
			FROM information_schema.columns 
			WHERE table_catalog = ? AND table_schema = 'public' AND table_name = ?`,
			dbName, table).Rows()

		if err != nil {
			klog.Errorf("Failed to fetch columns for table %s: %v", table, err)
			continue
		}

		defer rows.Close()

		for rows.Next() {
			var colName, dataType, isNullable string
			if err := rows.Scan(&colName, &dataType, &isNullable); err != nil {
				klog.Errorf("Failed to parse column for table %s: %v", table, err)
				continue
			}
			klog.Infof("Column: %-20s Type: %-15s Nullable: %s", colName, dataType, isNullable)
		}
	}
}

func logTableData(db *gorm.DB, tableName, dbName string, limit int) {
	if dbName == "" {
		klog.Error("Database name cannot be empty")
		return
	}

	var tables []string
	if tableName == "" {
		err := db.Raw(`
			SELECT table_name 
			FROM information_schema.tables 
			WHERE table_catalog = ? 
			  AND table_schema = 'public'`, dbName).
			Pluck("table_name", &tables).Error

		klog.Infof("Found %d tables in database %s", len(tables), dbName)

		if err != nil {
			klog.Errorf("Failed to fetch table list: %v", err)
			return
		}
	} else {
		tables = []string{tableName}
	}

	for _, table := range tables {
		klog.Infof("=== Table Data [%s.%s] ===", dbName, table)

		var results []map[string]interface{}
		query := db.Table(table)
		if limit > 0 {
			query = query.Limit(limit)
		}
		if err := query.Find(&results).Error; err != nil {
			klog.Errorf("Failed to fetch data from table %s: %v", table, err)
			continue
		}

		if len(results) == 0 {
			klog.Info("  (no records found)")
			continue
		}

		for i, row := range results {
			klog.Infof("Record %d:", i+1)
			for k, v := range row {
				if v == nil {
					klog.Infof("  %s: NULL", k)
				} else {
					klog.Infof("  %s: %v", k, v)
				}
			}
		}
	}
}

func connectDBWithRetry(dsn string, dbName string, maxRetries int) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			if sqlDB, err := db.DB(); err == nil {
				if err = sqlDB.Ping(); err == nil {
					return db, nil
				}
			}
		}

		klog.Warningf("connect to database %s failed，have retried %d time(s)，error：%v", dbName, i+1, err)
		time.Sleep(1 * time.Second)
	}
	return nil, fmt.Errorf("database %s connection failed（retried %d times）", dbName, maxRetries)
}

type PreviewData struct {
	Table      string
	PrimaryKey string
	Column     string
	OldValue   string
	NewValue   string
}

func generateDetailedPreviewReport(db1, db2 *gorm.DB, profiles []Profile) []PreviewData {
	var previewData []PreviewData
	totalAffected := 0

	for _, p := range profiles {
		originalEmail := p.User
		newEmail := strings.Replace(p.ContactEmail, "@seafile.com", "@auth.local", 1)

		previewData = append(previewData, checkTableDetails(db1, DB_NAME1, "emailuser", "email", originalEmail, newEmail)...)
		previewData = append(previewData, checkTableDetails(db1, DB_NAME1, "binding", "email", originalEmail, newEmail)...)
		previewData = append(previewData, checkTableDetails(db1, DB_NAME1, "userrole", "email", originalEmail, newEmail)...)

		previewData = append(previewData, checkTableDetails(db2, DB_NAME2, "repousertoken", "email", originalEmail, newEmail)...)
		previewData = append(previewData, checkTableDetails(db2, DB_NAME2, "sharedrepo", "from_email", originalEmail, newEmail)...)
		previewData = append(previewData, checkTableDetails(db2, DB_NAME2, "sharedrepo", "to_email", originalEmail, newEmail)...)
		previewData = append(previewData, checkTableDetails(db2, DB_NAME2, "repoowner", "owner_id", originalEmail, newEmail)...)
		previewData = append(previewData, checkTableDetails(db2, DB_NAME2, "repotrash", "owner_id", originalEmail, newEmail)...)
		previewData = append(previewData, checkTableDetails(db2, DB_NAME2, "userquota", "\"user\"", originalEmail, newEmail)...)
		previewData = append(previewData, checkTableDetails(db2, DB_NAME2, "usersharequota", "\"user\"", originalEmail, newEmail)...)
	}

	totalAffected = len(previewData)

	klog.Info("=== Data Pre-update Report ===")
	klog.Infof("Totally need to update %d record(s)", totalAffected)
	klog.Info("------------------------------------------------------------------")
	klog.Info("Table Name\t\tPrimary Key\t\tField Name\t\tOld Value\t\t\tNew Value")
	klog.Info("------------------------------------------------------------------")
	for _, data := range previewData {
		klog.Infof("%s\t%s\t%s\t%s\t→\t%s",
			data.Table,
			data.PrimaryKey,
			data.Column,
			data.OldValue,
			data.NewValue,
		)
	}
	klog.Info("------------------------------------------------------------------")
	return previewData
}

func checkTableDetails(db *gorm.DB, dbName, table, column, oldValue, newValue string) []PreviewData {
	var results []PreviewData

	rows, err := db.Table(table).
		Select("id, "+column).
		Where(column+" = ?", oldValue).
		Rows()
	if err != nil {
		klog.Errorf("query table %s.%s failed: %v", dbName, table, err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var (
			primaryKey  string
			columnValue string
		)
		err := rows.Scan(&primaryKey, &columnValue)
		if err != nil {
			klog.Errorf("parse records of table %s.%s failed: %v", dbName, table, err)
			continue
		}

		results = append(results, PreviewData{
			Table:      fmt.Sprintf("%s.%s", dbName, table),
			PrimaryKey: primaryKey,
			Column:     column,
			OldValue:   columnValue,
			NewValue:   newValue,
		})
	}

	return results
}

func executeUpdates(db1, db2 *gorm.DB, previewData []PreviewData) error {
	tx1 := db1.Begin()
	tx2 := db2.Begin()

	defer func() {
		if r := recover(); r != nil {
			tx1.Rollback()
			tx2.Rollback()
		}
	}()

	for _, data := range previewData {
		var tx *gorm.DB
		if strings.HasPrefix(data.Table, DB_NAME1) {
			tx = tx1
		} else if strings.HasPrefix(data.Table, DB_NAME2) {
			tx = tx2
		} else {
			continue
		}

		tableName := strings.SplitN(data.Table, ".", 2)[1]

		if err := tx.Table(tableName).
			Where("id = ?", data.PrimaryKey).
			Update(data.Column, data.NewValue).Error; err != nil {

			tx1.Rollback()
			tx2.Rollback()
			return fmt.Errorf("update failed for table %s: %v", tableName, err)
		}
	}

	if err := tx1.Commit().Error; err != nil {
		tx2.Rollback()
		return fmt.Errorf("db1 commit failed: %v", err)
	}

	if err := tx2.Commit().Error; err != nil {
		return fmt.Errorf("db2 commit failed: %v", err)
	}

	return nil
}

func main() {
	klog.SetOutput(os.Stdout)
	defer klog.Flush()

	dsn1 := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		DB_HOST, DB_USER, DB_PASSWORD, DB_NAME1, DB_PORT)
	dsn2 := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		DB_HOST, DB_USER, DB_PASSWORD, DB_NAME2, DB_PORT)
	dsn3 := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		DB_HOST, DB_USER, DB_PASSWORD, DB_NAME3, DB_PORT)
	klog.Infof("dsn1: %s", dsn1)
	klog.Infof("dsn2: %s", dsn2)
	klog.Infof("dsn3: %s", dsn3)

	db1, err := connectDBWithRetry(dsn1, DB_NAME1, 5)
	if err != nil {
		klog.Fatal(err.Error())
	}
	klog.Infof("db1: %v", db1)

	db2, err := connectDBWithRetry(dsn2, DB_NAME2, 5)
	if err != nil {
		klog.Fatal(err.Error())
	}
	klog.Infof("db2: %v", db2)

	db3, err := connectDBWithRetry(dsn3, DB_NAME3, 5)
	if err != nil {
		klog.Errorf("Warning: %v (but connection to main databases is successful, will go on processing)", err)
	}

	if db3 != nil {
		logTableStructure(db3, "profile_profile", DB_NAME3)
		logTableData(db3, "profile_profile", DB_NAME3, 0)
	}

	logTableStructure(db1, "", DB_NAME1)
	logTableData(db1, "", DB_NAME1, 0)

	logTableStructure(db2, "", DB_NAME2)
	logTableData(db2, "", DB_NAME2, 0)

	if db3 == nil {
		klog.Info("No need to update")
		return
	}

	var profiles []Profile
	if err = db3.Table("profile_profile").
		Where("contact_email LIKE ?", "%@seafile.com").
		Find(&profiles).Error; err != nil {
		klog.Fatal("Failed to query profile_profile:", err)
	}

	klog.Infof("Found %d profiles: %v", len(profiles), profiles)

	previewData := generateDetailedPreviewReport(db1, db2, profiles)

	if err := executeUpdates(db1, db2, previewData); err != nil {
		klog.Fatalf("Update failed: %v", err)
	}

	klog.Info("Data update completed successfully")

	klog.Info("\n=== Verification After Update ===")
	_ = generateDetailedPreviewReport(db1, db2, profiles)
	return
}
