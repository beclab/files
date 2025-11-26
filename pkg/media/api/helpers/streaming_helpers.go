package helpers

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"files/pkg/common"
	"files/pkg/models"

	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"

	//	"files/pkg/media/api/models/streamingdtos"
	"files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/controller/mediaencoding/transcodemanager"
	"files/pkg/media/mediabrowser/controller/streaming"
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/utils"

	"github.com/cloudwego/hertz/pkg/app"
	"k8s.io/klog/v2"
)

func GetPathProtocol(path string) mediaprotocol.MediaProtocol {
	switch {
	case strings.HasPrefix(strings.ToLower(path), "rtsp"):
		return mediaprotocol.Rtsp
	case strings.HasPrefix(strings.ToLower(path), "rtmp"):
		return mediaprotocol.Rtmp
	case strings.HasPrefix(strings.ToLower(path), "http"):
		return mediaprotocol.Http
	case strings.HasPrefix(strings.ToLower(path), "rtp"):
		return mediaprotocol.Rtp
	case strings.HasPrefix(strings.ToLower(path), "ftp"):
		return mediaprotocol.Ftp
	case strings.HasPrefix(strings.ToLower(path), "udp"):
		return mediaprotocol.Udp
	default:
		/*
		   if isPathFile(path) {
		       return MediaProtocol_File
		   }
		   return MediaProtocol_Http
		*/
		return mediaprotocol.File
	}
}

// func GetStreamingState(
//
//	//    streamingRequest *streaming.StreamingRequestDto,
//	//    streamingRequest interface{},
//	request interface{},
//	httpContext *http.Request,
//	//    mediaSourceManager MediaSourceManager,
//	//    userManager UserManager,
//	//    libraryManager LibraryManager,
//	serverConfigurationManager configuration.IServerConfigurationManager,
//	mediaEncoder mediaencoding.IMediaEncoder,
//	encodingHelper mediaencoding.EncodingHelper,
//	transcodeManager transcodemanager.ITranscodeManager,
//	transcodingJobType mediaencoding.TranscodingJobType,
//	ctx context.Context,
//
// ) (*streaming.StreamState, error) {
func GetStreamingState(
	request interface{},
	httpContext *app.RequestContext,
	serverConfigurationManager configuration.IServerConfigurationManager,
	mediaEncoder mediaencoding.IMediaEncoder,
	encodingHelper mediaencoding.EncodingHelper,
	transcodeManager transcodemanager.ITranscodeManager,
	transcodingJobType mediaencoding.TranscodingJobType,
	ctx context.Context,
) (*streaming.StreamState, error) {

	streamingRequest, ok := request.(*streaming.VideoRequestDto)
	if !ok {
		klog.Warningf("[media] GetStreamingState, parse request invalid")
	}
	klog.Infof("[media] GetStreamingState, request: %v, segmentContainer: %s", common.ParseString(streamingRequest), *streamingRequest.SegmentContainer)
	if streamingRequest.Params != nil && *streamingRequest.Params != "" {
		ParseParams(streamingRequest)
	}

	streamingRequest.StreamOptions = ParseStreamOptions(httpContext) // httpContext.URL.Query()
	// if httpContext.URL.Path == "" {
	if len(httpContext.Path()) == 0 {
		return nil, errors.New("resource not found")
	}

	urlRequest := filepath.Ext(string(httpContext.Path()))

	if streamingRequest.AudioCodec == "" {
		streamingRequest.AudioCodec = encodingHelper.InferAudioCodec(urlRequest)
	}

	userAgent := string(httpContext.GetHeader("User-Agent"))
	state := &streaming.StreamState{
		Request:      *streamingRequest.StreamingRequestDto,
		RequestedUrl: urlRequest,
		UserAgent:    &userAgent,
		VideoRequest: streamingRequest,
	}
	state.TranscodingType = transcodingJobType
	state.SetRequest(streamingRequest.StreamingRequestDto)

	if state.IsVideoRequest && streamingRequest.VideoCodec != "" {
		state.SupportedVideoCodecs = strings.Split(streamingRequest.VideoCodec, ",")
		var filtered []string
		for _, codec := range state.SupportedVideoCodecs {
			if codec != "" {
				filtered = append(filtered, codec)
			}
		}
		state.SupportedVideoCodecs = filtered
		if len(state.SupportedVideoCodecs) > 0 {
			state.BaseRequest.VideoCodec = state.SupportedVideoCodecs[0]
		}
	}

	if streamingRequest.AudioCodec != "" {
		state.SupportedAudioCodecs = strings.Split(streamingRequest.AudioCodec, ",")
		state.BaseRequest.AudioCodec = func(codecs []string, mediaEncoder mediaencoding.IMediaEncoder) string {
			for _, codec := range codecs {
				if mediaEncoder.CanEncodeToAudioCodec(codec) {
					return codec
				}
			}
			if len(codecs) > 0 {
				return codecs[0]
			}
			return ""
		}(state.SupportedAudioCodecs, mediaEncoder)
	}

	if streamingRequest.SubtitleCodec != "" {
		state.SupportedSubtitleCodecs = strings.Split(streamingRequest.SubtitleCodec, ",")
		state.BaseRequest.SubtitleCodec = func(codecs []string, mediaEncoder mediaencoding.IMediaEncoder) string {
			for _, codec := range codecs {
				if mediaEncoder.CanEncodeToSubtitleCodec(codec) {
					return codec
				}
			}
			if len(codecs) > 0 {
				return codecs[0]
			}
			return ""
		}(state.SupportedSubtitleCodecs, mediaEncoder)
	}

	state.IsInputVideo = true
	protocol := GetPathProtocol(streamingRequest.PlayPath)
	var headers string
	if protocol == mediaprotocol.Http {
		// bflName := httpContext.Header[common.REQUEST_HEADER_OWNER][0]
		bflName := httpContext.Request.Header.Get(common.REQUEST_HEADER_OWNER)
		authToken, err := utils.GetAuthToken(bflName)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		klog.Infof("[media] protocol, authToken: %s", authToken)
		fileParam, err := models.CreateFileParam(bflName, httpContext.Query("PlayPath"))
		if err != nil {
			klog.Infof("[media] GetStreamingState, parse url error: %v\n", err)
			return nil, errors.New("parse url error")
		}

		if fileParam.FileType == common.Share { // todo share

		} else if fileParam.FileType == common.Sync {
			remoteAccessToken := httpContext.Request.Header.Get("remote-accesstoken")
			headers = fmt.Sprintf("remote-accesstoken: %s", remoteAccessToken)
		} else if fileParam.FileType == common.GoogleDrive {
			accountResp, err := utils.GetToken(bflName, fileParam.Extend, fileParam.FileType, authToken)
			if err != nil {
				return nil, err
			}
			headers = "Authorization: Bearer " + accountResp.RawData.AccessToken
		} else if fileParam.FileType == common.DropBox {
			accountResp, err := utils.GetToken(bflName, fileParam.Extend, fileParam.FileType, authToken)
			if err != nil {
				return nil, err
			}

			headers = fmt.Sprintf("Authorization: Bearer %s", accountResp.RawData.AccessToken)
		} else if fileParam.FileType == common.AwsS3 {
		}

		klog.Infof("[media] GetStreamingState, headers: %s", headers)

		state.Headers = headers
	}

	var mediaSource *dto.MediaSourceInfo = nil
	if streamingRequest.LiveStreamId == "" {
		currentJob := transcodeManager.GetTranscodingJob(*streamingRequest.PlaySessionID)
		if currentJob != nil {
			mediaSource = currentJob.MediaSource
		}

		if mediaSource == nil {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var videoType = entities.VideoFile
			mediaInfo, err := mediaEncoder.GetMediaInfo(&mediaencoding.MediaInfoRequest{
				MediaSource: &dto.MediaSourceInfo{
					Protocol:  protocol,
					Path:      streamingRequest.PlayPath,
					VideoType: &videoType,
				},
				ExtractChapters: false,
				MediaType:       dlna.Video,
			}, ctx, headers)
			if err != nil {
				klog.Errorf("[media] GetStreamingState, get media info error: %v, playPath: %s", err, streamingRequest.PlayPath)
				return nil, err
			}
			klog.Infof("[media] GetStreamingState, get media info: %s", common.ParseString(mediaInfo))
			mediaSource = &mediaInfo.MediaSourceInfo
		}
	} else {
		// Enforce more restrictive transcoding profile for LiveTV due to compatability reasons
		// Cap the MaxStreamingBitrate to 30Mbps, because we are unable to reliably probe source bitrate,
		// which will cause the client to request extremely high bitrate that may fail the player/encoder
		if *streamingRequest.VideoBitRate > 30000000 {
			*streamingRequest.VideoBitRate = 30000000
		}

		if streamingRequest.SegmentContainer != nil {
			// Remove all fmp4 transcoding profiles, because it causes playback error and/or A/V sync issues
			// Notably: Some channels won't play on FireFox and LG webOS
			// Some channels from HDHomerun will experience A/V sync issues

			*streamingRequest.SegmentContainer = "ts"
			streamingRequest.VideoCodec = "h264"
		}
	}

	encodingOptions := serverConfigurationManager.GetEncodingOptions()
	encodingHelper.AttachMediaSourceInfo(&state.EncodingJobInfo, encodingOptions, mediaSource, urlRequest)

	containerInternal := filepath.Ext(state.RequestedUrl)
	if streamingRequest.Container != "" {
		containerInternal = streamingRequest.Container
	}

	if containerInternal == "" {
		if streamingRequest.Static {
			containerInternal = dlna.NormalizeMediaSourceFormatIntoSingleContainer(state.InputContainer, nil, dlna.Audio, nil)
		} else {
			containerInternal, _ = GetOutputFileExtension(state, mediaSource)
		}
	}

	outputAudioCodec := streamingRequest.AudioCodec
	state.OutputAudioCodec = outputAudioCodec
	state.OutputContainer = strings.TrimPrefix(containerInternal, ".")

	state.OutputAudioChannels = encodingHelper.GetNumAudioChannelsParam(state.EncodingJobInfo, state.AudioStream, state.OutputAudioCodec)
	if state.OutputAudioChannels != nil {
		klog.Infof("OutputAudioChannels: %d\n", *state.OutputAudioChannels)
	}

	outputAudioBitrate := 0
	if slices.Contains(mediaencoding.LosslessAudioCodecs, outputAudioCodec) {
		if state.AudioStream.BitRate != nil {
			state.OutputAudioBitrate = state.AudioStream.BitRate

		} else {
			state.OutputAudioBitrate = &outputAudioBitrate
		}

	} else {
		audioBitrate := encodingHelper.GetAudioBitrateParam(streamingRequest.AudioBitRate, streamingRequest.AudioCodec, state.AudioStream, state.OutputAudioChannels)
		if audioBitrate != nil {
			state.OutputAudioBitrate = audioBitrate
		} else {
			state.OutputAudioBitrate = &outputAudioBitrate
		}
	}

	if strings.HasPrefix(outputAudioCodec, "pcm_") {
		containerInternal = ".pcm"
	}

	if state.VideoRequest != nil {
		state.OutputVideoCodec = state.BaseRequest.VideoCodec
		outputVideoBitrate := encodingHelper.GetVideoBitrateParamValue(state.VideoRequest.StreamingRequestDto.BaseEncodingJobOptions, state.VideoStream, state.OutputVideoCodec)
		klog.Infof("outputVideoBitrate: ", outputVideoBitrate)
		state.OutputVideoBitrate = &outputVideoBitrate

		encodingHelper.TryStreamCopy(state.EncodingJobInfo)

		if !mediaencoding.IsCopyCodec(state.OutputVideoCodec) && state.OutputVideoBitrate != nil {

			isVideoResolutionNotRequested := state.VideoRequest.Width == nil && state.VideoRequest.Height == nil && state.VideoRequest.MaxWidth == nil && state.VideoRequest.MaxHeight == nil
			if isVideoResolutionNotRequested && state.VideoStream != nil && state.VideoRequest.VideoBitRate != nil && *state.VideoRequest.VideoBitRate >= *state.VideoStream.BitRate {
				// Don't downscale the resolution if the width/height/MaxWidth/MaxHeight is not requested,
				// and the requested video bitrate is higher than source video bitrate.
				if *state.VideoStream.Width != 0 || *state.VideoStream.Height != 0 {
					state.VideoRequest.MaxWidth = state.VideoStream.Width
					state.VideoRequest.MaxHeight = state.VideoStream.Height
				}
			} else {
				var inputBitRate *int
				if state.VideoStream != nil {
					inputBitRate = state.VideoStream.BitRate
				}
				h264EquivalentBitrate := mediaencoding.ScaleBitrate(*state.OutputVideoBitrate, state.ActualOutputVideoCodec(), "h264")
				resolution := dlna.Normalize(inputBitRate, *state.OutputVideoBitrate, h264EquivalentBitrate, state.VideoRequest.MaxWidth, state.VideoRequest.MaxHeight, state.TargetFramerate(), false)
				state.VideoRequest.MaxWidth = resolution.MaxWidth
				state.VideoRequest.MaxHeight = resolution.MaxHeight
			}
		}
	}

	var ext string
	if state.OutputContainer == "" {
		ext, _ = GetOutputFileExtension(state, mediaSource)
	} else {
		ext = "." + GetContainerFileExtension(state.OutputContainer)
	}

	//state.OutputFilePath = GetOutputFilePath(state, ext, serverConfigurationManager, streamingRequest.DeviceId, *streamingRequest.PlaySessionId)
	state.OutputFilePath = GetOutputFilePath(state, ext, serverConfigurationManager, streamingRequest.DeviceID, *streamingRequest.PlaySessionID)

	return state, nil
}

// func ParseStreamOptions(queryString url.Values) map[string]string {
func ParseStreamOptions(c *app.RequestContext) map[string]string {
	streamOptions := make(map[string]string)
	c.VisitAllQueryArgs(func(key, value []byte) {
		if len(string(key)) > 0 && islower(string(key)[0]) {
			streamOptions[string(key)] = string(value)
		}
	})

	// for key, value := range queryString {
	// 	if len(key) > 0 && islower(key[0]) {
	// 		// This was probably not parsed initially and should be a StreamOptions
	// 		// or the generated URL should correctly serialize it
	// 		// TODO: This should be incorporated either in the lower framework for parsing requests
	// 		streamOptions[key] = value[0]
	// 	}
	// }
	return streamOptions
}

func islower(b byte) bool {
	return 'a' <= b && b <= 'z'
}

func GetOutputFileExtension(state *streaming.StreamState, mediaSource *dto.MediaSourceInfo) (string, error) {
	ext := filepath.Ext(state.RequestedUrl)
	if ext != "" {
		return ext, nil
	}

	// Try to infer based on the desired video codec
	if state.IsVideoRequest {
		switch state.Request.VideoCodec {
		case "h264":
			return ".ts", nil
		case "hevc", "av1":
			return ".mp4", nil
		case "theora":
			return ".ogv", nil
		case "vp8", "vp9", "vpx":
			return ".webm", nil
		case "wmv":
			return ".asf", nil
		}
	} else {
		// Try to infer based on the desired audio codec
		switch state.Request.AudioCodec {
		case "aac":
			return ".aac", nil
		case "mp3":
			return ".mp3", nil
		case "vorbis":
			return ".ogg", nil
		case "wma":
			return ".wma", nil
		}
	}

	// Fallback to the container of mediaSource
	if mediaSource != nil && mediaSource.Container != "" {
		idx := strings.IndexRune(mediaSource.Container, ',')
		if idx == -1 {
			return "." + strings.TrimSpace(mediaSource.Container), nil
		}
		return "." + strings.TrimSpace(mediaSource.Container[:idx]), nil
	}

	return "", fmt.Errorf("failed to find an appropriate file extension")
}

func GetOutputFilePath(state *streaming.StreamState, outputFileExtension string, serverConfigurationManager configuration.IServerConfigurationManager, deviceId, playSessionId string) string {
	data := fmt.Sprintf("%s-%s-%s-%s", state.MediaPath, *state.UserAgent, deviceId, playSessionId)
	filename := fmt.Sprintf("%x", md5.Sum([]byte(data)))
	//	filename = "0d26134e6af438e180eddfdbf75cb851"
	klog.Infof("[media] GetOutputFilePath, filename: %v data: %v", filename, data)
	ext := strings.ToLower(outputFileExtension)
	//	folder := serverConfigurationManager.GetTranscodePath()
	//	folder := "./cache/transcodes"
	folder := serverConfigurationManager.GetTranscodePath()
	klog.Infof("[media] GetOutputFilePath, folder: %s", filepath.Join(folder, filename+ext))

	return filepath.Join(folder, filename+ext)
}

// func (s *StreamingHelpers) ParseParams(request *streaming.StreamingRequestDto) {
func ParseParams(r interface{}) {
	videoRequest, ok := r.(*streaming.VideoRequestDto)
	request, _ := r.(*streaming.StreamingRequestDto)

	if !ok || *videoRequest.Params == "" {
		return
	}

	klog.Info(*videoRequest.Params)
	vals := strings.Split(*videoRequest.Params, ";")

	for i, val := range vals {
		if strings.TrimSpace(val) == "" {
			continue
		}

		switch i {
		case 0:
			// DeviceProfileId
		case 1:
			request.DeviceID = val
		case 2:
			request.MediaSourceID = val
		case 3:
			request.Static = strings.EqualFold(val, "true")
		case 4:
			if ok {
				videoRequest.VideoCodec = val
			}
		case 5:
			request.AudioCodec = val
		case 6:
			if ok {
				*videoRequest.AudioStreamIndex, _ = strconv.Atoi(val)
			}
		case 7:
			if ok {
				*videoRequest.SubtitleStreamIndex, _ = strconv.Atoi(val)
			}
		case 8:
			if ok {
				*videoRequest.VideoBitRate, _ = strconv.Atoi(val)
			}
		case 9:
			*request.AudioBitRate, _ = strconv.Atoi(val)
		case 10:
			*request.MaxAudioChannels, _ = strconv.Atoi(val)
		case 11:
			if ok {
				//compile
				//                videoRequest.MaxFramerate, _ = strconv.ParseFloat(val, 64)
			}
		case 12:
			if ok {
				*videoRequest.MaxWidth, _ = strconv.Atoi(val)
			}
		case 13:
			if ok {
				*videoRequest.MaxHeight, _ = strconv.Atoi(val)
			}
		case 14:
			*request.StartTimeTicks, _ = strconv.ParseInt(val, 10, 64)
		case 15:
			if ok {
				videoRequest.Level = val
			}
		case 16:
			if ok {
				*videoRequest.MaxRefFrames, _ = strconv.Atoi(val)
			}
		case 17:
			if ok {
				*videoRequest.MaxVideoBitDepth, _ = strconv.Atoi(val)
			}
		case 18:
			if ok {
				videoRequest.Profile = val
			}
		case 19:
			// cabac no longer used
		case 20:
			*request.PlaySessionID = val
		case 21:
			// api_key
		case 22:
			request.LiveStreamId = val
		case 23:
			// Duplicating ItemId because of MediaMonkey
		case 24:
			if ok {
				videoRequest.CopyTimestamps = strings.EqualFold(val, "true")
			}
		case 25:
			if ok && val != "" {
				/* compile
				   var method dlna.SubtitleDeliveryMethod
				   if err := method.UnmarshalText([]byte(val)); err == nil {
				       videoRequest.SubtitleMethod = method
				   }
				*/
			}
		case 26:
			*request.TranscodingMaxAudioChannels, _ = strconv.Atoi(val)
		case 27:
			if ok {
				videoRequest.EnableSubtitlesInManifest = strings.EqualFold(val, "true")
			}
		case 28:
			*request.Tag = val
		case 29:
			if ok {
				videoRequest.RequireAvc = strings.EqualFold(val, "true")
			}
		case 30:
			request.SubtitleCodec = val
		case 31:
			if ok {
				videoRequest.RequireNonAnamorphic = strings.EqualFold(val, "true")
			}
		case 32:
			if ok {
				videoRequest.DeInterlace = strings.EqualFold(val, "true")
			}
		case 33:
			request.TranscodeReasons = val
		}
	}
}

func GetContainerFileExtension(container string) string {
	switch {
	case strings.EqualFold(container, "mpegts"):
		return "ts"
	case strings.EqualFold(container, "matroska"):
		return "mkv"
	default:
		return container
	}
}
