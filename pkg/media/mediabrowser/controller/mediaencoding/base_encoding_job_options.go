package mediaencoding

import (
	"github.com/google/uuid"

	"files/pkg/media/mediabrowser/model/dlna"
)

type BaseEncodingJobOptions struct {
	PlayPath                            string                      `json:"playPath"`
	Id                                  uuid.UUID                   `json:"id"`
	MediaSourceID                       string                      `json:"mediaSourceId"`
	DeviceID                            string                      `json:"deviceId"`
	Container                           string                      `json:"container"`
	AudioCodec                          string                      `json:"audioCodec"`
	EnableAutoStreamCopy                bool                        `json:"enableAutoStreamCopy"`
	AllowVideoStreamCopy                bool                        `json:"allowVideoStreamCopy"`
	AllowAudioStreamCopy                bool                        `json:"allowAudioStreamCopy"`
	BreakOnNonKeyFrames                 bool                        `json:"breakOnNonKeyFrames"`
	AudioSampleRate                     *int                        `json:"audioSampleRate"`
	MaxAudioBitDepth                    *int                        `json:"maxAudioBitDepth"`
	AudioBitRate                        *int                        `json:"audioBitRate"`
	AudioChannels                       *int                        `json:"audioChannels"`
	MaxAudioChannels                    *int                        `json:"maxAudioChannels"`
	Static                              bool                        `json:"static"`
	Profile                             string                      `json:"profile"`
	VideoRangeType                      string                      `json:"videoRangeType"`
	Level                               string                      `json:"level"`
	CodecTag                            string                      `json:"codecTag"`
	Framerate                           *float32                    `json:"framerate"`
	MaxFramerate                        *float32                    `json:"maxFramerate"`
	CopyTimestamps                      bool                        `json:"copyTimestamps"`
	StartTimeTicks                      *int64                      `json:"startTimeTicks"`
	Width                               *int                        `json:"width"`
	Height                              *int                        `json:"height"`
	MaxWidth                            *int                        `json:"maxWidth"`
	MaxHeight                           *int                        `json:"maxHeight"`
	VideoBitRate                        *int                        `json:"videoBitRate"`
	SubtitleStreamIndex                 *int                        `json:"subtitleStreamIndex"`
	SubtitleMethod                      dlna.SubtitleDeliveryMethod `json:"subtitleMethod"`
	MaxRefFrames                        *int                        `json:"maxRefFrames"`
	MaxVideoBitDepth                    *int                        `json:"maxVideoBitDepth"`
	RequireAvc                          bool                        `json:"requireAvc"`
	DeInterlace                         bool                        `json:"deInterlace"`
	RequireNonAnamorphic                bool                        `json:"requireNonAnamorphic"`
	TranscodingMaxAudioChannels         *int                        `json:"transcodingMaxAudioChannels"`
	CpuCoreLimit                        *int                        `json:"cpuCoreLimit"`
	LiveStreamId                        string                      `json:"liveStreamId"`
	EnableMpegtsM2TsMode                bool                        `json:"enableMpegtsM2TsMode"`
	VideoCodec                          string                      `json:"videoCodec"`
	SubtitleCodec                       string                      `json:"subtitleCodec"`
	TranscodeReasons                    string                      `json:"transcodeReasons"`
	AudioStreamIndex                    *int                        `json:"audioStreamIndex"`
	VideoStreamIndex                    *int                        `json:"videoStreamIndex"`
	Context                             dlna.EncodingContext        `json:"context"`
	StreamOptions                       map[string]string           `json:"streamOptions"`
	EnableAudioVbrEncoding              bool                        `json:"enableAudioVbrEncoding"`
	AlwaysBurnInSubtitleWhenTranscoding bool                        `json:"alwaysBurnInSubtitleWhenTranscoding"`
}

func (o *BaseEncodingJobOptions) GetOption(qualifier, name string) string {
	key := qualifier + "-" + name
	if value, ok := o.StreamOptions[key]; ok {
		return value
	}
	if value, ok := o.StreamOptions[name]; ok {
		return value
	}
	return ""
}
