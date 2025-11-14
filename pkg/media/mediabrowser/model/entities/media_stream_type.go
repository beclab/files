package entities

type MediaStreamType string

const (
	MediaStreamTypeAudio         MediaStreamType = "Audio"
	MediaStreamTypeVideo         MediaStreamType = "Video"
	MediaStreamTypeSubtitle      MediaStreamType = "Subtitle"
	MediaStreamTypeEmbeddedImage MediaStreamType = "EmbeddedImage"
	MediaStreamTypeData          MediaStreamType = "Data"
	MediaStreamTypeLyric         MediaStreamType = "Lyric"
)
