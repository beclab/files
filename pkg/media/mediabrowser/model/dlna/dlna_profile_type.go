package dlna

type DlnaProfileType int

const (
	Audio DlnaProfileType = iota
	Video
	Photo
	Subtitle
	Lyric
)
