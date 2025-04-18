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

	// db.AutoMigrate(&YourModel{})
	createPathInfoTable()
	createShareLinkTable()

	// test demo
	var count int
	DBServer.Raw("SELECT COUNT(*) FROM path_infos").Scan(&count)
	klog.Infof("Count: %d of path_infos\n", count)
	DBServer.Raw("SELECT COUNT(*) FROM share_links").Scan(&count)
	klog.Infof("Count: %d of share_links\n", count)
}
