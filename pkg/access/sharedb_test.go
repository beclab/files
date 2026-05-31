package access

import (
	"testing"
	"time"

	"files/pkg/hertz/biz/dal/database"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newShareTestDB points database.DB at a fresh in-memory sqlite instance
// with the share tables the access functions query, and restores the
// previous handle on cleanup. Tables are created with hand-written DDL
// rather than gorm AutoMigrate so the structs' Postgres-only column tags
// (gen_random_uuid() defaults, timestamptz) don't break under sqlite.
func newShareTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	ddl := []string{
		`CREATE TABLE share_paths (
			id TEXT PRIMARY KEY,
			owner TEXT,
			file_type TEXT,
			extend TEXT,
			path TEXT,
			share_type TEXT,
			expire_time TEXT,
			permission INTEGER
		)`,
		`CREATE TABLE share_members (
			id INTEGER PRIMARY KEY,
			path_id TEXT,
			share_member TEXT,
			permission INTEGER
		)`,
		`CREATE TABLE share_tokens (
			id INTEGER PRIMARY KEY,
			path_id TEXT,
			token TEXT,
			expire_at TEXT
		)`,
	}
	for _, stmt := range ddl {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create share table: %v", err)
		}
	}

	saved := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = saved })
}

// seedSharePath inserts a share_paths row. file_type/extend/path are fixed
// because none of the auth functions branch on them.
func seedSharePath(t *testing.T, id, owner, shareType, expireTime string, permission int32) {
	t.Helper()
	err := database.DB.Exec(
		`INSERT INTO share_paths (id, owner, file_type, extend, path, share_type, expire_time, permission)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, owner, "drive", "Home", "/", shareType, expireTime, permission,
	).Error
	if err != nil {
		t.Fatalf("seed share_paths: %v", err)
	}
}

func seedShareMember(t *testing.T, pathID, member string, permission int32) {
	t.Helper()
	err := database.DB.Exec(
		`INSERT INTO share_members (path_id, share_member, permission) VALUES (?, ?, ?)`,
		pathID, member, permission,
	).Error
	if err != nil {
		t.Fatalf("seed share_members: %v", err)
	}
}

func seedShareToken(t *testing.T, pathID, token, expireAt string) {
	t.Helper()
	err := database.DB.Exec(
		`INSERT INTO share_tokens (path_id, token, expire_at) VALUES (?, ?, ?)`,
		pathID, token, expireAt,
	).Error
	if err != nil {
		t.Fatalf("seed share_tokens: %v", err)
	}
}

func futureRFC3339(d time.Duration) string {
	return time.Now().Add(d).UTC().Format(time.RFC3339Nano)
}
