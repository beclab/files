package entities

import (
	"time"
)

type ChapterInfo struct {
	StartPositionTicks int64
	Name               *string
	ImagePath          *string
	ImageDateModified  time.Time
	ImageTag           *string
}
