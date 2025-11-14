package mediainfodtos

import (
	"files/pkg/media/mediabrowser/model/dlna"
)

type PlaybackInfoDto struct {
	UserId               *string
	MaxStreamingBitrate  *int
	StartTimeTicks       *int64
	AudioStreamIndex     *int
	SubtitleStreamIndex  *int
	MaxAudioChannels     *int
	MediaSourceId        *string
	LiveStreamId         *string
	DeviceProfile        *dlna.DeviceProfile
	EnableDirectPlay     *bool
	EnableDirectStream   *bool
	EnableTranscoding    *bool
	AllowVideoStreamCopy *bool
	AllowAudioStreamCopy *bool
	AutoOpenLiveStream   *bool
}
