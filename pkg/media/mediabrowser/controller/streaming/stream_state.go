package streaming

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/model/dlna"
)

type StreamState struct {
	mediaencoding.EncodingJobInfo
	//    _mediaSourceManager   IMediaSourceManager
	//	transcodeManager     *transcoding.TranscodeManager
	//	transcodeManager     interface{}
	//    _disposed             bool

	RequestedUrl  string
	Request       StreamingRequestDto
	VideoRequest  *VideoRequestDto
	IsOutputVideo bool
	//    DirectStreamProvider  IDirectStreamProvider
	WaitForPath           string
	UserAgent             *string
	EstimateContentLength bool
	TranscodeSeekInfo     dlna.TranscodeSeekInfo
	TranscodingJob        *mediaencoding.TranscodingJob

	disposed bool
}

func (s *StreamState) GetRequest() StreamingRequestDto {
	return StreamingRequestDto{
		BaseEncodingJobOptions: s.BaseRequest,
	}
}

func (s *StreamState) SetRequest(request *StreamingRequestDto) {
	fmt.Printf("------------------->>>>>>>>>>>>>%+v\n", request)
	fmt.Printf("-----------------------============== %+v\n", s.VideoRequest)
	s.BaseRequest = request.BaseEncodingJobOptions
	s.IsVideoRequest = s.VideoRequest != nil
}

/*
func (s *StreamState) IsOutputVideo() bool {
    _, ok := s.Request.(VideoRequestDto)
    return ok
}
*/

/*
func (s *StreamState) GetVideoRequest() *VideoRequestDto {
    streamingRequest := s.GetRequest()
    return streamingRequest.GetVideoRequest()
}
*/

/*
func NewStreamState(mediaSourceManager *MediaSourceManager, transcodingType TranscodingJobType, transcodeManager *TranscodeManager) *StreamState {
    return &StreamState{
        EncodingJobInfo: EncodingJobInfo{TranscodingType: transcodingType},
        mediaSourceManager: mediaSourceManager,
        transcodeManager: transcodeManager,
    }
}
*/

func (s *StreamState) Dispose() {
	s.Dispose2(true)
	runtime.GC()
}
func (s *StreamState) Dispose2(disposing bool) {
	if s.disposed {
		return
	}

	if disposing {
		if s.MediaSource.RequiresClosing &&
			(s.Request.LiveStreamId == "" && s.MediaSource.LiveStreamID != "") {
			//compile
			//            s.mediaSourceManager.CloseLiveStream(s.MediaSource.LiveStreamId)
		}
	}

	s.TranscodingJob = nil
	s.disposed = true
}

func (s *StreamState) ReportTranscodingProgress(transcodingPosition *time.Duration, framerate *float32, percentComplete *float64, bytesTranscoded *int64, bitRate *int) {
	fmt.Println("StreamState ReportTranscodingProgress.........................................................................")
	var ticks int64
	if transcodingPosition != nil {
		ticks = transcodingPosition.Nanoseconds() / 100
		fmt.Println(ticks)
	}

	job := s.TranscodingJob
	if job != nil {
		job.Framerate = framerate
		job.CompletionPercentage = percentComplete
		fmt.Println(ticks)
		job.TranscodingPositionTicks = &ticks
		job.BytesTranscoded = bytesTranscoded
		job.BitRate = bitRate
	}
	// transcodeManager := s.transcodeManager.(TranscodeManager)
	// transcodeManager.ReportTranscodingProgress(s.TranscodingJob, s, transcodingPosition, framerate, percentComplete, bytesTranscoded, bitRate)
}

func (s *StreamState) SegmentLength() int {
	if s.Request.SegmentLength != nil {
		return *s.Request.SegmentLength
	}

	if mediaencoding.IsCopyCodec(s.OutputVideoCodec) {
		userAgent := ""
		if s.UserAgent != nil {
			userAgent = *s.UserAgent
		}

		if strings.Contains(strings.ToLower(userAgent), "appletv") ||
			strings.Contains(strings.ToLower(userAgent), "cfnetwork") ||
			strings.Contains(strings.ToLower(userAgent), "ipad") ||
			strings.Contains(strings.ToLower(userAgent), "iphone") ||
			strings.Contains(strings.ToLower(userAgent), "ipod") {
			return 6
		}

		if s.IsSegmentedLiveStream() {
			return 3
		}

		return 6
	}

	return 3
}
