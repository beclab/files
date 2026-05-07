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

// Init connects the package-level DB to Postgres if all required
// PG* environment variables are set. If any of them is missing the
// function returns nil and DB is left as nil; callers must handle
// the "no DB" case (see dal.Init).
//
// On a real connection failure Init now returns the error instead
// of panicking; callers can decide whether to klog.Fatal or run
// degraded. The previous panic produced a backtrace into the log
// for what is conceptually a fatal-with-clear-message situation.
func Init() error {
	if PGHOST == "" || PGPORT == "" || PGUSER == "" || PGPASSWORD == "" || PGDB1 == "" {
		klog.Infoln("Postgres Database required environment variables are not set. Won't link to database.")
		return nil
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable timezone=UTC",
		PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDB1)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("postgres open: %w", err)
	}
	DB = db
	return nil
}

// Close releases the underlying *sql.DB connection pool. Safe to call when
// Init was skipped (DB == nil) or when the GORM-managed pool is missing.
// Only invoked from the graceful-shutdown coordinator after all request
// handlers, cron jobs, and watchers have stopped using the pool.
func Close() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	if sqlDB == nil {
		return nil
	}
	return sqlDB.Close()
}
