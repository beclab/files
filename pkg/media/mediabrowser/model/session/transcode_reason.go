package session

/*
type TranscodeReason int

const (

	// Primary
	ContainerNotSupported TranscodeReason = 1 << iota
	VideoCodecNotSupported
	AudioCodecNotSupported
	SubtitleCodecNotSupported
	AudioIsExternal
	SecondaryAudioNotSupported

	// Video Constraints
	VideoProfileNotSupported
	VideoRangeTypeNotSupported
	VideoLevelNotSupported
	VideoResolutionNotSupported
	VideoBitDepthNotSupported
	VideoFramerateNotSupported
	RefFramesNotSupported
	AnamorphicVideoNotSupported
	InterlacedVideoNotSupported

	// Audio Constraints
	AudioChannelsNotSupported
	AudioProfileNotSupported
	AudioSampleRateNotSupported
	AudioBitDepthNotSupported

	// Bitrate Constraints
	ContainerBitrateExceedsLimit
	VideoBitrateNotSupported
	AudioBitrateNotSupported

	// Errors
	UnknownVideoStreamInfo
	UnknownAudioStreamInfo
	DirectPlayError

)
*/
type TranscodeReason int64

const (
	// Primary
	ContainerNotSupported      TranscodeReason = 1 << 0
	VideoCodecNotSupported     TranscodeReason = 1 << 1
	AudioCodecNotSupported     TranscodeReason = 1 << 2
	SubtitleCodecNotSupported  TranscodeReason = 1 << 3
	AudioIsExternal            TranscodeReason = 1 << 4
	SecondaryAudioNotSupported TranscodeReason = 1 << 5

	// Video Constraints
	VideoProfileNotSupported    TranscodeReason = 1 << 6
	VideoRangeTypeNotSupported  TranscodeReason = 1 << 24
	VideoLevelNotSupported      TranscodeReason = 1 << 7
	VideoResolutionNotSupported TranscodeReason = 1 << 8
	VideoBitDepthNotSupported   TranscodeReason = 1 << 9
	VideoFramerateNotSupported  TranscodeReason = 1 << 10
	RefFramesNotSupported       TranscodeReason = 1 << 11
	AnamorphicVideoNotSupported TranscodeReason = 1 << 12
	InterlacedVideoNotSupported TranscodeReason = 1 << 13

	// Audio Constraints
	AudioChannelsNotSupported   TranscodeReason = 1 << 14
	AudioProfileNotSupported    TranscodeReason = 1 << 15
	AudioSampleRateNotSupported TranscodeReason = 1 << 16
	AudioBitDepthNotSupported   TranscodeReason = 1 << 17

	// Bitrate Constraints
	ContainerBitrateExceedsLimit TranscodeReason = 1 << 18
	VideoBitrateNotSupported     TranscodeReason = 1 << 19
	AudioBitrateNotSupported     TranscodeReason = 1 << 20

	// Errors
	UnknownVideoStreamInfo TranscodeReason = 1 << 21
	UnknownAudioStreamInfo TranscodeReason = 1 << 22
	DirectPlayError        TranscodeReason = 1 << 23
)
