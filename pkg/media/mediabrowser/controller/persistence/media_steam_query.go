package persistence

import (
	"files/pkg/media/mediabrowser/model/entities"
	"github.com/google/uuid"
)

type MediaStreamQuery struct {
	Type   *entities.MediaStreamType
	Index  *int
	ItemId uuid.UUID
}

func NewMediaStreamQuery(itemId uuid.UUID) *MediaStreamQuery {
	return &MediaStreamQuery{
		ItemId: itemId,
	}
}

func (q *MediaStreamQuery) SetType(t entities.MediaStreamType) {
	q.Type = &t
}

func (q *MediaStreamQuery) SetIndex(i int) {
	q.Index = &i
}
