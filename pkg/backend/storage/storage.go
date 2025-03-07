package storage

import (
	"files/pkg/backend/settings"
)

// Storage is a storage powered by a Backend which makes the necessary
// verifications when fetching and saving data to ensure consistency.
type Storage struct {
	Settings *settings.Storage
}
