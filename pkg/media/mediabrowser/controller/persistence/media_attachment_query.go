package persistence

import (
	"github.com/google/uuid"
)

type MediaAttachmentQuery struct {
	Index  *int
	ItemId uuid.UUID
}
