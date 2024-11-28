package postgres

import (
	"log"
	"time"
)

// ShareLink represents the structure of the share_link table in the database
type ShareLink struct {
	ID         int       `gorm:"primaryKey;autoIncrement"`
	LinkURL    string    `gorm:"type:text;not null;index:idx_share_links_link_url"`
	PathIDs    []int64   `gorm:"type:bigint[];default:NULL"`
	Password   string    `gorm:"type:varchar(32);not null"`
	OwnerID    string    `gorm:"type:text;not null"`
	OwnerName  string    `gorm:"type:text;not null"`
	Permission int       `gorm:"not null"`
	ExpireIn   int       `gorm:"not null"`
	ExpireTime time.Time `gorm:"not null;type:timestamptz"`
	Count      int       `gorm:"not null;default:0"`
	Status     int       `gorm:"not null"`
	CreateTime time.Time `gorm:"not null;type:timestamptz"`
	UpdateTime time.Time `gorm:"not null;type:timestamptz;autoUpdateTime"`
}

func createShareLinkTable() {
	// Automatically migrate the schema and create the table if it does not exist
	err := DBServer.AutoMigrate(&ShareLink{})
	if err != nil {
		log.Fatalf("Failed to migrate the database: %v", err)
	}

	// Optionally, you can create the index manually if you prefer
	// db.Model(&ShareLink{}).AddIndex("idx_link_url", "link_url")

	log.Println("Database migration succeeded.")
}
