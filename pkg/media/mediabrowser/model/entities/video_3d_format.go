package entities

type Video3DFormat int

const (
	HalfSideBySide Video3DFormat = iota
	FullSideBySide
	FullTopAndBottom
	HalfTopAndBottom
	MVC
)
