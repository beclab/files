package postgres

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
	"os"
)

var PGHOST = os.Getenv("PGHOST")
var PGPORT = os.Getenv("PGPORT")
var PGUSER = os.Getenv("PGUSER")
var PGPASSWORD = os.Getenv("PGPASSWORD")
var PGDB1 = os.Getenv("PGDB1")
var SEAFILEPGUSER = os.Getenv("SEAFILEPGUSER")
var SEAFILEPGPASSWORD = os.Getenv("SEAFILEPGPASSWORD")
var SEAFILEPGDB1 = os.Getenv("SEAFILEPGDB1")
var SEAFILEPGDB2 = os.Getenv("SEAFILEPGDB2")
var SEAFILEPGDB3 = os.Getenv("SEAFILEPGDB3")

var DBServer *gorm.DB = nil
var CCNetDBServer *gorm.DB = nil
var SeafileDBServer *gorm.DB = nil
var SeahubDBServer *gorm.DB = nil

func InitPostgres() {
	var err error

	if PGHOST == "" || PGPORT == "" || PGUSER == "" || PGPASSWORD == "" || PGDB1 == "" ||
		SEAFILEPGUSER == "" || SEAFILEPGPASSWORD == "" || SEAFILEPGDB1 == "" || SEAFILEPGDB2 == "" || SEAFILEPGDB3 == "" {
		klog.Infoln("Postgres Database required environment variables are not set. Won't link to database.")
		return
	}

	dbs := []string{PGDB1, SEAFILEPGDB1, SEAFILEPGDB2, SEAFILEPGDB3}

	for _, dbName := range dbs {
		var dsn string
		if dbName == PGDB1 {
			dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
				PGHOST, PGPORT, PGUSER, PGPASSWORD, dbName)
		} else {
			dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
				PGHOST, PGPORT, SEAFILEPGUSER, SEAFILEPGPASSWORD, dbName)
		}

		var dbConn *gorm.DB
		switch dbName {
		case PGDB1:
			DBServer, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
			dbConn = DBServer
		case SEAFILEPGDB1:
			CCNetDBServer, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
			dbConn = CCNetDBServer
		case SEAFILEPGDB2:
			SeafileDBServer, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
			dbConn = SeafileDBServer
		case SEAFILEPGDB3:
			SeahubDBServer, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
			dbConn = SeahubDBServer
		}

		if err != nil {
			klog.Errorf("[%s] Connection error: %v", dbName, err)
			continue
		}

		sqlDB, err := dbConn.DB()
		if err != nil {
			klog.Errorf("[%s] Get DB instance error: %v", dbName, err)
			continue
		}

		if err = sqlDB.Ping(); err != nil {
			klog.Errorf("[%s] Ping error: %v", dbName, err)
			continue
		}

		if dbName == PGDB1 {
			createPathInfoTable()
			createShareLinkTable()
		}

		// for test
		var tables []string
		if err := dbConn.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'").Scan(&tables).Error; err != nil {
			klog.Errorf("[%s] Query tables error: %v", dbName, err)
			continue
		}

		klog.Infof("[%s] Tables (%d):", dbName, len(tables))
		for _, table := range tables {
			klog.Infof("- %s", table)
		}
	}

	// for test
	var emailuserResults []map[string]interface{}
	if err = CCNetDBServer.Raw("SELECT * FROM emailuser").Scan(&emailuserResults).Error; err == nil {
		for _, row := range emailuserResults {
			klog.Infof("emailuser Row data: %v", row)
		}
	}

	// for test
	var profileResults []map[string]interface{}
	if err = SeahubDBServer.Raw("SELECT * FROM profile_profile").Scan(&profileResults).Error; err == nil {
		for _, row := range profileResults {
			klog.Infof("profile_profile Row data: %v", row)
		}
	}

	var tableSchema []map[string]interface{}
	if err = SeahubDBServer.Raw(`
		SELECT column_name, data_type, is_nullable, column_default 
		FROM information_schema.columns 
		WHERE table_name = 'profile_profile'
	`).Scan(&tableSchema).Error; err == nil {
		klog.Infof("Table structure for profile_profile:")
		for _, col := range tableSchema {
			klog.Infof("%v", col)
		}
	}

	klog.Infoln("Successfully connected to PostgreSQL!")
}

func ClearTable(db *gorm.DB, model interface{}) error {
	// example: ClearTable(db, &YourModel{})
	return db.Exec("DELETE FROM ?", model).Error
}

func DropTable(db *gorm.DB, model interface{}) error {
	// example: DropTable(db, &YourModel{})
	return db.Migrator().DropTable(model)
}

func RecreateTable(db *gorm.DB, model interface{}) error {
	// example: RecreateTable(db, &YourModel{})
	if err := DropTable(db, model); err != nil {
		return err
	}
	return db.Migrator().CreateTable(model)
}

func InsertData(db *gorm.DB, data interface{}) error {
	// example: InsertData(db, &YourModel{Name: "Example", Value: 42})
	return db.Create(data).Error
}

func DeleteData(db *gorm.DB, data interface{}) error {
	// example: DeleteData(db, &YourModel{ID: 1})
	return db.Delete(data).Error
}

func SearchData(db *gorm.DB, model interface{}, conditions map[string]interface{}, orderBy string, groupBy string, limit int, offset int) ([]interface{}, int64, error) {
	// example:
	//		conditions := map[string]interface{}{"name": "Example"}
	//		orderBy := "id DESC"
	//		groupBy := "name"
	//		limit := 10
	//		offset := 0
	//
	//		results, count, err := SearchData(db, &YourModel{}, conditions, orderBy, groupBy, limit, offset)
	//		if err != nil {
	//			klog.Errorln("Error:", err)
	//		} else {
	//			klog.Infof("Results: %v\nCount: %d\n", results, count)
	//		}

	var results []interface{}
	query := db.Model(model)

	// Apply conditions
	for key, value := range conditions {
		query = query.Where(key+" = ?", value)
	}

	// Apply ORDER BY if specified
	if orderBy != "" {
		query = query.Order(orderBy)
	}

	// Apply GROUP BY if specified
	if groupBy != "" {
		query = query.Group(groupBy)
	}

	// Apply LIMIT and OFFSET if specified
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	// Execute the query and store the results
	result := query.Find(&results)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	// Return results, count of rows, and error (if any)
	return results, result.RowsAffected, nil
}
