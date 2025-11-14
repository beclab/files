package session

import (
	"files/pkg/media/mediabrowser/model/entities"
)

type TranscodingInfo struct {
	AudioCodec               string
	VideoCodec               string
	Container                string
	IsVideoDirect            bool
	IsAudioDirect            bool
	Bitrate                  *int
	Framerate                *float32
	CompletionPercentage     *float64
	Width                    *int
	Height                   *int
	AudioChannels            *int
	HardwareAccelerationType *entities.HardwareAccelerationType
	TranscodeReasons         TranscodeReason
}
