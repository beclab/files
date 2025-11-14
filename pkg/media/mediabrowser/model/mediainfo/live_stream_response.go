package mediainfo

import (
	"files/pkg/media/mediabrowser/model/dto"
)

type LiveStreamResponse struct {
	MediaSource dto.MediaSourceInfo
}

func NewLiveStreamResponse(mediaSource dto.MediaSourceInfo) *LiveStreamResponse {
	return &LiveStreamResponse{
		MediaSource: mediaSource,
	}
}
