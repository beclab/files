package controllers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	cc "files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/controller/mediaencoding/transcodemanager"
	"files/pkg/media/mediabrowser/controller/streaming"
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/utils"

	//	"files/pkg/media/api/models/streamingdtos"
	"files/pkg/media/api/helpers"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"

	"k8s.io/klog/v2"
)

// VideosController struct with dependencies
type VideosController struct {
	logger *utils.Logger
	//    libraryManager              LibraryManager
	//    userManager                 UserManager
	//    dtoService                  DtoService
	//    mediaSourceManager          MediaSourceManager
	serverConfigurationManager cc.IServerConfigurationManager
	mediaEncoder               mediaencoding.IMediaEncoder
	transcodeManager           transcodemanager.ITranscodeManager
	//    httpClientFactory           HttpClientFactory
	encodingHelper mediaencoding.EncodingHelper
}

func NewVideosController(
	logger *utils.Logger,
	//    libraryManager LibraryManager,
	//    userManager UserManager,
	//    dtoService DtoService,
	//    mediaSourceManager MediaSourceManager,
	serverConfigurationManager cc.IServerConfigurationManager,
	mediaEncoder mediaencoding.IMediaEncoder,
	transcodeManager transcodemanager.ITranscodeManager,
	//    httpClientFactory HttpClientFactory,
	encodingHelper mediaencoding.EncodingHelper,
) *VideosController {
	return &VideosController{
		logger: logger,
		//        libraryManager:              libraryManager,
		//        userManager:                 userManager,
		//        dtoService:                  dtoService,
		//        mediaSourceManager:          mediaSourceManager,
		serverConfigurationManager: serverConfigurationManager,
		mediaEncoder:               mediaEncoder,
		transcodeManager:           transcodeManager,
		//        httpClientFactory:           httpClientFactory,
		encodingHelper: encodingHelper,
	}
}

func (vc *VideosController) GetVideoStreamByContainer(w http.ResponseWriter, r *http.Request) {
	var err error
	vars := mux.Vars(r)
	klog.Infoln(vars)
	itemId, err := uuid.Parse(vars["itemId"])
	if err != nil {
		vc.logger.Warnf("itemId: %v\n", err)
	} else {
		vc.logger.Debugf("itemId: %v\n", itemId)
	}

	container := vars["container"]

	dataDir := os.Getenv("MEDIA_SERVER_DATA_DIR")
	playPath := r.URL.Query().Get("PlayPath")
	playPath = filepath.Clean(playPath)
	if filepath.IsAbs(playPath) {
		playPath = dataDir + playPath
		klog.Infoln(playPath)
	} else {
		http.Error(w, "invalid PlayPath", http.StatusBadRequest)
		return
	}

	vc.logger.Infof(container)
	static, _ := strconv.ParseBool(r.URL.Query().Get("Static"))
	params := r.URL.Query().Get("params")
	tag := r.URL.Query().Get("tag")
	deviceProfileId := r.URL.Query().Get("deviceProfileId")
	klog.Infoln(deviceProfileId)
	playSessionId := r.URL.Query().Get("PlaySessionId")
	segmentContainer := r.URL.Query().Get("SegmentContainer")
	segmentLength, _ := strconv.Atoi(r.URL.Query().Get("SegmentLength"))
	minSegments, _ := strconv.Atoi(r.URL.Query().Get("MinSegments"))
	mediaSourceId := r.URL.Query().Get("MediaSourceId")
	deviceId := r.URL.Query().Get("DeviceId")
	audioCodec := r.URL.Query().Get("AudioCodec")
	enableAutoStreamCopy, _ := strconv.ParseBool(r.URL.Query().Get("EnableAutoStreamCopy"))
	allowVideoStreamCopy, _ := strconv.ParseBool(r.URL.Query().Get("AllowVideoStreamCopy"))
	allowAudioStreamCopy, _ := strconv.ParseBool(r.URL.Query().Get("AllowAudioStreamCopy"))
	breakOnNonKeyFrames, _ := strconv.ParseBool(r.URL.Query().Get("BreakOnNonKeyFrames"))
	audioSampleRate, _ := strconv.Atoi(r.URL.Query().Get("AudioSampleRate"))
	maxAudioBitDepth, _ := strconv.Atoi(r.URL.Query().Get("MaxAudioBitDepth"))
	audioBitRate, _ := strconv.Atoi(r.URL.Query().Get("AudioBitRate"))
	var audioChannels *int
	if _, ok := r.URL.Query()["AudioChannels"]; ok {
		tmp, _ := strconv.Atoi(r.URL.Query().Get("AudioChannels"))
		if err != nil {
			audioChannels = &tmp
		}

	}

	maxAudioChannels, _ := strconv.Atoi(r.URL.Query().Get("MaxAudioChannels"))
	profile := r.URL.Query().Get("Profile")
	level := r.URL.Query().Get("Level")
	//	framerate, _ := strconv.ParseFloat(r.URL.Query().Get("framerate"), 64)
	var framerate *float32
	if fr := r.URL.Query().Get("Framerate"); fr != "" {
		f64, err := strconv.ParseFloat(fr, 32)
		if err != nil {
			f32 := float32(f64)
			framerate = &f32
		}
	}
	var maxFramerate *float32
	if p := r.URL.Query().Get("MaxFramerate"); p != "" {
		f64, err := strconv.ParseFloat(p, 32)
		if err != nil {
			f32 := float32(f64)
			maxFramerate = &f32
		}
	}
	//	maxFramerate, _ := strconv.ParseFloat(r.URL.Query().Get("maxFramerate"), 64)
	copyTimestamps, _ := strconv.ParseBool(r.URL.Query().Get("CopyTimestamps"))
	startTimeTicks, _ := strconv.ParseInt(r.URL.Query().Get("StartTimeTicks"), 10, 64)
	width, _ := strconv.Atoi(r.URL.Query().Get("Width"))
	height, _ := strconv.Atoi(r.URL.Query().Get("Height"))
	maxWidth, _ := strconv.Atoi(r.URL.Query().Get("MaxWidth"))
	maxHeight, _ := strconv.Atoi(r.URL.Query().Get("MaxHeight"))
	videoBitRate, _ := strconv.Atoi(r.URL.Query().Get("VideoBitRate"))
	subtitleStreamIndex, _ := strconv.Atoi(r.URL.Query().Get("SubtitleStreamIndex"))
	subtitleMethod, _ := dlna.ParseSubtitleDeliveryMethod(r.URL.Query().Get("SubtitleMethod"))
	klog.Infoln(r.URL.Query().Get("SubtitleMethod"), "-----------------------30180--------------------------------------", subtitleMethod)
	maxRefFrames, _ := strconv.Atoi(r.URL.Query().Get("MaxRefFrames"))
	maxVideoBitDepth, _ := strconv.Atoi(r.URL.Query().Get("MaxVideoBitDepth"))

	requireAvc, _ := strconv.ParseBool(r.URL.Query().Get("RequireAvc"))
	deInterlace, _ := strconv.ParseBool(r.URL.Query().Get("DeInterlace"))
	requireNonAnamorphic, _ := strconv.ParseBool(r.URL.Query().Get("RequireNonAnamorphic"))
	transcodingMaxAudioChannels, _ := strconv.Atoi(r.URL.Query().Get("TranscodingMaxAudioChannels"))
	cpuCoreLimit, _ := strconv.Atoi(r.URL.Query().Get("CpuCoreLimit"))
	liveStreamId := r.URL.Query().Get("LiveStreamId")
	enableMpegtsM2TsMode, _ := strconv.ParseBool(r.URL.Query().Get("EnableMpegtsM2TsMode"))
	videoCodec := r.URL.Query().Get("VideoCodec")
	validationRegex := regexp.MustCompile(mediaencoding.ValidationRegex)
	if videoCodec != "" {
		matched := validationRegex.MatchString(videoCodec)
		if !matched {
			klog.Infoln("videcodec not match")
		}
	}

	subtitleCodec := r.URL.Query().Get("SubtitleCodec")
	if subtitleCodec != "" {
		matched := validationRegex.MatchString(subtitleCodec)
		if !matched {
			klog.Infoln("subtitlecodec not match")
		}
	}

	transcodeReasons := r.URL.Query().Get("TranscodeReasons")
	audioStreamIndex, _ := strconv.Atoi(r.URL.Query().Get("AudioStreamIndex"))
	videoStreamIndex, _ := strconv.Atoi(r.URL.Query().Get("VideoStreamIndex"))
	contextStr := r.URL.Query().Get("Context")
	var encodingContext dlna.EncodingContext
	if contextStr == "Streaming" {
		encodingContext = dlna.Streaming
	} else {
		encodingContext = dlna.Static
	}

	streamOptions := make(map[string]string)
	for k, v := range r.URL.Query() {
		if strings.HasPrefix(k, "streamOptions.") {
			streamOptions[strings.TrimPrefix(k, "streamOptions.")] = v[0]
		}
	}
	enableAudioVbrEncoding := r.URL.Query().Get("EnableAudioVbrEncoding") != "false"

	request := &streaming.VideoRequestDto{
		StreamingRequestDto: &streaming.StreamingRequestDto{
			BaseEncodingJobOptions: &mediaencoding.BaseEncodingJobOptions{
				PlayPath: playPath,
				Id:       itemId,
				Static:   static,
				//		Params:                         params,
				//		Tag:                            tag,
				//		PlaySessionId:                  playSessionId,
				//		SegmentContainer:               segmentContainer,
				//		SegmentLength:                  segmentLength,
				//		MinSegments:                    minSegments,
				MediaSourceID:               mediaSourceId,
				DeviceID:                    deviceId,
				AudioCodec:                  audioCodec,
				EnableAutoStreamCopy:        enableAutoStreamCopy,
				AllowAudioStreamCopy:        allowAudioStreamCopy,
				AllowVideoStreamCopy:        allowVideoStreamCopy,
				BreakOnNonKeyFrames:         breakOnNonKeyFrames,
				AudioSampleRate:             &audioSampleRate,
				MaxAudioChannels:            &maxAudioChannels,
				AudioBitRate:                &audioBitRate,
				MaxAudioBitDepth:            &maxAudioBitDepth,
				AudioChannels:               audioChannels,
				Profile:                     profile,
				Level:                       level,
				Framerate:                   framerate,
				MaxFramerate:                maxFramerate,
				CopyTimestamps:              copyTimestamps,
				StartTimeTicks:              &startTimeTicks,
				Width:                       &width,
				Height:                      &height,
				MaxWidth:                    &maxWidth,
				MaxHeight:                   &maxHeight,
				VideoBitRate:                &videoBitRate,
				SubtitleStreamIndex:         &subtitleStreamIndex,
				SubtitleMethod:              subtitleMethod,
				MaxRefFrames:                &maxRefFrames,
				MaxVideoBitDepth:            &maxVideoBitDepth,
				RequireAvc:                  requireAvc,
				DeInterlace:                 deInterlace,
				RequireNonAnamorphic:        requireNonAnamorphic,
				TranscodingMaxAudioChannels: &transcodingMaxAudioChannels,
				CpuCoreLimit:                &cpuCoreLimit,
				LiveStreamId:                liveStreamId,
				EnableMpegtsM2TsMode:        enableMpegtsM2TsMode,
				VideoCodec:                  videoCodec,
				SubtitleCodec:               subtitleCodec,
				TranscodeReasons:            transcodeReasons,
				AudioStreamIndex:            &audioStreamIndex,
				VideoStreamIndex:            &videoStreamIndex,
				Context:                     encodingContext,
				StreamOptions:               streamOptions,
				EnableAudioVbrEncoding:      enableAudioVbrEncoding,
			},
			Params:           &params,
			Tag:              &tag,
			PlaySessionID:    &playSessionId,
			SegmentContainer: &segmentContainer,
			SegmentLength:    &segmentLength,
			MinSegments:      &minSegments,
			//                        CurrentRuntimeTicks:      runtimeTicks,
			//                        ActualSegmentLengthTicks: actualSegmentLengthTicks,
		},
		//			EnableTrickplay: enableTrickplay,
	}

	vc.logger.Infof("%v", request)
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, "userId", "root")
	defer cancel()

	state, err := helpers.GetStreamingState(
		request,
		//r,
		nil,
		//              _mediaSourceManager,
		//              _userManager,
		//              _libraryManager,
		vc.serverConfigurationManager,
		vc.mediaEncoder,
		vc.encodingHelper,
		vc.transcodeManager,
		transcodingJobType,
		ctx,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
		return
	}
	klog.Infof("state: %+v\n", state)
	klog.Infoln("SegmentContainer", state.Request.SegmentContainer)

	if static && state.InputProtocol != mediaprotocol.File {
		//		return http.StatusBadRequest, fmt.Sprintf("Input protocol %s cannot be streamed statically", state.InputProtocol)
		http.Error(w, fmt.Sprintf("Input protocol %s cannot be streamed statically", state.InputProtocol), http.StatusBadRequest)
		return
	}

	if static && !(*state.MediaSource.VideoType == entities.BluRay || *state.MediaSource.VideoType == entities.Dvd) {
		contentType := state.GetMimeType("."+state.OutputContainer, false)
		if contentType == "" {
			contentType = state.GetMimeType(state.MediaPath, true)
		}

		if state.MediaSource.IsInfiniteStream {
			//			liveStream := NewProgressiveFileStream(state.MediaPath)
			//			return http.StatusOK, liveStream // Replace with actual file response
			http.Error(w, "not support infinite stream", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, state.MediaPath)

		//		return http.StatusOK, GetStaticFileResult(state.MediaPath, contentType)
	}

	/*
		playlist, err := vc.dynamicHlsHelper.GetMasterHlsPlaylist(r, transcodingJobType, streamingRequest, enableAdaptiveBitrateStreaming)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "video/mp4")
		_, err = w.Write([]byte(playlist))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	*/
}
