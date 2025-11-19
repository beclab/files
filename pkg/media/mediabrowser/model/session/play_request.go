package session

type VideoRangeType int

const (
	Unknown VideoRangeType = iota
	SDR
	HDR10
	HLG
	DOVI
	DOVIWithHDR10
	DOVIWithHLG
	DOVIWithSDR
	HDR10Plus
)
