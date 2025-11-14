package mediaencoding

import (
	"mime"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/drawing"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
	"files/pkg/media/mediabrowser/model/mediainfo/transportstreamtimestamp"
	"files/pkg/media/mediabrowser/model/session"
	//"files/pkg/media/jellyfin/data/enums"

	"k8s.io/klog/v2"
)

type EncodingJobInfo struct {
	Headers             string
	OutputAudioBitrate  *int
	OutputAudioChannels *int

	TranscodeReasons *session.TranscodeReason
	// compile
	//    Progress               Progress
	VideoStream                  *entities.MediaStream
	VideoType                    entities.VideoType
	RemoteHttpHeaders            map[string]string
	OutputVideoCodec             string
	InputProtocol                mediaprotocol.MediaProtocol
	MediaPath                    string
	IsInputVideo                 bool
	OutputAudioCodec             string
	OutputVideoBitrate           *int
	SubtitleStream               *entities.MediaStream
	SubtitleDeliveryMethod       dlna.SubtitleDeliveryMethod
	SupportedSubtitleCodecs      []string
	InternalSubtitleStreamOffset int
	MediaSource                  *dto.MediaSourceInfo
	// compile
	//    User                   User
	RunTimeTicks               *int64
	ReadInputAtNativeFramerate bool
	OutputFilePath             string
	MimeType                   string
	IgnoreInputDts             bool
	IgnoreInputIndex           bool
	GenPtsInput                bool
	DiscardCorruptFramesInput  bool
	EnableFastSeekInput        bool
	GenPtsOutput               bool
	OutputContainer            string
	OutputVideoSync            string
	AlbumCoverPath             string
	InputAudioSync             string
	InputVideoSync             string
	InputTimestamp             transportstreamtimestamp.TransportStreamTimestamp
	AudioStream                *entities.MediaStream
	SupportedAudioCodecs       []string
	SupportedVideoCodecs       []string
	InputContainer             string
	IsoType                    *entities.IsoType
	BaseRequest                *BaseEncodingJobOptions
	IsVideoRequest             bool
	TranscodingType            TranscodingJobType
	StartTimeTicks             *int64
	CopyTimestamps             bool
	TotalOutputBitrate         *int
	OutputWidth                *int
}

func NewEncodingJobInfo(jobType TranscodingJobType) *EncodingJobInfo {
	return &EncodingJobInfo{
		RemoteHttpHeaders:       make(map[string]string),
		SupportedAudioCodecs:    []string{},
		SupportedVideoCodecs:    []string{},
		SupportedSubtitleCodecs: []string{},
		TranscodingType:         jobType,
	}
}

func ParseTranscodeReason(value string) (session.TranscodeReason, error) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return session.ContainerNotSupported, err
	}
	return session.TranscodeReason(i), nil
}

func (e *EncodingJobInfo) GetTranscodeReasons() session.TranscodeReason {
	var transcodeReasons session.TranscodeReason = session.ContainerNotSupported
	if e.TranscodeReasons == nil {
		if e.BaseRequest == nil || e.BaseRequest.TranscodeReasons == "" {
			e.TranscodeReasons = &transcodeReasons
			return *e.TranscodeReasons
		}

		if reason, err := ParseTranscodeReason(e.BaseRequest.TranscodeReasons); err == nil {
			e.TranscodeReasons = &reason
		} else {
			e.TranscodeReasons = &transcodeReasons
		}
	}

	return *e.TranscodeReasons
}

func (e *EncodingJobInfo) GetOutputWidth() *int {
	if e.VideoStream != nil && e.VideoStream.Width != nil && e.VideoStream.Height != nil {
		size := drawing.ImageDimensions{
			Width:  *e.VideoStream.Width,
			Height: *e.VideoStream.Height,
		}

		newSize := drawing.Resize(
			size,
			*e.BaseRequest.Width,
			*e.BaseRequest.Height,
			*e.BaseRequest.MaxWidth,
			*e.BaseRequest.MaxHeight,
		)

		return &newSize.Width
	}

	if !e.IsVideoRequest {
		return nil
	}

	return e.BaseRequest.MaxWidth
}

func orElse[T any](t *T, defaultValue T) T {
	if t != nil {
		return *t
	}
	return defaultValue
}

func (e *EncodingJobInfo) OutputHeight() *int {
	if e.VideoStream != nil && e.VideoStream.Width != nil && e.VideoStream.Height != nil {
		size := drawing.ImageDimensions{
			Width:  *e.VideoStream.Width,
			Height: *e.VideoStream.Height,
		}
		newSize := drawing.Resize(
			size,
			orElse(e.BaseRequest.Width, 0),
			orElse(e.BaseRequest.Height, 0),
			orElse(e.BaseRequest.MaxWidth, 0),
			orElse(e.BaseRequest.MaxHeight, 0),
		)
		return &newSize.Height
	}

	if !e.IsVideoRequest {
		return nil
	}

	if e.BaseRequest.MaxHeight != nil {
		return e.BaseRequest.MaxHeight
	} else {
		return e.BaseRequest.Height
	}
}

func (e *EncodingJobInfo) OutputAudioSampleRate() *int {
	if e.BaseRequest.Static || IsCopyCodec(e.OutputAudioCodec) {
		if e.AudioStream != nil {
			return e.AudioStream.SampleRate
		}
	} else if e.BaseRequest.AudioSampleRate != nil {
		return e.BaseRequest.AudioSampleRate
	}
	return nil
}

func (e *EncodingJobInfo) OutputAudioBitDepth() *int {
	/* compile
	   if e.BaseRequest.Static || EncodingHelper_IsCopyCodec(e.OutputAudioCodec) {
	       if e.AudioStream != nil {
	           return e.AudioStream.BitDepth
	       }
	   }
	*/
	return nil
}

func (e *EncodingJobInfo) TargetVideoLevel() *float64 {
	/*
	   if e.BaseRequest.Static || EncodingHelper_IsCopyCodec(e.OutputVideoCodec) {
	       if e.VideoStream != nil {
	           return e.VideoStream.Level
	       }
	   } else {
	       level := GetRequestedLevel(e.ActualOutputVideoCodec())
	       var result float64
	       if _, err := fmt.Sscanf(level, "%f", &result); err == nil {
	           return &result
	       }
	   }
	*/
	return nil
}

func (e *EncodingJobInfo) TargetVideoBitDepth() *int {
	/*
	   if e.BaseRequest.Static || EncodingHelper_IsCopyCodec(e.OutputVideoCodec) {
	       if e.VideoStream != nil {
	           return e.VideoStream.BitDepth
	       }
	   }
	*/
	return nil
}

func (e *EncodingJobInfo) TargetRefFrames() *int {
	if e.BaseRequest.Static || IsCopyCodec(e.OutputVideoCodec) {
		if e.VideoStream != nil {
			return e.VideoStream.RefFrames
		}
	}
	return nil
}

func (e *EncodingJobInfo) TargetFramerate() *float32 {
	if e.BaseRequest.Static || IsCopyCodec(e.OutputVideoCodec) {
		if e.VideoStream != nil {
			if e.VideoStream.AverageFrameRate != nil {
				return e.VideoStream.AverageFrameRate
			} else {
				return e.VideoStream.RealFrameRate
			}
		} else {
			return nil
		}
	} else {
		if e.BaseRequest.MaxFramerate != nil {
			return e.BaseRequest.MaxFramerate
		} else {
			return e.BaseRequest.Framerate
		}
	}
}

func (e *EncodingJobInfo) TargetTimestamp() transportstreamtimestamp.TransportStreamTimestamp {
	if e.BaseRequest.Static {
		return e.InputTimestamp
	}

	if strings.EqualFold(e.OutputContainer, "m2ts") {
		return transportstreamtimestamp.Valid
	} else {
		return transportstreamtimestamp.None
	}
}

func (e *EncodingJobInfo) TargetPacketLength() *int {
	/*
		    if e.BaseRequest.Static || EncodingHelper_IsCopyCodec(e.OutputVideoCodec) {
		//        if e.VideoStream != nil {
		            return e.VideoStream.PacketLength
		 //       }
		    }
	*/
	return nil
}

func (e *EncodingJobInfo) TargetVideoProfile() string {
	/*
		    if e.BaseRequest.Static || EncodingHelper_IsCopyCodec(e.OutputVideoCodec) {
		//        if e.VideoStream != nil {
		            return e.VideoStream.Profile
		 //       }
		    } else {
		        requestedProfiles := GetRequestedProfiles(e.ActualOutputVideoCodec())
		        if len(requestedProfiles) > 0 {
		            return requestedProfiles[0]
		        }
		    }
	*/
	return ""
}

/*
func (e *EncodingJobInfo) TargetVideoRangeType() session.VideoRangeType {
    if e.BaseRequest.Static || EncodingHelper_IsCopyCodec(e.OutputVideoCodec) {
//        if e.VideoStream != nil {
            return e.VideoStream.VideoRangeType
 //       }
        return enums.Unknown2
    } else {
        requestedRangeTypes := GetRequestedRangeTypes(e.ActualOutputVideoCodec())
        if len(requestedRangeTypes) > 0 {
            var requestedRangeType VideoRangeType
            if err := fmt.Sscanf(requestedRangeTypes[0], "%s", &requestedRangeType); err == nil {
                return requestedRangeType
            }
        }
        return enums.Unknown2
    }
}
*/

func (e *EncodingJobInfo) TargetVideoCodecTag() string {
	/*
	   if e.BaseRequest.Static || EncodingHelper_IsCopyCodec(e.OutputVideoCodec) {
	       if e.VideoStream != nil {
	           return e.VideoStream.CodecTag
	       }
	   }
	*/
	return ""
}

func (e *EncodingJobInfo) ActualOutputVideoCodec() string {
	if e.VideoStream == nil {
		return ""
	}

	if IsCopyCodec(e.OutputVideoCodec) {
		return e.VideoStream.Codec
	}

	return e.OutputVideoCodec
}

func (e *EncodingJobInfo) GetRequestedAudioChannels(codec string) *int {
	if codec != "" {
		value := e.BaseRequest.GetOption(codec, "audiochannels")
		if result, err := strconv.Atoi(value); err == nil {
			klog.Infoln("########################>>>>>>>>>>>>>>>>>>>>>>>>>", result)
			return &result
		}
	}

	if e.BaseRequest.MaxAudioChannels != nil {
		return e.BaseRequest.MaxAudioChannels
	}

	if e.BaseRequest.AudioChannels != nil {
		return e.BaseRequest.AudioChannels
	}

	if e.BaseRequest.TranscodingMaxAudioChannels != nil {
		return e.BaseRequest.TranscodingMaxAudioChannels
	}

	return nil
}

func (e *EncodingJobInfo) GetRequestedProfiles(codec string) []string {
	if e.BaseRequest.Profile != "" {
		return strings.Split(e.BaseRequest.Profile, "|,")
	}

	if codec != "" {
		profile := e.BaseRequest.GetOption(codec, "profile")
		if profile != "" {
			return strings.Split(profile, "|,")
		}
	}

	return []string{}
}

func (e *EncodingJobInfo) DeInterlace(videoCodec string, forceDeinterlaceIfSourceIsInterlaced bool) bool {
	var isInputInterlaced bool
	if e.VideoStream != nil {
		isInputInterlaced = e.VideoStream.IsInterlaced
	}

	if !isInputInterlaced {
		return false
	}

	// Support general param
	if e.BaseRequest.DeInterlace {
		return true
	}

	if videoCodec != "" {
		deinterlaceOption := e.BaseRequest.GetOption(videoCodec, "deinterlace")
		if strings.EqualFold(deinterlaceOption, "true") {
			return true
		}
	}

	return forceDeinterlaceIfSourceIsInterlaced
}

func (e *EncodingJobInfo) GetRequestedRangeTypes(codec string) []string {
	if e.BaseRequest.VideoRangeType != "" {
		return strings.Split(e.BaseRequest.VideoRangeType, "|,")
	}

	if codec != "" {
		rangeType := e.BaseRequest.GetOption(codec, "rangetype")
		if rangeType != "" {
			return strings.Split(rangeType, "|,")
		}
	}

	return []string{}
}

func (e *EncodingJobInfo) GetRequestedVideoBitDepth(codec string) *int {
	if e.BaseRequest.MaxVideoBitDepth != nil {
		return e.BaseRequest.MaxVideoBitDepth
	}

	if codec != "" {
		value := e.BaseRequest.GetOption(codec, "videobitdepth")
		if result, err := strconv.Atoi(value); err == nil {
			return &result
		}
	}

	return nil
}

func (e *EncodingJobInfo) GetRequestedMaxRefFrames(codec string) *int {
	if e.BaseRequest.MaxRefFrames != nil {
		return e.BaseRequest.MaxRefFrames
	}

	if codec != "" {
		value := e.BaseRequest.GetOption(codec, "maxrefframes")
		if result, err := strconv.Atoi(value); err == nil {
			return &result
		}
	}

	return nil
}

func (e *EncodingJobInfo) GetRequestedLevel(codec string) string {
	if e.BaseRequest.Level != "" {
		return e.BaseRequest.Level
	}

	if codec != "" {
		value := e.BaseRequest.GetOption(codec, "level")
		return value
	}

	return ""
}

func (e *EncodingJobInfo) GetRequestedAudioBitDepth(codec string) *int {
	if e.BaseRequest.MaxAudioBitDepth != nil {
		return e.BaseRequest.MaxAudioBitDepth
	}

	if codec != "" {
		value := e.BaseRequest.GetOption(codec, "audiobitdepth")
		if result, err := strconv.Atoi(value); err == nil {
			return &result
		}
	}

	return nil
}

func (e *EncodingJobInfo) IsSegmentedLiveStream() bool {
	return e.TranscodingType != Progressive && e.RunTimeTicks == nil
}

func (e *EncodingJobInfo) EnableBreakOnNonKeyFrames(videoCodec string) bool {
	if e.TranscodingType != Progressive {
		if e.IsSegmentedLiveStream() {
			return false
		}
		return e.BaseRequest.BreakOnNonKeyFrames && IsCopyCodec(videoCodec)
	}
	return false
}
func (e *EncodingJobInfo) ActualOutputAudioCodec() string {
	if e.AudioStream == nil {
		return ""
	}

	if IsCopyCodec(e.OutputAudioCodec) {
		return e.AudioStream.Codec
	}

	return e.OutputAudioCodec
}

func (e *EncodingJobInfo) ReportTranscodingProgress(
	transcodingPosition *time.Duration,
	framerate *float32,
	percentComplete *float64,
	bytesTranscoded *int64,
	bitRate *int,
) {
	klog.Infoln("EncodingJobInfo ..................................$$$$$$$$$$$$$$$$$$$......... ReportTranscodingProgress")
	if percentComplete != nil {
		//        e.Progress.Report(*percentComplete)
	}
}

func (e *EncodingJobInfo) GetMimeType(outputPath string, enableStreamDefault bool) string {
	if e.MimeType != "" {
		return e.MimeType
	}

	if enableStreamDefault {
		return mime.TypeByExtension(filepath.Ext(outputPath))
	}

	return mime.TypeByExtension(filepath.Ext(outputPath))
}
