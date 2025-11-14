package entities

import (
	"files/pkg/media/jellyfin/data/enums"
)

type BaseItem struct {
	BaseItemKind             *enums.BaseItemKind
	ThemeSongFileName        string
	SupportedImageExtensions []string
	SupportedExtensions      []string
}

func NewBaseItem() *BaseItem {
	return &BaseItem{
		SupportedImageExtensions: []string{".png", ".jpg", ".jpeg", ".webp", ".tbn", ".gif"},
		SupportedExtensions: []string{
			".nfo", ".xml", ".srt", ".vtt", ".sub", ".sup",
			".idx", ".txt", ".edl", ".bif", ".smi", ".ttml",
			".lrc", ".elrc",
		},
	}
}

func (b *BaseItem) SetBaseItemKind(kind enums.BaseItemKind) {
	b.BaseItemKind = &kind
}
