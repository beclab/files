package library

import (
	"files/pkg/media/mediabrowser/controller/entities"
	"github.com/google/uuid"
)

type ILibraryManager interface {
	GetItemById(id uuid.UUID) *entities.BaseItem
	GetItemById2(id uuid.UUID, typ any) any
	GetItemById3(id uuid.UUID, userId uuid.UUID) any
	GetItemById4(id uuid.UUID /*, user *User*/) any
}
