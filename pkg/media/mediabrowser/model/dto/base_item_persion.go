package dto

import (
	"github.com/google/uuid"

	"files/pkg/media/jellyfin/data/enums"
	"files/pkg/media/mediabrowser/model/entities"
)

type BaseItemPerson struct {
	// Gets or sets the name.
	Name string `json:"name"`
	// Gets or sets the identifier.
	ID uuid.UUID `json:"id"`
	// Gets or sets the role.
	Role string `json:"role"`
	// Gets or sets the type.
	Type enums.PersonKind `json:"type"`
	// Gets or sets the primary image tag.
	PrimaryImageTag string `json:"primaryImageTag"`
	// Gets or sets the primary image blurhash.
	ImageBlurHashes map[entities.ImageType]map[string]string `json:"imageBlurHashes"`
	// Gets a value indicating whether this instance has primary image.
	HasPrimaryImage bool `json:"-"`
}
