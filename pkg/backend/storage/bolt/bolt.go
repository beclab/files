package bolt

import (
	"github.com/asdine/storm/v3"

	"files/pkg/backend/settings"
	"files/pkg/backend/storage"
)

// NewStorage creates a storage.Storage based on Bolt DB.
func NewStorage(db *storm.DB) (*storage.Storage, error) {
	settingsStore := settings.NewStorage(settingsBackend{db: db})

	err := save(db, "version", 2)
	if err != nil {
		return nil, err
	}

	return &storage.Storage{
		Settings: settingsStore,
	}, nil
}
