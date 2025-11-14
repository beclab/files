package mediainfo

import (
	"github.com/google/uuid"

	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
)

type LiveStreamRequest struct {
	OpenToken           string
	UserId              uuid.UUID
	PlaySessionId       string
	MaxStreamingBitrate *int
	StartTimeTicks      *int64
	AudioStreamIndex    *int
	SubtitleStreamIndex *int
	MaxAudioChannels    *int
	ItemId              uuid.UUID
	DeviceProfile       dlna.DeviceProfile
	EnableDirectPlay    bool
	EnableDirectStream  bool
	DirectPlayProtocols []mediaprotocol.MediaProtocol
}

func NewLiveStreamRequest() *LiveStreamRequest {
	return &LiveStreamRequest{
		EnableDirectPlay:    true,
		EnableDirectStream:  true,
		DirectPlayProtocols: []mediaprotocol.MediaProtocol{mediaprotocol.Http},
	}
}
