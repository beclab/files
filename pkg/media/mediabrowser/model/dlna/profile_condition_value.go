package dlna

type ProfileConditionValue int

const (
	AudioChannels ProfileConditionValue = iota
	AudioBitrate
	AudioProfile
	Width
	Height
	Has64BitOffsets
	PacketLength
	VideoBitDepth
	VideoBitrate
	VideoFramerate
	VideoLevel
	VideoProfile
	VideoTimestamp
	IsAnamorphic
	RefFrames
	NumAudioStreams
	NumVideoStreams
	IsSecondaryAudio
	VideoCodecTag
	IsAvc
	IsInterlaced
	AudioSampleRate
	AudioBitDepth
	VideoRangeType
)
