package dlna

import (
	"files/pkg/media/jellyfin/data/enums"
)

type TranscodingProfile struct {
	Container                 string
	Type                      DlnaProfileType
	VideoCodec                string
	AudioCodec                string
	Protocol                  enums.MediaStreamProtocol
	EstimateContentLength     bool
	EnableMpegtsM2TsMode      bool
	TranscodeSeekInfo         TranscodeSeekInfo
	CopyTimestamps            bool
	Context                   EncodingContext
	EnableSubtitlesInManifest bool
	MaxAudioChannels          *string
	MinSegments               int
	SegmentLength             int
	BreakOnNonKeyFrames       bool
	Conditions                []ProfileCondition
}

func NewTranscodingProfile() *TranscodingProfile {
	return &TranscodingProfile{
		Conditions: make([]ProfileCondition, 0),
	}
}

/*
func (tp *TranscodingProfile) GetAudioCodecs() []string {
    return ContainerProfile.SplitValue(tp.AudioCodec)
}
*/
