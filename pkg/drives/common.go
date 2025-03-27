package drives

import "time"

type DirentGeneratedEntry struct {
	Drive string
	Path  string
	Mtime time.Time
}

type DrivesAccounsResponseItem struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	ExpiresAt int64  `json:"expires_at"`
	Available bool   `json:"available"`
	CreateAt  int64  `json:"create_at"`
}

type DriveAccountsResponse struct {
	Code int                         `json:"code"`
	Data []DrivesAccounsResponseItem `json:"data"`
}

type ProcessedPathsEntry struct {
	Drive string    `json:"drive"`
	Path  string    `json:"path"`
	Mtime time.Time `json:"mtime"`
	Op    int       `json:"op"`
}
