package entities

type VideoType int

const (
	VideoFile VideoType = iota
	Iso
	Dvd
	BluRay
)
