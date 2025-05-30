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

var DBServer *gorm.DB = nil

func InitPostgres() {
	var err error

	if PGHOST == "" || PGPORT == "" || PGUSER == "" || PGPASSWORD == "" || PGDB1 == "" {
		klog.Infoln("Postgres Database required environment variables are not set. Won't link to database.")
		return
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDB1)

	DBServer, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		klog.Errorf("Error connecting to PostgreSQL: %v\n", err)
		return
	}

	db, err := DBServer.DB()
	if err != nil {
		klog.Errorf("Error connecting to PostgreSQL: %v\n", err)
	}
	if err = db.Ping(); err != nil {
		klog.Errorf("Error pinging PostgreSQL: %v\n", err)
		return
	}

	klog.Infoln("Successfully connected to PostgreSQL!")

	createPathListTable()
	createPathInfoTable()
	createShareLinkTable()

	// test demo
	var count int
	DBServer.Raw("SELECT COUNT(*) FROM path_infos").Scan(&count)
	klog.Infof("Count: %d of path_infos\n", count)
	DBServer.Raw("SELECT COUNT(*) FROM share_links").Scan(&count)
	klog.Infof("Count: %d of share_links\n", count)
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
