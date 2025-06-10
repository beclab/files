package drives

import "time"

type DirentGeneratedEntry struct {
	Drive string
	Path  string
	Mtime time.Time
}

type ProcessedPathsEntry struct {
	Drive string    `json:"drive"`
	Path  string    `json:"path"`
	Mtime time.Time `json:"mtime"`
	Op    int       `json:"op"`
}
