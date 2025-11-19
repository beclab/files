package dto

import (
	"files/pkg/media/jellyfin/data/enums"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
	"files/pkg/media/mediabrowser/model/mediainfo/transportstreamtimestamp"
	"files/pkg/media/mediabrowser/model/session"
)

type MediaSourceInfo struct {
	Protocol                   mediaprotocol.MediaProtocol
	ID                         string
	Path                       string
	EncoderPath                string
	EncoderProtocol            *mediaprotocol.MediaProtocol
	Type                       MediaSourceType
	Container                  string
	Size                       *int64
	Name                       string
	IsRemote                   bool
	ETag                       string
	RunTimeTicks               *int64
	ReadAtNativeFramerate      bool
	IgnoreDts                  bool
	IgnoreIndex                bool
	GenPtsInput                bool
	SupportsTranscoding        bool
	SupportsDirectStream       bool
	SupportsDirectPlay         bool
	IsInfiniteStream           bool
	RequiresOpening            bool
	OpenToken                  string
	RequiresClosing            bool
	LiveStreamID               string
	BufferMs                   *int
	RequiresLooping            bool
	SupportsProbing            bool
	VideoType                  *entities.VideoType
	IsoType                    *entities.IsoType
	Video3DFormat              *entities.Video3DFormat
	MediaStreams               []entities.MediaStream
	MediaAttachments           []entities.MediaAttachment
	Formats                    []string
	Bitrate                    *int
	Timestamp                  *transportstreamtimestamp.TransportStreamTimestamp
	RequiredHttpHeaders        map[string]string
	TranscodingUrl             string
	TranscodingSubProtocol     enums.MediaStreamProtocol
	TranscodingContainer       string
	AnalyzeDurationMs          *int
	TranscodeReasons           session.TranscodeReason
	DefaultAudioStreamIndex    *int
	DefaultSubtitleStreamIndex *int
}

func NewMediaSourceInfo() *MediaSourceInfo {
	return &MediaSourceInfo{
		Formats:              []string{},
		MediaStreams:         []entities.MediaStream{},
		MediaAttachments:     []entities.MediaAttachment{},
		RequiredHttpHeaders:  make(map[string]string),
		SupportsTranscoding:  true,
		SupportsDirectStream: true,
		SupportsDirectPlay:   true,
		SupportsProbing:      true,
	}
}

func (m *MediaSourceInfo) InferTotalBitrate(force bool) {
	if m.MediaStreams == nil {
		return
	}

	if !force && m.Bitrate != nil {
		return
	}

	var bitrate int
	for _, stream := range m.MediaStreams {
		if !stream.IsExternal {
			if stream.BitRate != nil {
				bitrate += *stream.BitRate
			}
		}
	}

	if bitrate > 0 {
		m.Bitrate = &bitrate
	}
}

func (m *MediaSourceInfo) GetDefaultAudioStream(defaultIndex *int) *entities.MediaStream {
	if defaultIndex != nil && *defaultIndex != -1 {
		for _, stream := range m.MediaStreams {
			if stream.Type == entities.MediaStreamTypeAudio && stream.Index == *defaultIndex {
				return &stream
			}
		}
	}

	for _, stream := range m.MediaStreams {
		if stream.Type == entities.MediaStreamTypeAudio && stream.IsDefault {
			return &stream
		}
	}

	for _, stream := range m.MediaStreams {
		if stream.Type == entities.MediaStreamTypeAudio {
			return &stream
		}
	}

	return nil
}

func (m *MediaSourceInfo) GetMediaStream(streamType entities.MediaStreamType, index int) *entities.MediaStream {
	for _, stream := range m.MediaStreams {
		if stream.Type == streamType && stream.Index == index {
			return &stream
		}
	}
	return nil
}

func (m *MediaSourceInfo) GetStreamCount(streamType entities.MediaStreamType) *int {
	var numMatches, numStreams int
	for _, stream := range m.MediaStreams {
		numStreams++
		if stream.Type == streamType {
			numMatches++
		}
	}

	if numStreams == 0 {
		return nil
	}
	return &numMatches
}

func (m *MediaSourceInfo) IsSecondaryAudio(stream *entities.MediaStream) *bool {
	if stream.IsExternal {
		return boolPtr(false)
	}

	for _, currentStream := range m.MediaStreams {
		if currentStream.Type == entities.MediaStreamTypeAudio && !currentStream.IsExternal {
			return boolPtr(currentStream.Index != stream.Index)
		}
	}

	return nil
}

func boolPtr(b bool) *bool {
	return &b
}
