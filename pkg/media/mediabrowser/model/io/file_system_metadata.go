package io

import (
	"time"
)

type FileSystemMetadata struct {
	Exists           bool
	FullName         string
	Name             string
	Extension        string
	Length           int64
	LastWriteTimeUtc time.Time
	CreationTimeUtc  time.Time
	IsDirectory      bool
}
