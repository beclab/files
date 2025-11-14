package mediainfo

import (
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/mediabrowser/model/entities"
	"time"
)

type MediaInfo struct {
	dto.MediaSourceInfo
	entities.IHasProviderIds
	Chapters          []*entities.ChapterInfo
	Album             string
	Artists           []string
	AlbumArtists      []string
	Studios           []string
	Genres            []string
	ShowName          string
	ForcedSortName    string
	IndexNumber       *int
	ParentIndexNumber *int
	ProductionYear    *int
	PremiereDate      *time.Time
	//    People             []*BaseItemPerson
	ProviderIds               map[string]string
	OfficialRating            string
	OfficialRatingDescription string
	Overview                  string
}

func NewMediaInfo() *MediaInfo {
	return &MediaInfo{
		ProviderIds: make(map[string]string, 0),
	}
}

func (m *MediaInfo) GetProviderIds() map[string]string {
	return m.ProviderIds
}

func (m *MediaInfo) SetProviderIds(providerIds map[string]string) {
	m.ProviderIds = providerIds
}
