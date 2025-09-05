package database

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"k8s.io/klog/v2"
	"os"
)

var PGHOST = os.Getenv("PGHOST")
var PGPORT = os.Getenv("PGPORT")
var PGUSER = os.Getenv("PGUSER")
var PGPASSWORD = os.Getenv("PGPASSWORD")
var PGDB1 = os.Getenv("PGDB1")

var DB *gorm.DB

func Init() {
	var err error
	if PGHOST == "" || PGPORT == "" || PGUSER == "" || PGPASSWORD == "" || PGDB1 == "" {
		klog.Infoln("Postgres Database required environment variables are not set. Won't link to database.")
		return
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable timezone=UTC",
		PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDB1)
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		Logger:                 logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		panic(err)
	}
}
