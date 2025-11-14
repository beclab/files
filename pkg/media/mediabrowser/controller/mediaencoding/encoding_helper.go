package mediaencoding

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"files/pkg/media/jellyfin/data/enums"
	cc "files/pkg/media/mediabrowser/common/configuration"
	"files/pkg/media/mediabrowser/model/configuration"
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
	"files/pkg/media/utils/version"

	"k8s.io/klog/v2"
)

type EncodingHelper struct {
	mediaEncoder IMediaEncoder
	/*
	   _appPaths              IApplicationPaths
	   _subtitleEncoder       ISubtitleEncoder
	   _config                IConfiguration
	*/
	configurationManager cc.IConfigurationManager
}

var (
	ValidationRegex = `^[a-zA-Z0-9\-\._,|]{0,40}$`

	QsvAlias          = "qs"
	VaapiAlias        = "va"
	D3d11vaAlias      = "dx11"
	VideotoolboxAlias = "vt"
	RkmppAlias        = "rk"
	OpenclAlias       = "ocl"
	CudaAlias         = "cu"
	DrmAlias          = "dr"
	VulkanAlias       = "vk"

	minKerneli915Hang                = version.Version{5, 18, 0}
	maxKerneli915Hang                = version.Version{6, 1, 3}
	minFixedKernel60i915Hang         = version.Version{6, 0, 18}
	minKernelVersionAmdVkFmtModifier = version.Version{5, 15, 0}
	minFFmpegImplictHwaccel          = version.Version{6, 0, 0}
	minFFmpegHwaUnsafeOutput         = version.Version{6, 0, 0}
	minFFmpegOclCuTonemapMode        = version.Version{5, 1, 3}
	minFFmpegSvtAv1Params            = version.Version{5, 1, 0}
	minFFmpegVaapiH26xEncA53CcSei    = version.Version{6, 0, 0}
	minFFmpegReadrateOption          = version.Version{5, 0, 0}
	minFFmpegWorkingVtHwSurface      = version.Version{7, 0, 1}
	minFFmpegDisplayRotationOption   = version.Version{6, 0, 0}
	minFFmpegAdvancedTonemapMode     = version.Version{7, 0, 1}
	minFFmpegAlteredVaVkInterop      = version.Version{7, 0, 1}
	minFFmpegQsvVppTonemapOption     = version.Version{7, 0, 1}
	minFFmpegQsvVppOutRangeOption    = version.Version{7, 0, 1}
	minFFmpegVaapiDeviceVendorId     = version.Version{7, 0, 1}
	minFFmpegQsvVppScaleModeOption   = version.Version{6, 0, 0}
	minFFmpegRkmppHevcDecDoviRpu     = version.Version{7, 1, 1}

	validationRegex   = regexp.MustCompile(ValidationRegex)
	videoProfilesH264 = []string{
		"ConstrainedBaseline",
		"Baseline",
		"Extended",
		"Main",
		"High",
		"ProgressiveHigh",
		"ConstrainedHigh",
		"High10",
	}
	videoProfilesH265 = []string{
		"Main",
		"Main10",
	}
	videoProfilesAv1 = []string{
		"Main",
		"High",
		"Professional",
	}
	mp4ContainerNames = map[string]bool{
		"mp4": true,
		"m4a": true,
		"m4p": true,
		"m4b": true,
		"m4r": true,
		"m4v": true,
	}
	audioTranscodeChannelLookup = map[string]int{
		"wmav2":      2,
		"libmp3lame": 2,
		"libfdk_aac": 6,
		"ac3":        6,
		"eac3":       6,
		"dca":        6,
		"mlp":        6,
		"truehd":     6,
	}
	defaultMjpegEncoder = "mjpeg"
	mjpegCodecMap       = map[entities.HardwareAccelerationType]string{
		entities.HardwareAccelerationType_VAAPI: "mjpeg_vaapi",
		entities.HardwareAccelerationType_QSV:   "mjpeg_qsv",
	}
	LosslessAudioCodecs = []string{
		"alac",
		"ape",
		"flac",
		"mlp",
		"truehd",
		"wavpack",
	}
)

func NewEncodingHelper(
	mediaEncoder IMediaEncoder,
	/*
	   appPaths IApplicationPaths,
	   subtitleEncoder ISubtitleEncoder,
	   config IConfiguration,
	*/
	configurationManager cc.IConfigurationManager,
) EncodingHelper {
	return EncodingHelper{
		mediaEncoder: mediaEncoder,
		/*
		   _appPaths:              appPaths,
		   _subtitleEncoder:       subtitleEncoder,
		   _config:                config,
		*/
		configurationManager: configurationManager,
	}
}

func (e *EncodingHelper) IsVaapiSupported(state *EncodingJobInfo) bool {
	// vaapi will throw an error with this input
	// [vaapi @ 0x7faed8000960] No VAAPI support for codec mpeg4 profile -99.
	if strings.EqualFold(state.VideoStream.Codec, "mpeg4") {
		return false
	}

	return e.mediaEncoder.SupportsHwaccel("vaapi")
}

func (e *EncodingHelper) IsVaapiFullSupported() bool {
	return e.mediaEncoder.SupportsHwaccel("drm") &&
		e.mediaEncoder.SupportsHwaccel("vaapi") &&
		e.mediaEncoder.SupportsFilter("scale_vaapi") &&
		e.mediaEncoder.SupportsFilter("deinterlace_vaapi") &&
		e.mediaEncoder.SupportsFilter("tonemap_vaapi") &&
		e.mediaEncoder.SupportsFilter("procamp_vaapi") &&
		e.mediaEncoder.SupportsFilterWithOption(OverlayVaapiFrameSync) &&
		e.mediaEncoder.SupportsFilter("transpose_vaapi") &&
		e.mediaEncoder.SupportsFilter("hwupload_vaapi")
}

func (e *EncodingHelper) IsRkmppFullSupported() bool {
	return e.mediaEncoder.SupportsHwaccel("rkmpp") &&
		e.mediaEncoder.SupportsFilter("scale_rkrga") &&
		e.mediaEncoder.SupportsFilter("vpp_rkrga") &&
		e.mediaEncoder.SupportsFilter("overlay_rkrga")
}

func (e *EncodingHelper) IsOpenclFullSupported() bool {
	return e.mediaEncoder.SupportsHwaccel("opencl") &&
		e.mediaEncoder.SupportsFilter("scale_opencl") &&
		e.mediaEncoder.SupportsFilterWithOption(TonemapOpenclBt2390) &&
		e.mediaEncoder.SupportsFilterWithOption(OverlayOpenclFrameSync)
}

func (e *EncodingHelper) IsCudaFullSupported() bool {
	return e.mediaEncoder.SupportsHwaccel("cuda") &&
		e.mediaEncoder.SupportsFilterWithOption(ScaleCudaFormat) &&
		e.mediaEncoder.SupportsFilter("yadif_cuda") &&
		e.mediaEncoder.SupportsFilterWithOption(TonemapCudaName) &&
		e.mediaEncoder.SupportsFilter("overlay_cuda") &&
		e.mediaEncoder.SupportsFilter("hwupload_cuda")
}

func (e *EncodingHelper) IsVulkanFullSupported() bool {
	return e.mediaEncoder.SupportsHwaccel("vulkan") &&
		e.mediaEncoder.SupportsFilter("libplacebo") &&
		e.mediaEncoder.SupportsFilter("scale_vulkan") &&
		e.mediaEncoder.SupportsFilterWithOption(OverlayVulkanFrameSync) &&
		e.mediaEncoder.SupportsHwaccel("transpose_vulkan") &&
		e.mediaEncoder.SupportsFilter("flip_vulkan")
}

func (e *EncodingHelper) IsVideoToolboxFullSupported() bool {
	return e.mediaEncoder.SupportsHwaccel("videotoolbox") &&
		e.mediaEncoder.SupportsFilter("yadif_videotoolbox") &&
		e.mediaEncoder.SupportsFilter("overlay_videotoolbox") &&
		e.mediaEncoder.SupportsFilter("tonemap_videotoolbox") &&
		e.mediaEncoder.SupportsFilter("scale_vt")
}

func (e *EncodingHelper) InferAudioCodec(container string) string {
	ext := "." + (container)
	if ext == "" {
		ext = ".copy"
	}

	switch ext {
	case ".mp3":
		return "mp3"
	case ".aac":
		return "aac"
	case ".wma":
		return "wma"
	case ".ogg", ".oga", ".ogv", ".webm", ".webma":
		return "vorbis"
	default:
		return "copy"
	}
}

func (e *EncodingHelper) AttachMediaSourceInfo(
	state *EncodingJobInfo,
	encodingOptions *configuration.EncodingOptions,
	mediaSource *dto.MediaSourceInfo,
	requestedUrl string,
) {
	if state == nil {
		panic("state is nil")
	}
	if mediaSource == nil {
		panic("mediaSource is nil")
	}

	path := mediaSource.Path
	protocol := mediaSource.Protocol

	if mediaSource.EncoderPath != "" && mediaSource.EncoderProtocol != nil {
		path = mediaSource.EncoderPath
		protocol = *mediaSource.EncoderProtocol
	}

	state.MediaPath = path
	state.InputProtocol = protocol
	state.InputContainer = mediaSource.Container
	state.RunTimeTicks = mediaSource.RunTimeTicks
	state.RemoteHttpHeaders = mediaSource.RequiredHttpHeaders
	state.IsoType = mediaSource.IsoType

	if mediaSource.Timestamp != nil {
		state.InputTimestamp = *mediaSource.Timestamp
	}

	state.RunTimeTicks = mediaSource.RunTimeTicks
	state.RemoteHttpHeaders = mediaSource.RequiredHttpHeaders
	state.ReadInputAtNativeFramerate = mediaSource.ReadAtNativeFramerate

	if state.ReadInputAtNativeFramerate || (mediaSource.Protocol == mediaprotocol.File && strings.EqualFold(mediaSource.Container, "wtv")) {
		state.InputVideoSync = "-1"
		state.InputAudioSync = "1"
	}

	if strings.EqualFold(mediaSource.Container, "wma") || strings.EqualFold(mediaSource.Container, "asf") {
		state.InputAudioSync = "1"
	}

	mediaStreams := mediaSource.MediaStreams

	if state.IsVideoRequest {
		videoRequest := state.BaseRequest

		if videoRequest.VideoCodec == "" {
			if requestedUrl == "" {
				requestedUrl = "test." + videoRequest.Container
			}
			videoRequest.VideoCodec = e.InferVideoCodec(requestedUrl)
		}

		state.VideoStream = e.GetMediaStream(mediaStreams, videoRequest.VideoStreamIndex, entities.MediaStreamTypeVideo, true)
		state.SubtitleStream = e.GetMediaStream(mediaStreams, videoRequest.SubtitleStreamIndex, entities.MediaStreamTypeSubtitle, false)
		state.SubtitleDeliveryMethod = videoRequest.SubtitleMethod
		state.AudioStream = e.GetMediaStream(mediaStreams, videoRequest.AudioStreamIndex, entities.MediaStreamTypeAudio, true)

		if state.SubtitleStream != nil && !state.SubtitleStream.IsExternal {
			state.InternalSubtitleStreamOffset = GetInternalSubtitleStreamOffset(mediaStreams, state.SubtitleStream)
		}

		e.EnforceResolutionLimit(state)
		e.NormalizeSubtitleEmbed(state)
	} else {
		state.AudioStream = e.GetMediaStream(mediaStreams, nil, entities.MediaStreamTypeAudio, true)
	}

	state.MediaSource = mediaSource

	request := state.BaseRequest
	supportedAudioCodecs := state.SupportedAudioCodecs
	if request != nil && supportedAudioCodecs != nil && len(supportedAudioCodecs) > 0 {
		supportedAudioCodecsList := make([]string, len(state.SupportedAudioCodecs))
		copy(supportedAudioCodecsList, state.SupportedAudioCodecs)

		e.ShiftAudioCodecsIfNeeded(supportedAudioCodecsList, state.AudioStream)
		state.SupportedAudioCodecs = supportedAudioCodecsList
		//       request.AudioCodec = getFirstSupportedAudioCodec(shiftedAudioCodecs, state)
		// compile
	}

	supportedVideoCodecs := state.SupportedVideoCodecs
	if request != nil && supportedVideoCodecs != nil && len(supportedVideoCodecs) > 0 {
		supportedVideoCodecsList := make([]string, len(state.SupportedVideoCodecs))
		copy(supportedVideoCodecsList, state.SupportedVideoCodecs)

		e.ShiftVideoCodecsIfNeeded(&supportedVideoCodecsList, encodingOptions)
		state.SupportedVideoCodecs = supportedVideoCodecsList
		request.VideoCodec = state.SupportedVideoCodecs[0]
		klog.Infoln("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", request.VideoCodec)
	}
}

/*
func getFirstSupportedAudioCodec(request *EncodeRequest, supportedAudioCodecs []string, mediaEncoder MediaEncoder) {
    for _, codec := range supportedAudioCodecs {
        if mediaEncoder.CanEncodeToAudioCodec(codec) {
            request.AudioCodec = codec
            return
        }
    }

    if len(supportedAudioCodecs) > 0 {
        request.AudioCodec = supportedAudioCodecs[0]
    }
}
*/

func GetInternalSubtitleStreamOffset(mediaStreams []entities.MediaStream, subtitleStream *entities.MediaStream) int {
	var internalSubtitleStreams []*entities.MediaStream
	for i := range mediaStreams {
		if mediaStreams[i].Type == entities.MediaStreamTypeSubtitle && !mediaStreams[i].IsExternal {
			internalSubtitleStreams = append(internalSubtitleStreams, &mediaStreams[i])
		}
	}

	for i, stream := range internalSubtitleStreams {
		if stream == subtitleStream {
			return i
		}
	}

	return -1
}

func (e *EncodingHelper) GetNumAudioChannelsParam(state EncodingJobInfo, audioStream *entities.MediaStream, outputAudioCodec string) *int {
	if audioStream == nil {
		return nil
	}

	request := state.BaseRequest

	codec := outputAudioCodec
	if codec == "" {
		codec = ""
	}

	resultChannels := state.GetRequestedAudioChannels(codec)

	inputChannels := audioStream.Channels
	if *inputChannels > 0 {
		if *inputChannels < *resultChannels {
			resultChannels = inputChannels
		}
	}

	isTranscodingAudio := !IsCopyCodec(codec)
	if isTranscodingAudio {
		audioEncoder := e.GetAudioEncoder(state)
		transcoderChannelLimit, ok := audioTranscodeChannelLookup[audioEncoder]
		if !ok {
			// Set default max transcoding channels to 8 to prevent encoding errors due to asking for too many channels.
			transcoderChannelLimit = 8
		}

		// Set resultChannels to minimum between resultChannels, TranscodingMaxAudioChannels, transcoderChannelLimit
		if transcoderChannelLimit < *resultChannels {
			resultChannels = &transcoderChannelLimit
		}
		if *request.TranscodingMaxAudioChannels < *resultChannels {
			resultChannels = request.TranscodingMaxAudioChannels
		}

		// Avoid transcoding to audio channels other than 1ch, 2ch, 6ch (5.1 layout) and 8ch (7.1 layout).
		// https://developer.apple.com/documentation/http_live_streaming/hls_authoring_specification_for_apple_devices
		if state.TranscodingType != Progressive && (*resultChannels > 2 && *resultChannels < 6 || *resultChannels == 7) {
			// We can let FFMpeg supply an extra LFE channel for 5ch and 7ch to make them 5.1 and 7.1
			if *resultChannels == 5 {
				resultChannels = toPtr(6)
			} else if *resultChannels == 7 {
				resultChannels = toPtr(8)
			} else {
				// For other weird layout, just downmix to stereo for compatibility
				resultChannels = toPtr(2)
			}
		}
	}

	return resultChannels
}

func toPtr[T any](v T) *T {
	return &v
}

func (e *EncodingHelper) getAudioBitrateParam(request BaseEncodingJobOptions, audioStream *entities.MediaStream, outputAudioChannels *int) *int {
	return e.GetAudioBitrateParam(request.AudioBitRate, request.AudioCodec, audioStream, outputAudioChannels)
}

func (e *EncodingHelper) GetAudioBitrateParam(audioBitRate *int, audioCodec string, audioStream *entities.MediaStream, outputAudioChannels *int) *int {
	if audioStream == nil {
		return nil
	}

	inputChannels := 0
	if audioStream.Channels != nil {
		inputChannels = *audioStream.Channels
	}
	outputChannels := 0
	if outputAudioChannels != nil {
		outputChannels = *outputAudioChannels
	}
	bitrate := int(math.MaxInt)
	if audioBitRate != nil {
		bitrate = *audioBitRate
	}

	switch {
	case audioCodec == "" || strings.EqualFold(audioCodec, "aac") || strings.EqualFold(audioCodec, "mp3") || strings.EqualFold(audioCodec, "opus") || strings.EqualFold(audioCodec, "vorbis") || strings.EqualFold(audioCodec, "ac3") || strings.EqualFold(audioCodec, "eac3"):
		if inputChannels >= 6 && (outputChannels >= 6 || outputChannels == 0) {
			return toPtr(min(640000, bitrate))
		} else if inputChannels > 0 && outputChannels > 0 {
			return toPtr(min(outputChannels*128000, bitrate))
		} else if inputChannels > 0 {
			return toPtr(min(inputChannels*128000, bitrate))
		} else {
			return toPtr(min(384000, bitrate))
		}
	case strings.EqualFold(audioCodec, "dts") || strings.EqualFold(audioCodec, "dca"):
		if inputChannels >= 6 && (outputChannels >= 6 || outputChannels == 0) {
			return toPtr(min(768000, bitrate))
		} else if inputChannels > 0 && outputChannels > 0 {
			return toPtr(min(outputChannels*136000, bitrate))
		} else if inputChannels > 0 {
			return toPtr(min(inputChannels*136000, bitrate))
		} else {
			return toPtr(min(672000, bitrate))
		}
	default:
		// Empty bitrate area is not allow on iOS
		// Default audio bitrate to 128K per channel if we don't have codec specific defaults
		// https://ffmpeg.org/ffmpeg-codecs.html#toc-Codec-Options
		if outputAudioChannels != nil {
			return toPtr(128000 * *outputAudioChannels)
		} else if audioStream.Channels != nil {
			return toPtr(128000 * *audioStream.Channels)
		} else {
			return toPtr(256000)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func GetSegmentFileExtension(segmentContainer *string) string {
	if segmentContainer != nil {
		return "." + *segmentContainer
	}
	return ".ts"
}

func (e *EncodingHelper) GetVideoBitrateParamValue(request *BaseEncodingJobOptions, videoStream *entities.MediaStream, outputVideoCodec string) int {
	bitrate := request.VideoBitRate
	klog.Infoln("bitrate: ", *bitrate)

	if videoStream != nil {
		isUpscaling := request.Height != nil && videoStream.Height != nil && *request.Height > *videoStream.Height &&
			request.Width != nil && videoStream.Width != nil && *request.Width > *videoStream.Width

		// Don't allow bitrate increases unless upscaling
		if !isUpscaling && bitrate != nil && videoStream.BitRate != nil {
			bitrate = toPtr(e.GetMinBitrate(*videoStream.BitRate, *bitrate))
		}

		if bitrate != nil {
			inputVideoCodec := videoStream.Codec
			bitrate = toPtr(ScaleBitrate(*bitrate, inputVideoCodec, outputVideoCodec))

			// If a max bitrate was requested, don't let the scaled bitrate exceed it
			if request.VideoBitRate != nil {
				bitrate = toPtr(min(*bitrate, *request.VideoBitRate))
			}
		}
	}

	// Cap the max target bitrate to intMax/2 to satisfy the bufsize=bitrate*2.
	if bitrate == nil {
		return int(math.MaxInt / 2)
	}
	return min(*bitrate, int(math.MaxInt/2))
}

func (e *EncodingHelper) GetMinBitrate(sourceBitrate, requestedBitrate int) int {
	// these values were chosen from testing to improve low bitrate streams
	if sourceBitrate <= 2000000 {
		sourceBitrate = int(float64(sourceBitrate) * 2.5)
	} else if sourceBitrate <= 3000000 {
		sourceBitrate *= 2
	}

	bitrate := int(math.Min(float64(sourceBitrate), float64(requestedBitrate)))

	return bitrate
}

/*
func (e *EncodingHelper) ScaleBitrate(bitrate int, inputVideoCodec, outputVideoCodec string) int {
	inputScaleFactor := e.GetVideoBitrateScaleFactor(inputVideoCodec)
	outputScaleFactor := e.GetVideoBitrateScaleFactor(outputVideoCodec)

	// Don't scale the real bitrate lower than the requested bitrate
	scaleFactor := math.Max(outputScaleFactor/inputScaleFactor, 1)

	if bitrate <= 500000 {
		scaleFactor = math.Max(scaleFactor, 4)
	} else if bitrate <= 1000000 {
		scaleFactor = math.Max(scaleFactor, 3)
	} else if bitrate <= 2000000 {
		scaleFactor = math.Max(scaleFactor, 2.5)
	} else if bitrate <= 3000000 {
		scaleFactor = math.Max(scaleFactor, 2)
	}

	return int(scaleFactor * float64(bitrate))
}

func (e *EncodingHelper) GetVideoBitrateScaleFactor(codec string) float64 {
	// hevc & vp9 - 40% more efficient than h.264
	if strings.EqualFold(codec, "h265") || strings.EqualFold(codec, "hevc") || strings.EqualFold(codec, "vp9") {
		return 0.6
	}

	// av1 - 50% more efficient than h.264
	if strings.EqualFold(codec, "av1") {
		return 0.5
	}

	return 1.0
}
*/

func (e *EncodingHelper) TryStreamCopy(state EncodingJobInfo) {
	if state.VideoStream != nil && e.CanStreamCopyVideo(state, state.VideoStream) {
		state.OutputVideoCodec = "copy"
	} else {
		//        user := state.User
		// If the user doesn't have access to transcoding, then force stream copy, regardless of whether it will be compatible or not
		//        if user != nil && !user.HasPermission(PermissionKind_EnableVideoPlaybackTranscoding) {
		//            state.OutputVideoCodec = "copy"
		//        }
	}

	if state.AudioStream != nil && e.CanStreamCopyAudio(state, state.AudioStream, state.SupportedAudioCodecs) {
		state.OutputAudioCodec = "copy"
	} else {
		//        user := state.User
		// If the user doesn't have access to transcoding, then force stream copy, regardless of whether it will be compatible or not
		//        if user != nil && !user.HasPermission(PermissionKind_EnableAudioPlaybackTranscoding) {
		//           state.OutputAudioCodec = "copy"
		//        }
	}
}

func (e *EncodingHelper) CanStreamCopyVideo(state EncodingJobInfo, videoStream *entities.MediaStream) bool {
	request := state.BaseRequest

	if !request.AllowVideoStreamCopy {
		return false
	}

	if videoStream.IsInterlaced && state.DeInterlace(videoStream.Codec, false) {
		return false
	}

	if videoStream.IsAnamorphic != nil && *videoStream.IsAnamorphic && request.RequireNonAnamorphic {
		return false
	}

	if request.SubtitleStreamIndex != nil && *request.SubtitleStreamIndex >= 0 && state.SubtitleDeliveryMethod == dlna.Encode {
		return false
	}

	if strings.EqualFold(videoStream.Codec, "h264") && videoStream.IsAVC != nil && !*videoStream.IsAVC && request.RequireAvc {
		return false
	}

	if videoStream.Codec == "" || (len(state.SupportedVideoCodecs) > 0 && !contains(state.SupportedVideoCodecs, videoStream.Codec, true)) {
		return false
	}

	requestedProfiles := state.GetRequestedProfiles(videoStream.Codec)
	if len(requestedProfiles) > 0 {
		if videoStream.Profile == "" {
			// return false
		}

		requestedProfile := requestedProfiles[0]
		if videoStream.Profile != "" && !contains(requestedProfiles, strings.ReplaceAll(videoStream.Profile, " ", ""), true) {
			currentScore := e.GetVideoProfileScore(videoStream.Codec, videoStream.Profile)
			requestedScore := e.GetVideoProfileScore(videoStream.Codec, requestedProfile)
			if currentScore == -1 || currentScore > requestedScore {
				return false
			}
		}
	}

	requestedRangeTypes := state.GetRequestedRangeTypes(videoStream.Codec)
	if len(requestedRangeTypes) > 0 {
		if videoStream.VideoRangeType == enums.VideoRangeTypeUnknown {
			return false
		}

		/* comple
		   requestHasHDR10 := contains(requestedRangeTypes, enums.VideoRangeType_HDR10.String(), true)
		   requestHasHLG := contains(requestedRangeTypes, enums.VideoRangeType_HLG.String(), true)
		   requestHasSDR := contains(requestedRangeTypes, enums.VideoRangeType_SDR.String(), true)

		   if !contains(requestedRangeTypes, videoStream.VideoRangeType.String(), true) &&
		       !((requestHasHDR10 && videoStream.VideoRangeType == enums.VideoRangeType_DOVIWithHDR10) ||
		           (requestHasHLG && videoStream.VideoRangeType == enums.VideoRangeType_DOVIWithHLG) ||
		           (requestHasSDR && videoStream.VideoRangeType == enums.VideoRangeType_DOVIWithSDR)) {
		       return false
		   }
		*/
	}

	/*
	   if request.MaxWidth != nil && (!videoStream.Width.HasValue || *videoStream.Width > *request.MaxWidth) {
	       return false
	   }

	   if request.MaxHeight != nil && (!videoStream.Height.HasValue || *videoStream.Height > *request.MaxHeight) {
	       return false
	   }

	   requestedFramerate := request.MaxFramerate
	   if requestedFramerate == nil {
	       requestedFramerate = request.Framerate
	   }
	   if requestedFramerate != nil {
	       videoFrameRate := videoStream.AverageFrameRate
	       if videoFrameRate == nil {
	           videoFrameRate = videoStream.RealFrameRate
	       }
	       if videoFrameRate == nil || *videoFrameRate > *requestedFramerate {
	           return false
	       }
	   }
	*/

	if request.VideoBitRate != nil && (videoStream.BitRate != nil || *videoStream.BitRate > *request.VideoBitRate) {
		if request.LiveStreamId == "" || videoStream.BitRate != nil {
			return false
		}
	}

	maxBitDepth := state.GetRequestedVideoBitDepth(videoStream.Codec)
	if maxBitDepth != nil && videoStream.BitDepth != nil && *videoStream.BitDepth > *maxBitDepth {
		return false
	}

	maxRefFrames := state.GetRequestedMaxRefFrames(videoStream.Codec)
	if maxRefFrames != nil && videoStream.RefFrames != nil && *videoStream.RefFrames > *maxRefFrames {
		return false
	}

	level := state.GetRequestedLevel(videoStream.Codec)
	if level != "" {
		requestLevel, err := strconv.ParseFloat(level, 64)
		if err == nil {
			if videoStream.Level != nil {
				// return false
			}
			if videoStream.Level != nil && *videoStream.Level > requestLevel {
				return false
			}
		}
	}

	if strings.EqualFold(state.InputContainer, "avi") && strings.EqualFold(videoStream.Codec, "h264") && (videoStream.IsAVC == nil || !*videoStream.IsAVC) {
		return false
	}

	return true
}

func (e *EncodingHelper) CanStreamCopyAudio(state EncodingJobInfo, audioStream *entities.MediaStream, supportedAudioCodecs []string) bool {
	request := state.BaseRequest

	if !request.AllowAudioStreamCopy {
		return false
	}

	maxBitDepth := state.GetRequestedAudioBitDepth(audioStream.Codec)
	if maxBitDepth != nil && audioStream.BitDepth != nil && *audioStream.BitDepth > *maxBitDepth {
		return false
	}

	if audioStream.Codec == "" || !contains(supportedAudioCodecs, audioStream.Codec, true) {
		return false
	}

	channels := state.GetRequestedAudioChannels(audioStream.Codec)
	if channels != nil {
		if audioStream.Channels != nil || *audioStream.Channels <= 0 {
			return false
		}
		if *audioStream.Channels > *channels {
			return false
		}
	}

	if request.AudioSampleRate != nil {
		if audioStream.SampleRate != nil || *audioStream.SampleRate <= 0 {
			return false
		}
		if *audioStream.SampleRate > *request.AudioSampleRate {
			return false
		}
	}

	if request.AudioBitRate != nil && audioStream.BitRate != nil && *audioStream.BitRate > *request.AudioBitRate {
		return false
	}

	return request.EnableAutoStreamCopy
}

/*
func contains(slice []string, value string, ignoreCase bool) bool {
	for _, item := range slice {
		if (ignoreCase && strings.EqualFold(item, value)) || (!ignoreCase && item == value) {
			return true
		}
	}
	return false
}
*/

// Contains searches for a value in a slice of comparable types (e.g., string or int).
// For strings, ignoreCase determines if the comparison is case-insensitive.
// For non-string types, ignoreCase is ignored.
func contains[T comparable](slice []T, value T, ignoreCase bool) bool {
	for _, item := range slice {
		if any(item) == any(value) {
			return true
		}
		// Handle string-specific case-insensitive comparison
		if ignoreCase {
			if strItem, ok := any(item).(string); ok {
				if strValue, ok := any(value).(string); ok {
					if strings.EqualFold(strItem, strValue) {
						return true
					}
				}
			}
		}
	}
	return false
}

type PermissionKind int

const (
	PermissionKind_EnableVideoPlaybackTranscoding PermissionKind = iota
	PermissionKind_EnableAudioPlaybackTranscoding
)

type User struct {
	Permissions []PermissionKind
}

func (u *User) HasPermission(permission PermissionKind) bool {
	for _, p := range u.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

func (e *EncodingHelper) InferVideoCodec(url string) string {
	ext := filepath.Ext(url)

	switch {
	case strings.EqualFold(ext, ".asf"):
		return "wmv"
	case strings.EqualFold(ext, ".webm"):
		// TODO: this may not always mean VP8, as the codec ages
		return "vp8"
	case strings.EqualFold(ext, ".ogg") || strings.EqualFold(ext, ".ogv"):
		return "theora"
	case strings.EqualFold(ext, ".m3u8") || strings.EqualFold(ext, ".ts"):
		return "h264"
	default:
		return "copy"
	}
}

func (e *EncodingHelper) GetMediaStream(allStream []entities.MediaStream, desiredIndex *int, streamType entities.MediaStreamType, returnFirstIfNoIndex bool) *entities.MediaStream {
	if desiredIndex != nil {
		klog.Infoln("zzzzzzzzzz", *desiredIndex)
	}
	streams := make([]*entities.MediaStream, 0, len(allStream))
	for i := range allStream {
		if allStream[i].Type == streamType {
			streams = append(streams, &allStream[i])
		}
	}
	sort.Slice(streams, func(i, j int) bool {
		return streams[i].Index < streams[j].Index
	})

	klog.Infoln(len(streams))
	if desiredIndex != nil {
		for i := range streams {
			if streams[i].Index == *desiredIndex {
				return streams[i]
			}
		}
	}

	if returnFirstIfNoIndex && streamType == entities.MediaStreamTypeAudio {
		for i := range streams {
			if streams[i].Channels != nil && *streams[i].Channels > 0 {
				return streams[i]
			}
		}
		if len(streams) > 0 {
			return streams[0]
		}
	}

	if returnFirstIfNoIndex && len(streams) > 0 {
		return streams[0]
	}

	return nil
}

func (e *EncodingHelper) ShiftVideoCodecsIfNeeded(videoCodecs *[]string, encodingOptions *configuration.EncodingOptions) {
	// No need to shift if there is only one supported video codec.
	if videoCodecs == nil || len(*videoCodecs) < 2 {
		return
	}

	// Shift codecs to the end of list if it's not allowed.
	shiftVideoCodecs := []string{}
	if !encodingOptions.AllowHevcEncoding {
		shiftVideoCodecs = append(shiftVideoCodecs, "hevc", "h265")
	}

	if !encodingOptions.AllowAv1Encoding {
		shiftVideoCodecs = append(shiftVideoCodecs, "av1")
	}
	klog.Infoln("+++++++++++++", shiftVideoCodecs, *videoCodecs)

	if len(shiftVideoCodecs) == 0 {
		return
	}

	if containsAll(shiftVideoCodecs, *videoCodecs) {
		return
	}

	l := len(*videoCodecs)
	for i := 0; i < l; i++ {
		if contains(shiftVideoCodecs, (*videoCodecs)[0], true) {
			removed := (*videoCodecs)[0]
			*videoCodecs = append((*videoCodecs)[1:], removed)
		}
	}
	klog.Infoln("shift result", *videoCodecs)
}

func (e *EncodingHelper) NormalizeSubtitleEmbed(state *EncodingJobInfo) {
	if state.SubtitleStream == nil || state.SubtitleDeliveryMethod != dlna.Embed {
		return
	}

	// This is tricky to remux in, after converting to dvdsub it's not positioned correctly
	// Therefore, let's just burn it in
	if strings.EqualFold(state.SubtitleStream.Codec, "DVBSUB") {
		state.SubtitleDeliveryMethod = dlna.Encode
	}
}

func containsAll(source, target []string) bool {
	for _, t := range target {
		if !contains(source, t, true) {
			return false
		}
	}
	return true
}

func containsAny(slice []string, elements []string) bool {
	for _, e := range elements {
		if contains(slice, e, true) {
			return true
		}
	}
	return false
}

/*
func contains(slice []string, element string) bool {
    for _, e := range slice {
        if strings.EqualFold(e, element) {
            return true
        }
    }
    return false
}
*/

func (e *EncodingHelper) EnforceResolutionLimit(state *EncodingJobInfo) {
	videoRequest := state.BaseRequest

	// Switch the incoming params to be ceilings rather than fixed values
	if videoRequest.MaxWidth == nil {
		videoRequest.MaxWidth = videoRequest.Width
	}
	if videoRequest.MaxHeight == nil {
		videoRequest.MaxHeight = videoRequest.Height
	}

	videoRequest.Width = nil
	videoRequest.Height = nil
}

func (e *EncodingHelper) ShiftAudioCodecsIfNeeded(audioCodecs []string, audioStream *entities.MediaStream) {
	// No need to shift if there is only one supported audio codec.
	if len(audioCodecs) < 2 {
		return
	}

	inputChannels := 6
	if audioStream != nil && audioStream.Channels != nil {
		inputChannels = *audioStream.Channels
	}

	shiftAudioCodecs := []string{}
	if inputChannels >= 6 {
		// DTS and TrueHD are not supported by HLS
		// Keep them in the supported codecs list, but shift them to the end of the list so that if transcoding happens, another codec is used
		shiftAudioCodecs = append(shiftAudioCodecs, "dca", "truehd")
	} else {
		// Transcoding to 2ch ac3 or eac3 almost always causes a playback failure
		// Keep them in the supported codecs list, but shift them to the end of the list so that if transcoding happens, another codec is used
		shiftAudioCodecs = append(shiftAudioCodecs, "ac3", "eac3")
	}

	if containsAll(audioCodecs, shiftAudioCodecs) {
		return
	}

	for containsAny(audioCodecs, shiftAudioCodecs) {
		audioCodecs = append(audioCodecs[1:], audioCodecs[0])
	}
}

func IsCopyCodec(codec string) bool {
	return strings.EqualFold(codec, "copy")
}

func IsDovi(stream *entities.MediaStream) bool {
	if stream == nil {
		return false
	}
	rangeType := stream.VideoRangeType

	return IsDoviWithHdr10Bl(stream) ||
		rangeType == enums.VideoRangeTypeDOVI ||
		rangeType == enums.VideoRangeTypeDOVIWithHLG ||
		rangeType == enums.VideoRangeTypeDOVIWithSDR
}

func IsDoviWithHdr10Bl(stream *entities.MediaStream) bool {
	if stream == nil {
		return false
	}
	rangeType := stream.VideoRangeType

	return rangeType == enums.VideoRangeTypeDOVIWithHDR10 ||
		rangeType == enums.VideoRangeTypeDOVIWithEL ||
		rangeType == enums.VideoRangeTypeDOVIWithHDR10Plus ||
		rangeType == enums.VideoRangeTypeDOVIWithELHDR10Plus ||
		rangeType == enums.VideoRangeTypeDOVIInvalid
}

func (e *EncodingHelper) IsDoviRemoved(state *EncodingJobInfo) bool {
	return state != nil && state.VideoStream != nil &&
		ShouldRemoveDynamicHdrMetadata(state) == DynamicHdrMetadataRemovalPlanRemoveDovi &&
		e.CanEncoderRemoveDynamicHdrMetadata(DynamicHdrMetadataRemovalPlanRemoveDovi, state.VideoStream)
}

func (e *EncodingHelper) GetAudioEncoder(state EncodingJobInfo) string {
	codec := state.OutputAudioCodec

	if !validationRegex.MatchString(codec) {
		codec = "aac"
	}

	if strings.EqualFold(codec, "aac") {
		// Use Apple's aac encoder if available as it provides best audio quality
		if e.mediaEncoder.SupportsEncoder("aac_at") {
			return "aac_at"
		}

		// Use libfdk_aac for better audio quality if using custom build of FFmpeg which has fdk_aac support
		if e.mediaEncoder.SupportsEncoder("libfdk_aac") {
			return "libfdk_aac"
		}

		return "aac"
	}

	if strings.EqualFold(codec, "mp3") {
		return "libmp3lame"
	}

	if strings.EqualFold(codec, "vorbis") {
		return "libvorbis"
	}

	if strings.EqualFold(codec, "wma") {
		return "wmav2"
	}

	if strings.EqualFold(codec, "opus") {
		return "libopus"
	}

	if strings.EqualFold(codec, "flac") {
		return "flac"
	}

	if strings.EqualFold(codec, "dts") {
		return "dca"
	}

	if strings.EqualFold(codec, "alac") {
		// The ffmpeg upstream breaks the AudioToolbox ALAC encoder in version 6.1 but fixes it in version 7.0.
		// Since ALAC is lossless in quality and the AudioToolbox encoder is not faster,
		// its only benefit is a smaller file size.
		// To prevent problems, use the ffmpeg native encoder instead.
		return "alac"
	}

	return strings.ToLower(codec)
}

func (e *EncodingHelper) GetVideoProfileScore(videoCodec, videoProfile string) int {
	// strip spaces because they may be stripped out on the query string
	profile := strings.ReplaceAll(videoProfile, " ", "")
	if strings.EqualFold(videoCodec, "h264") {
		for i, p := range videoProfilesH264 {
			if strings.EqualFold(p, profile) {
				return i
			}
		}
	} else if strings.EqualFold(videoCodec, "hevc") {
		for i, p := range videoProfilesH265 {
			if strings.EqualFold(p, profile) {
				return i
			}
		}
	} else if strings.EqualFold(videoCodec, "av1") {
		for i, p := range videoProfilesAv1 {
			if strings.EqualFold(p, profile) {
				return i
			}
		}
	}
	return -1
}

func _GetNumberOfThreads(state *EncodingJobInfo, encodingOptions *configuration.EncodingOptions, outputVideoCodec string) int {
	// VP8 and VP9 encoders must have their thread counts set.
	mustSetThreadCount := strings.EqualFold(outputVideoCodec, "libvpx") ||
		strings.EqualFold(outputVideoCodec, "libvpx-vp9")

	var threads int
	if state != nil && state.BaseRequest != nil {
		threads = *state.BaseRequest.CpuCoreLimit
	} else {
		threads = encodingOptions.EncodingThreadCount
	}

	if threads <= 0 {
		// Automatically set thread count
		if mustSetThreadCount {
			return mmax(runtime.NumCPU()-1, 1)
		} else {
			return 0
		}
	}

	if threads >= runtime.NumCPU() {
		return runtime.NumCPU()
	}

	return threads
}

func GetNumberOfThreads(state *EncodingJobInfo, encodingOptions configuration.EncodingOptions, outputVideoCodec *string) int {
	threads := encodingOptions.EncodingThreadCount
	if state != nil && state.BaseRequest != nil && state.BaseRequest.CpuCoreLimit != nil && *state.BaseRequest.CpuCoreLimit > 0 {
		threads = *state.BaseRequest.CpuCoreLimit
	}

	if threads <= 0 {
		// Automatically set thread count
		return 0
	}

	return int(math.Min(float64(threads), float64(runtime.NumCPU())))
}

func mmax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (e *EncodingHelper) GetVideoEncoder(state *EncodingJobInfo, encodingOptions *configuration.EncodingOptions) string {
	codec := state.OutputVideoCodec

	switch {
	case strings.EqualFold(codec, "av1"):
		return e.GetAv1Encoder(state, encodingOptions)
	case strings.EqualFold(codec, "h265") || strings.EqualFold(codec, "hevc"):
		return e.GetH265Encoder(state, encodingOptions)
	case strings.EqualFold(codec, "h264"):
		return e.GetH264Encoder(state, encodingOptions)
	case strings.EqualFold(codec, "mjpeg"):
		return e.GetMjpegEncoder(state, encodingOptions)
		/*
			case strings.EqualFold(codec, "vp8") || strings.EqualFold(codec, "vpx"):
				return "libvpx"
			case strings.EqualFold(codec, "vp9"):
				return "libvpx-vp9"
			case strings.EqualFold(codec, "wmv"):
				return "wmv2"
			case strings.EqualFold(codec, "theora"):
				return "libtheora"
		*/
	case validationRegex.MatchString(codec):
		return strings.ToLower(codec)
	default:
		return "copy"
	}
}

func GetMapArgs(state *EncodingJobInfo) string {
	// If we don't have known media info
	// If input is video, use -sn to drop subtitles
	// Otherwise just return empty
	if state.VideoStream == nil && state.AudioStream == nil {
		if state.IsInputVideo {
			return "-sn"
		}
		return ""
	}

	// We have media info, but we don't know the stream index
	if state.VideoStream != nil && state.VideoStream.Index == -1 {
		return "-sn"
	}

	// We have media info, but we don't know the stream index
	if state.AudioStream != nil && state.AudioStream.Index == -1 {
		if state.IsInputVideo {
			return "-sn"
		}
		return ""
	}

	var args string

	if state.VideoStream != nil {
		klog.Infof("%+v\n", state.VideoStream)
		videoStreamIndex := FindIndex(state.MediaSource.MediaStreams, state.VideoStream)
		args += fmt.Sprintf("-map 0:%d", videoStreamIndex)
	} else {
		// No known video stream
		args += "-vn"
	}

	if state.AudioStream != nil {
		klog.Infof("%+v\n", state.AudioStream)
		audioStreamIndex := FindIndex(state.MediaSource.MediaStreams, state.AudioStream)
		if state.AudioStream.IsExternal {
			hasExternalGraphicsSubs := state.SubtitleStream != nil &&
				state.SubtitleDeliveryMethod == dlna.Encode &&
				state.SubtitleStream.IsExternal &&
				!state.SubtitleStream.IsTextSubtitleStream()
			externalAudioMapIndex := 1
			if hasExternalGraphicsSubs {
				externalAudioMapIndex = 2
			}
			args += fmt.Sprintf(" -map %d:%d", externalAudioMapIndex, audioStreamIndex)
		} else {
			args += fmt.Sprintf(" -map 0:%d", audioStreamIndex)
		}
	} else {
		args += " -map -0:a"
	}

	klog.Infoln(state.SubtitleStream, state.SubtitleDeliveryMethod, "methodddddddddddddddddddddddd")
	subtitleMethod := state.SubtitleDeliveryMethod
	if state.SubtitleStream == nil || subtitleMethod == dlna.Hls {
		args += " -map -0:s"
	} else if subtitleMethod == dlna.Embed {
		subtitleStreamIndex := FindIndex(state.MediaSource.MediaStreams, state.SubtitleStream)
		args += fmt.Sprintf(" -map 0:%d", subtitleStreamIndex)
	} else if state.SubtitleStream.IsExternal && !state.SubtitleStream.IsTextSubtitleStream() {
		externalSubtitleStreamIndex := FindIndex(state.MediaSource.MediaStreams, state.SubtitleStream)
		args += fmt.Sprintf(" -map 1:%d -sn", externalSubtitleStreamIndex)
	}

	klog.Infoln("GetMapArgs", args)
	//	args = "-map 0:0 -map 0:1 -map -0:s"
	klog.Infoln(args)
	return args
}

func (e *EncodingHelper) GetInputModifier(state *EncodingJobInfo, encodingOptions *configuration.EncodingOptions, segmentContainer *string) string {
	var inputModifier string
	var analyzeDurationArgument string

	// Apply -analyzeduration as per the environment variable,
	// otherwise ffmpeg will break on certain files due to default value is 0.
	//	ffmpegAnalyzeDuration := _config.GetFFmpegAnalyzeDuration()
	ffmpegAnalyzeDuration := "200M" //"FFmpeg:analyzeduration"
	if state.MediaSource.AnalyzeDurationMs != nil && *state.MediaSource.AnalyzeDurationMs > -1 {
		analyzeDurationArgument = fmt.Sprintf("-analyzeduration %d", *state.MediaSource.AnalyzeDurationMs*1000)
	} else if ffmpegAnalyzeDuration != "" {
		analyzeDurationArgument = fmt.Sprintf("-analyzeduration %s", ffmpegAnalyzeDuration)
	}

	if analyzeDurationArgument != "" {
		inputModifier += " " + analyzeDurationArgument
	}

	inputModifier = strings.TrimSpace(inputModifier)

	// Apply -probesize if configured
	//	ffmpegProbeSize := config.GetFFmpegProbeSize()
	ffmpegProbeSize := "1G" //"FFmpeg:probesize"

	if ffmpegProbeSize != "" {
		inputModifier += fmt.Sprintf(" -probesize %s", ffmpegProbeSize)
	}

	userAgentParam := e.GetUserAgentParam(state)
	if userAgentParam != "" {
		inputModifier += " " + userAgentParam
	}

	refererParam := e.GetRefererParam(state)
	if refererParam != "" {
		inputModifier += " " + refererParam
	}

	inputModifier += " " + e.GetFastSeekCommandLineParameter(state, encodingOptions, segmentContainer)

	if state.InputProtocol == mediaprotocol.Http {
		if state.Headers != "" {
			inputModifier += ` -headers "` + state.Headers + `"`
		}
	}

	if state.InputProtocol == mediaprotocol.Rtsp {
		inputModifier += " -rtsp_transport tcp+udp -rtsp_flags prefer_tcp"
	}

	if state.InputAudioSync != "" {
		inputModifier += fmt.Sprintf(" -async %s", state.InputAudioSync)
	}

	if state.InputVideoSync != "" {
		inputModifier += fmt.Sprintf(" -vsync %s", state.InputVideoSync)
	}

	if state.ReadInputAtNativeFramerate && state.InputProtocol != mediaprotocol.Rtsp {
		inputModifier += " -re"
	} else if encodingOptions.EnableSegmentDeletion && state.VideoStream != nil && state.TranscodingType == Hls && IsCopyCodec(state.OutputVideoCodec) /* compile && mediaEncoder.EncoderVersion >= minFFmpegReadrateOption */ {
		// Set an input read rate limit 10x for using SegmentDeletion with stream-copy
		// to prevent ffmpeg from exiting prematurely (due to fast drive)
		inputModifier += " -readrate 10"
	}

	var flags []string
	if state.IgnoreInputDts {
		flags = append(flags, "+igndts")
	}
	if state.IgnoreInputIndex {
		flags = append(flags, "+ignidx")
	}
	if state.GenPtsInput || IsCopyCodec(state.OutputVideoCodec) {
		flags = append(flags, "+genpts")
	}
	if state.DiscardCorruptFramesInput {
		flags = append(flags, "+discardcorrupt")
	}
	if state.EnableFastSeekInput {
		flags = append(flags, "+fastseek")
	}

	if len(flags) > 0 {
		inputModifier += fmt.Sprintf(" -fflags %s", strings.Join(flags, ""))
	}

	klog.Infoln("^^^^^^^^^^^^^^^^^^^^^ is video request", state.IsVideoRequest)
	if state.IsVideoRequest {
		if state.InputContainer != "" && state.VideoType == entities.VideoFile && encodingOptions.HardwareAccelerationType == entities.HardwareAccelerationType_None {
			inputFormat := GetInputFormat(state.InputContainer)
			if inputFormat != "" {
				klog.Infoln("^^^^^^^^^^^^^^^^^^4^^^^^^^^^^^^^^^^^^^^^^^^", inputFormat)
				inputModifier += fmt.Sprintf(" -f %s", inputFormat)
			}
		}
	}

	if state.MediaSource.RequiresLooping {
		inputModifier += " -stream_loop -1 -reconnect_at_eof 1 -reconnect_streamed 1 -reconnect_delay_max 2"
	}

	return strings.TrimSpace(inputModifier)
}

func (e *EncodingHelper) GetHlsVideoKeyFrameArguments(state *EncodingJobInfo, codec string, segmentLength int, isEventPlaylist bool, startNumber *int) string {
	var args string
	var gopArg string

	keyFrameArg := fmt.Sprintf(" -force_key_frames:0 \"expr:gte(t,n_forced*%f)\"", float64(segmentLength))

	if state.VideoStream != nil && state.VideoStream.RealFrameRate != nil {
		// This is to make sure keyframe interval is limited to our segment,
		// as forcing keyframes is not enough.
		// Example: we encoded half of desired length, then codec detected
		// scene cut and inserted a keyframe; next forced keyframe would
		// be created outside of segment, which breaks seeking.
		gopArg = fmt.Sprintf(" -g:v:0 %d -keyint_min:v:0 %d", int(math.Ceil(float64(float32(segmentLength)**state.VideoStream.RealFrameRate))), int(math.Ceil(float64(float32(segmentLength)**state.VideoStream.RealFrameRate))))
	}

	// Unable to force key frames using these encoders, set key frames by GOP.
	if strings.EqualFold(codec, "h264_qsv") || strings.EqualFold(codec, "h264_nvenc") || strings.EqualFold(codec, "h264_amf") ||
		strings.EqualFold(codec, "h264_rkmpp") || strings.EqualFold(codec, "hevc_qsv") || strings.EqualFold(codec, "hevc_nvenc") ||
		strings.EqualFold(codec, "hevc_rkmpp") || strings.EqualFold(codec, "av1_qsv") || strings.EqualFold(codec, "av1_nvenc") ||
		strings.EqualFold(codec, "av1_amf") || strings.EqualFold(codec, "libsvtav1") {
		args += gopArg
	} else if strings.EqualFold(codec, "libx264") || strings.EqualFold(codec, "libx265") || strings.EqualFold(codec, "h264_vaapi") ||
		strings.EqualFold(codec, "hevc_vaapi") || strings.EqualFold(codec, "av1_vaapi") {
		args += keyFrameArg

		// prevent the libx264 from post processing to break the set keyframe.
		if strings.EqualFold(codec, "libx264") {
			args += " -sc_threshold:v:0 0"
		}
	} else {
		args += keyFrameArg + gopArg
	}

	// global_header produced by AMD HEVC VA-API encoder causes non-playable fMP4 on iOS
	if strings.EqualFold(codec, "hevc_vaapi") && e.mediaEncoder.IsVaapiDeviceAmd() {
		args += " -flags:v -global_header"
	}

	return args
}

func RemoveEmptyFilters(filters []string) []string {
	var result []string
	for _, filter := range filters {
		if filter != "" {
			result = append(result, filter)
		}
	}
	return result
}

func (e *EncodingHelper) GetVideoProcessingFilterParam(state *EncodingJobInfo, options *configuration.EncodingOptions, outputVideoCodec string) string {
	if state.VideoStream == nil {
		return ""
	}

	hasSubs := state.SubtitleStream != nil && state.SubtitleDeliveryMethod == dlna.Encode
	hasTextSubs := hasSubs && state.SubtitleStream.IsTextSubtitleStream()
	hasGraphicalSubs := hasSubs && !state.SubtitleStream.IsTextSubtitleStream()
	klog.Infoln(hasGraphicalSubs)

	var mainFilters, subFilters, overlayFilters []string

	switch options.HardwareAccelerationType {
	case entities.HardwareAccelerationType_VAAPI:
		mainFilters, subFilters, overlayFilters = e.GetVaapiVidFilterChain(state, options, outputVideoCodec)
	case entities.HardwareAccelerationType_QSV:
		mainFilters, subFilters, overlayFilters = e.GetIntelVidFilterChain(state, options, outputVideoCodec)
	case entities.HardwareAccelerationType_NVENC:
		mainFilters, subFilters, overlayFilters = e.GetNvidiaVidFilterChain(state, options, outputVideoCodec)
	case entities.HardwareAccelerationType_AMF:
		mainFilters, subFilters, overlayFilters = e.GetAmdVidFilterChain(state, options, outputVideoCodec)
	case entities.HardwareAccelerationType_VideoToolbox:
		mainFilters, subFilters, overlayFilters = e.GetAppleVidFilterChain(state, options, outputVideoCodec)
	case entities.HardwareAccelerationType_RKMPP:
		mainFilters, subFilters, overlayFilters = e.GetRkmppVidFilterChain(state, options, outputVideoCodec)
	default:
		mainFilters, subFilters, overlayFilters = e.GetSwVidFilterChain(state, options, outputVideoCodec)
	}

	mainFilters = RemoveEmptyFilters(mainFilters)
	subFilters = RemoveEmptyFilters(subFilters)
	overlayFilters = RemoveEmptyFilters(overlayFilters)

	framerate := e.GetFramerateParam(state)
	if framerate != nil {
		mainFilters = append([]string{fmt.Sprintf("......fps=%f", *framerate)}, mainFilters...)
	}

	var mainStr string
	if len(mainFilters) > 0 {
		mainStr = fmt.Sprintf("%s", strings.Join(mainFilters, ","))
	}

	if len(overlayFilters) == 0 {
		// -vf "scale..."
		if mainStr == "" {
			return ""
		}
		//return fmt.Sprintf(" -vf \"%s\"", mainStr)
		return fmt.Sprintf(" -vf %s", mainStr)
	}

	if len(overlayFilters) > 0 && len(subFilters) > 0 && state.SubtitleStream != nil {
		// overlay graphical/text subtitles
		subStr := fmt.Sprintf("%s", strings.Join(subFilters, ","))
		overlayStr := fmt.Sprintf("%s", strings.Join(overlayFilters, ","))

		mapPrefix := boolToInt(state.SubtitleStream.IsExternal)
		subtitleStreamIndex := FindIndex(state.MediaSource.MediaStreams, state.SubtitleStream)
		videoStreamIndex := FindIndex(state.MediaSource.MediaStreams, state.VideoStream)

		if false && hasSubs {
			filterStr := ""
			// -filter_complex "[0:s]scale=s[sub]..."
			if mainStr == "" {
				filterStr = fmt.Sprintf(" -filter_complex \"[%s:%d]%s[sub];[0:%d][sub]%s\"", mapPrefix, subtitleStreamIndex, subStr, videoStreamIndex, overlayStr)
			} else {
				filterStr = fmt.Sprintf(" -filter_complex \"[%s:%d]%s[sub];[0:%d]%s[main];[main][sub]%s\"", mapPrefix, subtitleStreamIndex, subStr, videoStreamIndex, mainStr, overlayStr)
			}

			if hasTextSubs {
				if mainStr == "" {
					filterStr = fmt.Sprintf(" -filter_complex \"%s[sub];[0:%d][sub]%s\"", subStr, videoStreamIndex, overlayStr)
				} else {
					filterStr = fmt.Sprintf(" -filter_complex \"%s[sub];[0:%d]%s[main];[main][sub]%s\"", subStr, videoStreamIndex, mainStr, overlayStr)
				}
			}
			return filterStr
		}
	}

	return ""
}

func (e *EncodingHelper) GetNegativeMapArgsByFilters(state *EncodingJobInfo, videoProcessFilters string) string {
	var args string

	if state.VideoStream != nil && strings.Contains(videoProcessFilters, "-filter_complex") {
		videoStreamIndex := FindIndex(state.MediaSource.MediaStreams, state.VideoStream)
		args = fmt.Sprintf("-map -0:%d ", videoStreamIndex)
	}

	return args
}

func (e *EncodingHelper) GetOutputFFlags(state *EncodingJobInfo) string {
	var flags []string
	if state.GenPtsOutput {
		flags = append(flags, "+genpts")
	}

	if len(flags) > 0 {
		return " -fflags " + strings.Join(flags, "")
	}

	return ""
}

func (e *EncodingHelper) GetH264Encoder(state *EncodingJobInfo, options *configuration.EncodingOptions) string {
	return e.GetH26xOrAv1Encoder("libx264", "h264", state, options)
}

func (e *EncodingHelper) GetH265Encoder(state *EncodingJobInfo, options *configuration.EncodingOptions) string {
	return e.GetH26xOrAv1Encoder("libx265", "hevc", state, options)
}

func (e *EncodingHelper) GetAv1Encoder(state *EncodingJobInfo, options *configuration.EncodingOptions) string {
	return e.GetH26xOrAv1Encoder("libsvtav1", "av1", state, options)
}

func (e *EncodingHelper) GetH26xOrAv1Encoder(defaultEncoder, hwEncoder string, state *EncodingJobInfo, options *configuration.EncodingOptions) string {
	if state.VideoType == entities.VideoFile {
		hwType := options.HardwareAccelerationType
		codecMap := map[entities.HardwareAccelerationType]string{
			entities.HardwareAccelerationType_AMF:          hwEncoder + "_amf",
			entities.HardwareAccelerationType_NVENC:        hwEncoder + "_nvenc",
			entities.HardwareAccelerationType_QSV:          hwEncoder + "_qsv",
			entities.HardwareAccelerationType_VAAPI:        hwEncoder + "_vaapi",
			entities.HardwareAccelerationType_VideoToolbox: hwEncoder + "_videotoolbox",
			entities.HardwareAccelerationType_V4L2M2M:      hwEncoder + "_v4l2m2m",
			entities.HardwareAccelerationType_RKMPP:        hwEncoder + "_rkmpp",
		}

		if hwType != entities.HardwareAccelerationType_None && options.EnableHardwareEncoding {
			if preferredEncoder, ok := codecMap[hwType]; ok && e.mediaEncoder.SupportsEncoder(preferredEncoder) {
				return preferredEncoder
			}
		}
	}

	return defaultEncoder
}

func FindIndex(mediaStreams []entities.MediaStream, streamToFind *entities.MediaStream) int {
	index := 0
	length := len(mediaStreams)
	klog.Infoln(length)

	for i := 0; i < length; i++ {
		currentMediaStream := &mediaStreams[i]

		if currentMediaStream == streamToFind {
			return index
		}

		klog.Infoln("||||||||||||||||||||||||||", currentMediaStream.Path, streamToFind.Path)
		if currentMediaStream.Path == streamToFind.Path {
			klog.Infoln(streamToFind.Path)
			index++
		}
	}

	return -1
}

func (e *EncodingHelper) GetMjpegEncoder(state *EncodingJobInfo, options *configuration.EncodingOptions) string {
	if state.VideoType == entities.VideoFile {
		hwType := options.HardwareAccelerationType
		if hwType != entities.HardwareAccelerationType_None && options.EnableHardwareEncoding {
			if preferredEncoder, ok := mjpegCodecMap[hwType]; ok && e.mediaEncoder.SupportsEncoder(preferredEncoder) {
				return preferredEncoder
			}
		}
	}

	return defaultMjpegEncoder
}

func (e *EncodingHelper) GetUserAgentParam(state *EncodingJobInfo) string {
	if userAgent, ok := state.RemoteHttpHeaders["User-Agent"]; ok {
		return "-user_agent \"" + userAgent + "\""
	}
	return ""
}

func (e *EncodingHelper) GetRefererParam(state *EncodingJobInfo) string {
	if referer, ok := state.RemoteHttpHeaders["Referer"]; ok {
		return "-referer \"" + referer + "\""
	}
	return ""
}

func (e *EncodingHelper) GetFastSeekCommandLineParameter(state *EncodingJobInfo, options *configuration.EncodingOptions, segmentContainer *string) string {
	var time int64 = 0
	if state.BaseRequest.StartTimeTicks != nil {
		time = *state.BaseRequest.StartTimeTicks
	}
	seekParam := ""

	if time > 0 {
		isHlsRemuxing := state.IsVideoRequest && state.TranscodingType == Hls && IsCopyCodec(state.OutputVideoCodec)
		seekTick := time
		if isHlsRemuxing {
			seekTick += 5000000
		}
		seekParam += fmt.Sprintf("-ss %s", e.mediaEncoder.GetTimeParameter(seekTick))

		if state.IsVideoRequest {
			outputVideoCodec := e.GetVideoEncoder(state, options)
			segmentFormat := strings.TrimPrefix(GetSegmentFileExtension(segmentContainer), ".")

			if !strings.EqualFold(segmentFormat, "wtv") && segmentFormat != "ts" && state.TranscodingType != Progressive && !state.EnableBreakOnNonKeyFrames(outputVideoCodec) && time > 0 {
				seekParam += " -noaccurate_seek"
			}
		}
	}

	return seekParam
}

func GetInputFormat(container string) string {
	klog.Infoln(container)
	if container == "" || !validationRegex.MatchString(container) {
		return ""
	}

	container = strings.ReplaceAll(strings.ToLower(container), "mkv", "matroska")

	switch strings.ToLower(container) {
	case "ts":
		return "mpegts"
	case "m2ts", "wmv", "mts", "vob", "mpg", "mpeg", "rec", "dvr-ms", "ogm", "divx", "tp", "rmvb", "rtp", "m4v", "strm", "iso":
		klog.Infoln("empty...................................||||||||||||||||||||||||||--------------->.")
		return ""
	default:
		return container
	}
}

func (e *EncodingHelper) GetVaapiVidFilterChain(state *EncodingJobInfo, options *configuration.EncodingOptions, vidEncoder string) ([]string, []string, []string) {
	if options.HardwareAccelerationType != entities.HardwareAccelerationType_VAAPI {
		return nil, nil, nil
	}

	isLinux := runtime.GOOS == "linux"
	vidDecoder := e.GetHardwareVideoDecoder(state, options)
	klog.Infoln("++++++++++++++++++", vidDecoder)
	isSwDecoder := vidDecoder == ""
	isSwEncoder := !strings.Contains(strings.ToLower(vidEncoder), "vaapi")
	isVaapiFullSupported := isLinux && e.IsVaapiSupported(state) && e.IsVaapiFullSupported()
	isVaapiOclSupported := isVaapiFullSupported && e.IsOpenclFullSupported()
	isVaapiVkSupported := isVaapiFullSupported && e.IsVulkanFullSupported()

	// legacy vaapi pipeline (copy-back)
	if (isSwDecoder && isSwEncoder) || !isVaapiOclSupported || !e.mediaEncoder.SupportsFilter("alphasrc") {
		//swFilterChain := e.GetSwVidFilterChain(state, options, vidEncoder)
		type SwFilterChain struct {
			MainFilters    []string
			SubFilters     []string
			OverlayFilters []string
		}
		mainFilters, subFilters, overlayFilters := e.GetSwVidFilterChain(state, options, vidEncoder)
		var swFilterChain = &SwFilterChain{
			MainFilters:    mainFilters,
			SubFilters:     subFilters,
			OverlayFilters: overlayFilters,
		}
		if !isSwEncoder {
			newfilters := append([]string{}, swFilterChain.MainFilters...)
			if len(swFilterChain.OverlayFilters) == 0 {
				newfilters = append(newfilters, "hwupload=derive_device=vaapi")
			} else {
				newfilters = append(swFilterChain.OverlayFilters, "hwupload=derive_device=vaapi")
			}
			return newfilters, swFilterChain.SubFilters, swFilterChain.OverlayFilters
		}
		return swFilterChain.MainFilters, swFilterChain.SubFilters, swFilterChain.OverlayFilters
	}

	// preferred vaapi + opencl filters pipeline
	if e.mediaEncoder.IsVaapiDeviceInteliHD() {
		return e.GetIntelVaapiFullVidFiltersPrefered(*state, *options, vidDecoder, vidEncoder)
	}

	// preferred vaapi + vulkan filters pipeline
	if e.mediaEncoder.IsVaapiDeviceAmd() && isVaapiVkSupported && e.mediaEncoder.IsVaapiDeviceSupportVulkanDrmInterop() /*&& systemVersion >= minKernelVersionAmdVkFmtModifier*/ {
		return e.GetAmdVaapiFullVidFiltersPrefered(*state, *options, vidDecoder, vidEncoder)
	}

	// Intel i965 and Amd legacy driver path, only featuring scale and deinterlace support
	return e.GetVaapiLimitedVidFiltersPrefered(*state, *options, vidDecoder, vidEncoder)
}

func (e *EncodingHelper) GetIntelVidFilterChain(state *EncodingJobInfo, options *configuration.EncodingOptions, vidEncoder string) ([]string, []string, []string) {
	if options.HardwareAccelerationType != entities.HardwareAccelerationType_QSV {
		return nil, nil, nil
	}

	isWindows := runtime.GOOS == "windows"
	isLinux := runtime.GOOS == "linux"
	vidDecoder := e.GetHardwareVideoDecoder(state, options)
	isSwDecoder := vidDecoder == ""
	isSwEncoder := !strings.Contains(strings.ToLower(vidEncoder), "qsv")
	isQsvOclSupported := e.mediaEncoder.SupportsHwaccel("qsv") && e.IsOpenclFullSupported()
	isIntelDx11OclSupported := isWindows && e.mediaEncoder.SupportsHwaccel("d3d11va") && isQsvOclSupported
	isIntelVaapiOclSupported := isLinux && e.IsVaapiSupported(state) && isQsvOclSupported

	// legacy qsv pipeline(copy-back)
	if (isSwDecoder && isSwEncoder) || (!isIntelVaapiOclSupported && !isIntelDx11OclSupported) || !e.mediaEncoder.SupportsFilter("alphasrc") {
		return e.GetSwVidFilterChain(state, options, vidEncoder)
	}

	// preferred qsv(vaapi) + opencl filters pipeline
	if isIntelVaapiOclSupported {
		return e.GetIntelQsvVaapiVidFiltersPrefered(*state, *options, vidDecoder, vidEncoder)
	}

	// preferred qsv(d3d11) + opencl filters pipeline
	if isIntelDx11OclSupported {
		return e.GetIntelQsvDx11VidFiltersPrefered(*state, *options, vidDecoder, vidEncoder)
	}

	return nil, nil, nil
}

func (e *EncodingHelper) GetNvidiaVidFiltersPrefered(
	state EncodingJobInfo,
	options configuration.EncodingOptions,
	vidDecoder string,
	vidEncoder string,
) ([]string, []string, []string) {
	var inW, inH, reqW, reqH, reqMaxW, reqMaxH *int
	var threeDFormat *entities.Video3DFormat

	if state.VideoStream != nil {
		inW = state.VideoStream.Width
		inH = state.VideoStream.Height
	}
	if state.BaseRequest != nil {
		reqW = state.BaseRequest.Width
		reqH = state.BaseRequest.Height
		reqMaxW = state.BaseRequest.MaxWidth
		reqMaxH = state.BaseRequest.MaxHeight
	}
	if state.MediaSource != nil {
		threeDFormat = state.MediaSource.Video3DFormat
	}

	isNvDecoder := strings.Contains(strings.ToLower(vidDecoder), "cuda")
	isNvencEncoder := strings.Contains(strings.ToLower(vidEncoder), "nvenc")
	isSwDecoder := vidDecoder == ""
	isSwEncoder := !isNvencEncoder
	isMjpegEncoder := strings.Contains(strings.ToLower(vidEncoder), "mjpeg")
	isCuInCuOut := isNvDecoder && isNvencEncoder

	//unused
	//	doubleRateDeint := options.DeinterlaceDoubleRate && (state.VideoStream == nil || state.VideoStream.ReferenceFrameRate <= 30)
	doDeintH264 := state.DeInterlace("h264", true) || state.DeInterlace("avc", true)
	doDeintHevc := state.DeInterlace("h265", true) || state.DeInterlace("hevc", true)
	doDeintH2645 := doDeintH264 || doDeintHevc
	doCuTonemap := e.IsHwTonemapAvailable(&state, &options)

	hasSubs := state.SubtitleStream != nil && ShouldEncodeSubtitle(state)
	hasTextSubs := hasSubs && state.SubtitleStream.IsTextSubtitleStream()
	hasGraphicalSubs := hasSubs && !state.SubtitleStream.IsTextSubtitleStream()
	hasAssSubs := hasSubs && (strings.EqualFold(state.SubtitleStream.Codec, "ass") || strings.EqualFold(state.SubtitleStream.Codec, "ssa"))
	var subW, subH *int
	if state.SubtitleStream != nil {
		subW = state.SubtitleStream.Width
		subH = state.SubtitleStream.Height
	}

	rotation := 0
	if state.VideoStream != nil {
		if state.VideoStream.Rotation != nil {
			rotation = *state.VideoStream.Rotation
		}
	}
	transposeDir := ""
	if rotation != 0 {
		transposeDir = e.GetVideoTransposeDirection(state)
	}
	doCuTranspose := transposeDir != "" && e.mediaEncoder.SupportsFilter("transpose_cuda")
	swapWAndH := rotation != 0 && (rotation == 90 || rotation == -90) && (isSwDecoder || (isNvDecoder && doCuTranspose))
	swpInW := inW
	swpInH := inH
	if swapWAndH {
		swpInW = inH
		swpInH = inW
	}

	// Make main filters for video stream
	mainFilters := []string{}
	mainFilters = append(mainFilters, e.GetOverwriteColorPropertiesParam(state, doCuTonemap))

	if isSwDecoder {
		// INPUT sw surface (memory)
		// sw deint
		if doDeintH2645 {
			swDeintFilter := GetSwDeinterlaceFilter(state, options)
			mainFilters = append(mainFilters, swDeintFilter)
		}

		outFormat := "yuv420p"
		if doCuTonemap {
			outFormat = "yuv420p10le"
		}
		swScaleFilter := GetSwScaleFilter(&state, &options, vidEncoder, swpInW, swpInH, threeDFormat, reqW, reqH, reqMaxW, reqMaxH)
		mainFilters = append(mainFilters, swScaleFilter)
		mainFilters = append(mainFilters, "format="+outFormat)

		// sw => hw
		if doCuTonemap {
			mainFilters = append(mainFilters, "hwupload=derive_device=cuda")
		}
	}

	if isNvDecoder {
		// INPUT cuda surface (vram)
		// hw deint
		if doDeintH2645 {
			deintFilter := GetHwDeinterlaceFilter(state, options, "cuda")
			mainFilters = append(mainFilters, deintFilter)
		}

		// hw transpose
		if doCuTranspose {
			mainFilters = append(mainFilters, "transpose_cuda=dir="+transposeDir)
		}

		isRext := e.IsVideoStreamHevcRext(state)
		outFormat := "yuv420p"
		if doCuTonemap {
			if isRext {
				outFormat = "p010"
			} else {
				outFormat = ""
			}
		}
		hwScaleFilter := e.GetHwScaleFilter("scale", "cuda", outFormat, false, swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH)
		mainFilters = append(mainFilters, hwScaleFilter)
	}

	// hw tonemap
	if doCuTonemap {
		tonemapFilter := GetHwTonemapFilter(options, "cuda", "yuv420p", isMjpegEncoder)
		mainFilters = append(mainFilters, tonemapFilter)
	}

	memoryOutput := false
	isUploadForCuTonemap := isSwDecoder && doCuTonemap
	if (isNvDecoder && isSwEncoder) || (isUploadForCuTonemap && hasSubs) {
		memoryOutput = true
		mainFilters = append(mainFilters, "hwdownload", "format=yuv420p")
	}

	// OUTPUT yuv420p surface (memory)
	if isSwDecoder && isNvencEncoder && !isUploadForCuTonemap {
		memoryOutput = true
	}

	if memoryOutput && hasTextSubs {
		textSubtitlesFilter := e.GetTextSubtitlesFilter(state, false, false)
		mainFilters = append(mainFilters, textSubtitlesFilter)
	}

	// Make sub and overlay filters for subtitle stream
	subFilters := []string{}
	overlayFilters := []string{}
	if isCuInCuOut && hasSubs {
		alphaFormatOpt := ""
		if hasGraphicalSubs {
			subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, reqMaxH)
			subFilters = append(subFilters, subPreProcFilters, "format=yuva420p")
		} else if hasTextSubs {
			framerate := 25.0
			if state.VideoStream != nil && state.VideoStream.RealFrameRate != nil {
				framerate = float64(*state.VideoStream.RealFrameRate)
			}
			subFramerate := 10.0
			if hasAssSubs {
				if framerate > 60 {
					subFramerate = 60
				} else {
					subFramerate = framerate
				}
			}

			alphaSrcFilter := GetAlphaSrcFilter(state, swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH, &subFramerate)
			subTextSubtitlesFilter := e.GetTextSubtitlesFilter(state, true, true)
			subFilters = append(subFilters, alphaSrcFilter, "format=yuva420p", subTextSubtitlesFilter)

			if e.mediaEncoder.SupportsFilterWithOption(OverlayCudaAlphaFormat) {
				alphaFormatOpt = ":alpha_format=premultiplied"
			}
		}
		subFilters = append(subFilters, "hwupload=derive_device=cuda")
		overlayFilters = append(overlayFilters, "overlay_cuda=eof_action=pass:repeatlast=0"+alphaFormatOpt)
	} else if hasGraphicalSubs {
		subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, reqMaxH)
		subFilters = append(subFilters, subPreProcFilters)
		overlayFilters = append(overlayFilters, "overlay=eof_action=pass:repeatlast=0")
	}

	return mainFilters, subFilters, overlayFilters
}

func (e *EncodingHelper) GetNvidiaVidFilterChain(state *EncodingJobInfo, options *configuration.EncodingOptions, vidEncoder string) ([]string, []string, []string) {
	if options.HardwareAccelerationType != entities.HardwareAccelerationType_NVENC {
		return nil, nil, nil
	}

	vidDecoder := e.GetHardwareVideoDecoder(state, options)
	isSwDecoder := vidDecoder == ""
	isSwEncoder := !strings.Contains(strings.ToLower(vidEncoder), "nvenc")

	// legacy cuvid pipeline(copy-back)
	if (isSwDecoder && isSwEncoder) || !e.IsCudaFullSupported() || !e.mediaEncoder.SupportsFilter("alphasrc") {
		return e.GetSwVidFilterChain(state, options, vidEncoder)
	}

	// preferred nvdec/cuvid + cuda filters + nvenc pipeline
	return e.GetNvidiaVidFiltersPrefered(*state, *options, vidDecoder, vidEncoder)
}

func (e *EncodingHelper) GetAmdVidFilterChain(state *EncodingJobInfo, options *configuration.EncodingOptions, vidEncoder string) ([]string, []string, []string) {
	return nil, nil, nil
	/*
		if options.HardwareAccelerationType != "amf" {
			return nil, nil, nil
		}

		isWindows := runtime.GOOS == "windows"
		vidDecoder := e.GetHardwareVideoDecoder(state, options)
		isSwDecoder := vidDecoder == ""
		isSwEncoder := !strings.Contains(strings.ToLower(vidEncoder), "amf")
		isAmfDx11OclSupported := isWindows && e.mediaEncoder.SupportsHwaccel("d3d11va") && e.IsOpenclFullSupported()

		if (isSwDecoder && isSwEncoder) || !isAmfDx11OclSupported || !e.mediaEncoder.SupportsFilter("alphasrc") {
			return e.GetSwVidFilterChain(state, options, vidEncoder)
		}

		return GetAmdDx11VidFiltersPrefered(state, options, vidDecoder, vidEncoder)
	*/
}

func (e *EncodingHelper) GetAppleVidFilterChain(state *EncodingJobInfo, options *configuration.EncodingOptions, vidEncoder string) ([]string, []string, []string) {
	return nil, nil, nil
	/*
		if options.HardwareAccelerationType != "videotoolbox" {
			return nil, nil, nil
		}

		isMacOS := runtime.GOOS == "darwin"
		vidDecoder := e.GetHardwareVideoDecoder(state, options)
		isVtEncoder := strings.Contains(strings.ToLower(vidEncoder), "videotoolbox")
		isVtFullSupported := isMacOS && e.IsVideoToolboxFullSupported()

		if !isVtEncoder || !isVtFullSupported || !SupportsFilter(state, "alphasrc") {
			return e.GetSwVidFilterChain(state, options, vidEncoder)
		}

		return GetAppleVidFiltersPreferred(state, options, vidDecoder, vidEncoder)
	*/
}

func (e *EncodingHelper) GetRkmppVidFilterChain(state *EncodingJobInfo, options *configuration.EncodingOptions, vidEncoder string) ([]string, []string, []string) {
	/*
		if options.HardwareAccelerationType != "rkmpp" {
			return nil, nil, nil
		}

		isLinux := runtime.GOOS == "linux"
		vidDecoder := e.GetHardwareVideoDecoder(state, options)
		isSwDecoder := vidDecoder == ""
		isSwEncoder := !strings.Contains(strings.ToLower(vidEncoder), "rkmpp")
		isRkmppOclSupported := isLinux && IsRkmppFullSupported() && e.IsOpenclFullSupported()

		if (isSwDecoder && isSwEncoder) || !isRkmppOclSupported || !SupportsFilter(state, "alphasrc") {
			return e.GetSwVidFilterChain(state, options, vidEncoder)
		}

		if isRkmppOclSupported {
			return e.GetRkmppVidFiltersPrefered(state, options, vidDecoder, vidEncoder)
		}

	*/
	return nil, nil, nil
}

func (e *EncodingHelper) GetSwVidFilterChain(state *EncodingJobInfo, options *configuration.EncodingOptions, vidEncoder string) ([]string, []string, []string) {
	inW, inH := state.VideoStream.Width, state.VideoStream.Height
	reqW, reqH, reqMaxW, reqMaxH := state.BaseRequest.Width, state.BaseRequest.Height, state.BaseRequest.MaxWidth, state.BaseRequest.MaxHeight
	threeDFormat := state.MediaSource.Video3DFormat

	vidDecoder := e.GetHardwareVideoDecoder(state, options)
	isSwDecoder := vidDecoder == ""
	isVaapiEncoder := strings.Contains(strings.ToLower(vidEncoder), "vaapi")
	isV4l2Encoder := strings.Contains(strings.ToLower(vidEncoder), "h264_v4l2m2m")

	doDeintH264 := state.DeInterlace("h264", true) || state.DeInterlace("avc", true)
	doDeintHevc := state.DeInterlace("h265", true) || state.DeInterlace("hevc", true)
	doDeintH2645 := doDeintH264 || doDeintHevc

	hasSubs := state.SubtitleStream != nil && state.SubtitleDeliveryMethod == dlna.Encode
	hasTextSubs := hasSubs && state.SubtitleStream.IsTextSubtitleStream()
	hasGraphicalSubs := hasSubs && !state.SubtitleStream.IsTextSubtitleStream()

	rotation := 0
	if state.VideoStream != nil && state.VideoStream.Rotation != nil {
		rotation = *state.VideoStream.Rotation
	}
	swapWAndH := math.Abs(float64(rotation)) == 90
	var swpInW, swpInH *int
	if swapWAndH {
		swpInW = inH
		swpInH = inW
	} else {
		swpInW = inW
		swpInH = inH
	}

	mainFilters := []string{
		e.GetOverwriteColorPropertiesParam(*state, false),
	}

	// INPUT sw surface(memory/copy-back from vram)
	// sw deint
	if doDeintH2645 {
		mainFilters = append(mainFilters, GetSwDeinterlaceFilter(*state, *options))
	}

	outFormat := "yuv420p"
	if isSwDecoder {
		outFormat = "nv12"
	}
	if isVaapiEncoder {
		outFormat = "nv12"
	} else if isV4l2Encoder {
		outFormat = "yuv420p"
	}

	mainFilters = append(mainFilters, GetSwScaleFilter(state, options, vidEncoder, inW, inH, threeDFormat, reqW, reqH, reqMaxW, reqMaxH))
	mainFilters = append(mainFilters, "format="+outFormat)

	subFilters := []string{}
	overlayFilters := []string{}
	if hasTextSubs {
		mainFilters = append(mainFilters, e.GetTextSubtitlesFilter(*state, false, false))
	} else if hasGraphicalSubs {
		subFilters = append(subFilters, GetGraphicalSubPreProcessFilters(inW, inH, swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH))
		overlayFilters = append(overlayFilters, "overlay=eof_action=pass:repeatlast=0")
	}

	return mainFilters, subFilters, overlayFilters
}

func (e *EncodingHelper) GetFramerateParam(state *EncodingJobInfo) *float32 {
	if state.BaseRequest.Framerate != nil {
		return state.BaseRequest.Framerate
	}

	maxrate := state.BaseRequest.MaxFramerate
	if maxrate != nil && state.VideoStream != nil {
		contentRate := state.VideoStream.AverageFrameRate
		if contentRate == nil {
			contentRate = state.VideoStream.RealFrameRate
		}
		if contentRate != nil && *contentRate > *maxrate {
			return maxrate
		}
	}

	return nil
}

func (e *EncodingHelper) GGetHardwareVideoDecoder(state *EncodingJobInfo, options *configuration.EncodingOptions) string {
	klog.Infoln("GetHardwareVideoDecoder 000000000000000000000")
	if state.VideoStream == nil || state.MediaSource == nil {
		return ""
	}
	// HWA decoders can handle both video files and video folders.
	if state.VideoType != entities.VideoFile &&
		state.VideoType != entities.Iso &&
		state.VideoType != entities.Dvd &&
		state.VideoType != entities.BluRay {
		return ""
	}
	if IsCopyCodec(state.OutputVideoCodec) {

		return ""
	}
	hardwareAccelerationType := options.HardwareAccelerationType
	if state.VideoStream.Codec != "" && hardwareAccelerationType != entities.HardwareAccelerationType_None {
		bitDepth := GetVideoColorBitDepth(state)

		// Only HEVC, VP9 and AV1 formats have 10-bit hardware decoder support now.
		if bitDepth == 10 &&
			!(strings.EqualFold(state.VideoStream.Codec, "hevc") ||
				strings.EqualFold(state.VideoStream.Codec, "h265") ||
				strings.EqualFold(state.VideoStream.Codec, "vp9") ||
				strings.EqualFold(state.VideoStream.Codec, "av1")) {

			hasHardwareHi10P := hardwareAccelerationType == entities.HardwareAccelerationType_RKMPP

			// VideoToolbox on Apple Silicon has H.264 Hi10P mode enabled after macOS 14.6
			if hardwareAccelerationType == entities.HardwareAccelerationType_VideoToolbox {
				ver := runtime.GOOS
				arch := runtime.GOARCH

				if arch == "arm64" && ver >= "14.6" {
					hasHardwareHi10P = true
				}
			}
			if !hasHardwareHi10P &&
				strings.EqualFold(state.VideoStream.Codec, "h264") {
				return ""
			}
		}
		switch options.HardwareAccelerationType {
		case entities.HardwareAccelerationType_QSV:
			return e.GetQsvHwVidDecoder(state, options, state.VideoStream, bitDepth)
		case entities.HardwareAccelerationType_NVENC:
			return e.GetNvdecVidDecoder(state, options, state.VideoStream, bitDepth)
		case entities.HardwareAccelerationType_AMF:
			return e.GetAmfVidDecoder(state, options, state.VideoStream, bitDepth)
		case entities.HardwareAccelerationType_VAAPI:
			return e.GetVaapiVidDecoder(state, options, state.VideoStream, bitDepth)
		case entities.HardwareAccelerationType_VideoToolbox:
			return e.GetVideotoolboxVidDecoder(state, options, state.VideoStream, bitDepth)
		case entities.HardwareAccelerationType_RKMPP:
			return e.GetRkmppVidDecoder(state, options, state.VideoStream, bitDepth)
		}
	}
	/*
		whichCodec := state.VideoStream.Codec
		if strings.EqualFold(whichCodec, "avc") {
			whichCodec = "h264"
		} else if strings.EqualFold(whichCodec, "h265") {
			whichCodec = "hevc"
		}
		// Avoid a second attempt if no hardware acceleration is being used
		for i, codec := range options.HardwareDecodingCodecs {
			if strings.EqualFold(codec, whichCodec) {
				options.HardwareDecodingCodecs = append(options.HardwareDecodingCodecs[:i], options.HardwareDecodingCodecs[i+1:]...)
				break
			}
		}
	*/

	// leave blank so ffmpeg will decide
	return ""
}

// GetHardwareVideoDecoder determines the appropriate hardware video decoder based on the encoding job state and options.
func (e *EncodingHelper) GetHardwareVideoDecoder(state *EncodingJobInfo, options *configuration.EncodingOptions) string {
	videoStream := state.VideoStream
	mediaSource := state.MediaSource
	if videoStream == nil || mediaSource == nil {
		return ""
	}

	// HWA decoders can handle both video files and video folders.
	videoType := state.VideoType
	if videoType != entities.VideoFile &&
		videoType != entities.Iso &&
		videoType != entities.Dvd &&
		videoType != entities.BluRay {
		return ""
	}

	if IsCopyCodec(state.OutputVideoCodec) {
		return ""
	}

	hardwareAccelerationType := options.HardwareAccelerationType

	if videoStream.Codec != "" && hardwareAccelerationType != entities.HardwareAccelerationType_None {
		bitDepth := GetVideoColorBitDepth(state)

		// Only HEVC, VP9, and AV1 formats have 10-bit hardware decoder support for most platforms
		if bitDepth == 10 &&
			!(strings.EqualFold(videoStream.Codec, "hevc") ||
				strings.EqualFold(videoStream.Codec, "h265") ||
				strings.EqualFold(videoStream.Codec, "vp9") ||
				strings.EqualFold(videoStream.Codec, "av1")) {
			// RKMPP has H.264 Hi10P decoder
			hasHardwareHi10P := hardwareAccelerationType == entities.HardwareAccelerationType_RKMPP

			// VideoToolbox on Apple Silicon has H.264 Hi10P mode enabled after macOS 14.6
			if hardwareAccelerationType == entities.HardwareAccelerationType_VideoToolbox {
				arch := runtime.GOARCH
				if arch == "arm64" && isMacOSVersionAtLeast(14, 6) {
					hasHardwareHi10P = true
				}
			}

			if !hasHardwareHi10P && strings.EqualFold(videoStream.Codec, "h264") {
				return ""
			}
		}

		var decoder string
		switch hardwareAccelerationType {
		case entities.HardwareAccelerationType_VAAPI:
			decoder = e.GetVaapiVidDecoder(state, options, videoStream, bitDepth)
		case entities.HardwareAccelerationType_AMF:
			decoder = e.GetAmfVidDecoder(state, options, videoStream, bitDepth)
		case entities.HardwareAccelerationType_QSV:
			decoder = e.GetQsvHwVidDecoder(state, options, videoStream, bitDepth)
		case entities.HardwareAccelerationType_NVENC:
			decoder = e.GetNvdecVidDecoder(state, options, videoStream, bitDepth)
		case entities.HardwareAccelerationType_VideoToolbox:
			decoder = e.GetVideotoolboxVidDecoder(state, options, videoStream, bitDepth)
		case entities.HardwareAccelerationType_RKMPP:
			decoder = e.GetRkmppVidDecoder(state, options, videoStream, bitDepth)
		default:
			decoder = ""
		}

		if decoder != "" {
			return decoder
		}
	}

	// Leave blank so ffmpeg will decide
	return ""
}

// isMacOSVersionAtLeast checks if the macOS version is at least the specified major and minor version.
func isMacOSVersionAtLeast(major, minor int) bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	versionStr := strings.TrimSpace(string(output))
	parts := strings.Split(versionStr, ".")
	if len(parts) < 2 {
		return false
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minorVersion, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	return majorVersion > major || (majorVersion == major && minorVersion >= minor)
}

func (e *EncodingHelper) GetIntelVaapiFullVidFiltersPrefered(
	state EncodingJobInfo,
	options configuration.EncodingOptions,
	vidDecoder string,
	vidEncoder string,
) (mainFilters, subFilters, overlayFilters []string) {
	inW := state.VideoStream.Width
	inH := state.VideoStream.Height
	reqW := state.BaseRequest.Width
	reqH := state.BaseRequest.Height
	reqMaxW := state.BaseRequest.MaxWidth
	reqMaxH := state.BaseRequest.MaxHeight
	threeDFormat := state.MediaSource.Video3DFormat

	isVaapiDecoder := strings.Contains(strings.ToLower(vidDecoder), "vaapi")
	isVaapiEncoder := strings.Contains(strings.ToLower(vidEncoder), "vaapi")
	isSwDecoder := vidDecoder == ""
	isSwEncoder := !isVaapiEncoder
	isMjpegEncoder := strings.Contains(strings.ToLower(vidEncoder), "mjpeg")
	isVaInVaOut := isVaapiDecoder && isVaapiEncoder

	doDeintH264 := state.DeInterlace("h264", true) || state.DeInterlace("avc", true)
	doDeintHevc := state.DeInterlace("h265", true) || state.DeInterlace("hevc", true)
	doVaVppTonemap := isVaapiDecoder && e.IsVaapiVppTonemapAvailable(&state, &options)
	doOclTonemap := !doVaVppTonemap && e.IsHwTonemapAvailable(&state, &options)
	doTonemap := doVaVppTonemap || doOclTonemap
	doDeintH2645 := doDeintH264 || doDeintHevc

	hasSubs := state.SubtitleStream != nil && state.SubtitleDeliveryMethod == dlna.Encode
	hasTextSubs := hasSubs && state.SubtitleStream.IsTextSubtitleStream()
	hasGraphicalSubs := hasSubs && !state.SubtitleStream.IsTextSubtitleStream()
	hasAssSubs := hasSubs && (strings.EqualFold(state.SubtitleStream.Codec, "ass") || strings.EqualFold(state.SubtitleStream.Codec, "ssa"))

	var subW, subH *int
	if state.SubtitleStream != nil {
		subW = state.SubtitleStream.Width
		subH = state.SubtitleStream.Height
	}

	var rotation int
	if state.VideoStream != nil && state.VideoStream.Rotation != nil {
		rotation = *state.VideoStream.Rotation
	} else {
		rotation = 0
	}

	transposeDir := ""
	if rotation != 0 {
		transposeDir = e.GetVideoTransposeDirection(state)
	}

	doVaVppTranspose := transposeDir != ""
	swapWAndH := math.Abs(float64(rotation)) == 90 && (isSwDecoder || (isVaapiDecoder && doVaVppTranspose))

	var swpInW, swpInH *int
	if swapWAndH {
		swpInW = inH
		swpInH = inW
	} else {
		swpInW = inW
		swpInH = inH
	}

	// Make main filters for video stream
	mainFilters = append(mainFilters, e.GetOverwriteColorPropertiesParam(state, doTonemap))

	if isSwDecoder {
		// INPUT sw surface(memory)
		// sw deint
		if doDeintH2645 {
			swDeintFilter := GetSwDeinterlaceFilter(state, options)
			mainFilters = append(mainFilters, swDeintFilter)
		}

		outFormat := "nv12"
		if doOclTonemap {
			outFormat = "yuv420p10le"
		}
		swScaleFilter := GetSwScaleFilter(&state, &options, vidEncoder, inW, inH, threeDFormat, reqW, reqH, reqMaxW, reqMaxH)
		if isMjpegEncoder && !doOclTonemap {
			// sw decoder + hw mjpeg encoder
			if swScaleFilter == "" {
				swScaleFilter = "scale=out_range=pc"
			} else {
				swScaleFilter = fmt.Sprintf("%s:out_range=pc", swScaleFilter)
			}
		}

		// sw scale
		mainFilters = append(mainFilters, swScaleFilter)
		mainFilters = append(mainFilters, "format="+outFormat)

		// keep video at memory except ocl tonemap,
		// since the overhead caused by hwupload >>> using sw filter.
		// sw => hw
		if doOclTonemap {
			mainFilters = append(mainFilters, "hwupload=derive_device=opencl")
		}
	} else if isVaapiDecoder {
		isRext := e.IsVideoStreamHevcRext(state)

		// INPUT vaapi surface(vram)
		// hw deint
		if doDeintH2645 {
			deintFilter := GetHwDeinterlaceFilter(state, options, "vaapi")
			mainFilters = append(mainFilters, deintFilter)
		}

		// hw transpose
		if doVaVppTranspose {
			mainFilters = append(mainFilters, fmt.Sprintf("transpose_vaapi=dir=%s", transposeDir))
		}

		outFormat := "nv12"
		if doTonemap {
			if isRext {
				outFormat = "p010"
			} else {
				outFormat = ""
			}
		}
		hwScaleFilter := e.GetHwScaleFilter("scale", "vaapi", outFormat, false, inW, inH, reqW, reqH, reqMaxW, reqMaxH)
		if hwScaleFilter != "" && isMjpegEncoder {
			if !doOclTonemap {
				hwScaleFilter += ":out_range=pc"
			}
			hwScaleFilter += ":mode=hq"
		}

		// allocate extra pool sizes for vaapi vpp
		if hwScaleFilter != "" {
			hwScaleFilter += ":extra_hw_frames=24"
		}

		// hw scale
		mainFilters = append(mainFilters, hwScaleFilter)
	}

	// vaapi vpp tonemap
	if doVaVppTonemap && isVaapiDecoder {
		tonemapFilter := GetHwTonemapFilter(options, "vaapi", "nv12", isMjpegEncoder)
		mainFilters = append(mainFilters, tonemapFilter)
	}

	if doOclTonemap && isVaapiDecoder {
		// map from vaapi to opencl via vaapi-opencl interop(Intel only).
		mainFilters = append(mainFilters, "hwmap=derive_device=opencl")
	}

	// ocl tonemap
	if doOclTonemap {
		tonemapFilter := GetHwTonemapFilter(options, "opencl", "nv12", isMjpegEncoder)
		mainFilters = append(mainFilters, tonemapFilter)
	}

	if doOclTonemap && isVaInVaOut {
		// OUTPUT vaapi(nv12) surface(vram)
		// reverse-mapping via vaapi-opencl interop.
		mainFilters = append(mainFilters, "hwmap=derive_device=vaapi:reverse=1")
		mainFilters = append(mainFilters, "format=vaapi")
	}

	memoryOutput := false
	isUploadForOclTonemap := isSwDecoder && doOclTonemap
	isHwmapNotUsable := isUploadForOclTonemap && isVaapiEncoder
	if (isVaapiDecoder && isSwEncoder) || isUploadForOclTonemap {
		memoryOutput = true

		// OUTPUT nv12 surface(memory)
		// prefer hwmap to hwdownload on opencl/vaapi.
		if isHwmapNotUsable {
			mainFilters = append(mainFilters, "hwdownload")
		} else {
			mainFilters = append(mainFilters, "hwmap=mode=read")
		}
		mainFilters = append(mainFilters, "format=nv12")
	}

	// OUTPUT nv12 surface(memory)
	if isSwDecoder && isVaapiEncoder {
		memoryOutput = true
	}

	if memoryOutput {
		// text subtitles
		if hasTextSubs {
			textSubtitlesFilter := e.GetTextSubtitlesFilter(state, false, false)
			mainFilters = append(mainFilters, textSubtitlesFilter)
		}
	}

	if memoryOutput && isVaapiEncoder {
		if !hasGraphicalSubs {
			mainFilters = append(mainFilters, "hwupload_vaapi")
		}
	}
	// Make sub and overlay filters for subtitle stream
	subFilters = make([]string, 0)
	overlayFilters = make([]string, 0)
	if isVaInVaOut {
		if hasSubs {
			if hasGraphicalSubs {
				// overlay_vaapi can handle overlay scaling, setup a smaller height to reduce transfer overhead
				reqMaxH := 1080
				subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, &reqMaxH)
				subFilters = append(subFilters, subPreProcFilters)
				subFilters = append(subFilters, "format=bgra")
			} else if hasTextSubs {
				framerate := state.VideoStream.RealFrameRate
				var subFramerate float64
				if hasAssSubs {
					framerateTmp := float64(*framerate)
					subFramerate = math.Min(framerateTmp, 60)
				} else {
					subFramerate = 10
				}

				reqMaxH := 1080
				alphaSrcFilter := GetAlphaSrcFilter(state, inW, inH, reqW, reqH, reqMaxW, &reqMaxH, &subFramerate)
				subTextSubtitlesFilter := e.GetTextSubtitlesFilter(state, true, true)
				subFilters = append(subFilters, alphaSrcFilter)
				subFilters = append(subFilters, "format=bgra")
				subFilters = append(subFilters, subTextSubtitlesFilter)
			}

			subFilters = append(subFilters, "hwupload=derive_device=vaapi")

			overlayW, overlayH := GetFixedOutputSize(inW, inH, reqW, reqH, reqMaxW, reqMaxH)
			var overlaySize string
			if overlayW != nil && overlayH != nil {
				overlaySize = fmt.Sprintf(":w=%d:h=%d", *overlayW, *overlayH)
			} else {
				overlaySize = ""
			}
			overlayVaapiFilter := fmt.Sprintf("overlay_vaapi=eof_action=pass:repeatlast=0%s", overlaySize)
			overlayFilters = append(overlayFilters, overlayVaapiFilter)
		}
	} else if memoryOutput {
		if hasGraphicalSubs {
			subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, reqMaxH)
			subFilters = append(subFilters, subPreProcFilters)
			overlayFilters = append(overlayFilters, "overlay=eof_action=pass:repeatlast=0")

			if isVaapiEncoder {
				overlayFilters = append(overlayFilters, "hwupload_vaapi")
			}
		}
	}

	return mainFilters, subFilters, overlayFilters
}

func (e *EncodingHelper) GetOverwriteColorPropertiesParam(state EncodingJobInfo, isTonemapAvailable bool) string {
	if isTonemapAvailable {
		return e.GetInputHdrParam(state.VideoStream.ColorTransfer)
	}
	return e.GetOutputSdrParam("")
}

func (e *EncodingHelper) GetInputHdrParam(colorTransfer string) string {
	if strings.EqualFold(colorTransfer, "arib-std-b67") {
		// HLG
		return "setparams=color_primaries=bt2020:color_trc=arib-std-b67:colorspace=bt2020nc"
	}

	// HDR10
	return "setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc"
}

func (e *EncodingHelper) GetOutputSdrParam(tonemappingRange string) string {
	// SDR
	if strings.EqualFold(tonemappingRange, "tv") {
		return "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709:range=tv"
	}

	if strings.EqualFold(tonemappingRange, "pc") {
		return "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709:range=pc"
	}

	return "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709"
}

func GetSwDeinterlaceFilter(state EncodingJobInfo, options configuration.EncodingOptions) string {
	doubleRateDeint := options.DeinterlaceDoubleRate && *state.VideoStream.AverageFrameRate <= 30
	var deintMethod string
	if strings.EqualFold(options.DeinterlaceMethod, "bwdif") {
		deintMethod = "bwdif"
	} else {
		deintMethod = "yadif"
	}
	return fmt.Sprintf("%s=%d:-1:0", deintMethod, boolToInt(doubleRateDeint))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (e *EncodingHelper) GetTextSubtitlesFilter(state EncodingJobInfo, enableAlpha, enableSub2video bool) string {
	return ""
	/*
	   seconds := math.Round(float64(*state.StartTimeTicks) / float64(time.Second))

	   var setPtsParam string
	   if state.CopyTimestamps || state.TranscodingType != Progressive {
	       setPtsParam = ""
	   } else {
	       setPtsParam = fmt.Sprintf(",setpts=PTS-%d/TB", int(seconds))
	   }

	   alphaParam := ""
	   if enableAlpha {
	       alphaParam = ":alpha=1"
	   }

	   sub2videoParam := ""
	   if enableSub2video {
	       sub2videoParam = ":sub2video=1"
	   }

	   fontPath := filepath.Join(_appPaths.CachePath, "attachments", state.MediaSource.Id)
	   fontParam := fmt.Sprintf(":fontsdir='%s'", e.mediaEncoder.EscapeSubtitleFilterPath(fontPath))

	   if state.SubtitleStream.IsExternal {
	       charsetParam := ""
	       if state.SubtitleStream.Language != "" {
	           charenc := e.mediaEncoder.GetSubtitleFileCharacterSet(_subtitleEncoder, state.SubtitleStream, state.SubtitleStream.Language, state.MediaSource, context.Background())
	           if charenc != "" {
	               charsetParam = ":charenc=" + charenc
	           }
	       }

	       return fmt.Sprintf("subtitles=f='%s'%s%s%s%s%s",
	           EscapeSubtitleFilterPath(e.mediaEncoder, state.SubtitleStream.Path),
	           charsetParam,
	           alphaParam,
	           sub2videoParam,
	           fontParam,
	           setPtsParam)
	   }

	   mediaPath := state.MediaPath
	   if mediaPath == "" {
	       mediaPath = ""
	   }

	   return fmt.Sprintf("subtitles=f='%s':si=%d%s%s%s%s",
	       e.mediaEncoder.EscapeSubtitleFilterPath(path),
	       state.InternalSubtitleStreamOffset,
	       alphaParam,
	       sub2videoParam,
	       fontParam,
	       setPtsParam)
	*/
}

func GetGraphicalSubPreProcessFilters(videoWidth, videoHeight, subtitleWidth, subtitleHeight, requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight *int) string {
	outWidth, outHeight := GetFixedOutputSize(videoWidth, videoHeight, requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight)

	if outWidth == nil || outHeight == nil || *outWidth <= 0 || *outHeight <= 0 {
		return ""
	}
	filters := "scale,scale=-1:%d:fast_bilinear,crop,pad=max(%d\\,iw):max(%d\\,ih):(ow-iw)/2:(oh-ih)/2:black@0,crop=%d:%d"
	if subtitleWidth != nil && subtitleHeight != nil && *subtitleWidth > 0 && *subtitleHeight > 0 {
		videoDar := float64(*outWidth) / float64(*outHeight)
		subtitleDar := float64(*subtitleWidth) / float64(*subtitleHeight)

		// No need to add padding when DAR is the same
		if math.Abs(videoDar-subtitleDar) < 0.01 {
			filters = "scale,scale=%d:%d:fast_bilinear"
		}
	}

	return fmt.Sprintf(filters, *outWidth, *outHeight, *outWidth, *outHeight)
}

func GetFixedOutputSize(videoWidth, videoHeight, requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight *int) (*int, *int) {
	if videoWidth == nil && requestedWidth == nil {
		return nil, nil
	}

	if videoHeight == nil && requestedHeight == nil {
		return nil, nil
	}

	inputWidth := 0
	if videoWidth != nil {
		inputWidth = *videoWidth
	} else if requestedWidth != nil {
		inputWidth = *requestedWidth
	}

	inputHeight := 0
	if videoHeight != nil {
		inputHeight = *videoHeight
	} else if requestedHeight != nil {
		inputHeight = *requestedHeight
	}

	outputWidth := inputWidth
	if requestedWidth != nil {
		outputWidth = *requestedWidth
	}

	outputHeight := inputHeight
	if requestedHeight != nil {
		outputHeight = *requestedHeight
	}

	maximumWidth := outputWidth
	if requestedMaxWidth != nil {
		maximumWidth = *requestedMaxWidth
	}
	if maximumWidth > 4096 {
		maximumWidth = 4096
	}

	maximumHeight := outputHeight
	if requestedMaxHeight != nil {
		maximumHeight = *requestedMaxHeight
	}
	if maximumHeight > 4096 {
		maximumHeight = 4096
	}

	if outputWidth > maximumWidth || outputHeight > maximumHeight {
		scaleW := float64(maximumWidth) / float64(outputWidth)
		scaleH := float64(maximumHeight) / float64(outputHeight)
		scale := math.Min(scaleW, scaleH)
		outputWidth = int(math.Min(float64(maximumWidth), float64(outputWidth)*scale))
		outputHeight = int(math.Min(float64(maximumHeight), float64(outputHeight)*scale))
	}

	outputWidth = 2 * (outputWidth / 2)
	outputHeight = 2 * (outputHeight / 2)

	return &outputWidth, &outputHeight
}

func getIntValue(a, b *int) int {
	if a != nil {
		return *a
	}
	return *b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func GetSwScaleFilter(
	state *EncodingJobInfo,
	options *configuration.EncodingOptions,
	videoEncoder string,
	videoWidth, videoHeight *int,
	threedFormat *entities.Video3DFormat,
	requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight *int,
) string {
	isV4l2 := strings.EqualFold(videoEncoder, "h264_v4l2m2m")
	isMjpeg := videoEncoder != "" && strings.Contains(strings.ToLower(videoEncoder), "mjpeg")
	scaleVal := 64
	if !isV4l2 {
		scaleVal = 2
	}
	targetAr := "(a*sar)"
	if isMjpeg {
		targetAr = "a"
	}

	// If fixed dimensions were supplied
	if requestedWidth != nil && requestedHeight != nil {
		if isV4l2 {
			widthParam := strconv.Itoa(*requestedWidth)
			heightParam := strconv.Itoa(*requestedHeight)
			return fmt.Sprintf("\"scale=trunc(%s/64)*64:trunc(%s/2)*2\"", widthParam, heightParam)
		}
		return GetFixedSwScaleFilter(threedFormat, *requestedWidth, *requestedHeight)
	}

	// If Max dimensions were supplied, for width selects lowest even number between input width and width req size and selects lowest even number from in width*display aspect and requested size
	if requestedMaxWidth != nil && requestedMaxHeight != nil {
		maxWidthParam := strconv.Itoa(*requestedMaxWidth)
		maxHeightParam := strconv.Itoa(*requestedMaxHeight)
		return fmt.Sprintf("\"scale=trunc(min(max(iw\\,ih*%s)\\,min(%s\\,%s*%s))/%d)*%d:trunc(min(max(iw/%s\\,ih)\\,min(%s/%s\\,%s))/2)*2\"",
			targetAr, maxWidthParam, maxHeightParam, targetAr, scaleVal, scaleVal, targetAr, maxWidthParam, targetAr, maxHeightParam)
	}

	// If a fixed width was requested
	if requestedWidth != nil {
		if threedFormat != nil {
			return GetFixedSwScaleFilter(threedFormat, *requestedWidth, 0)
		}
		widthParam := strconv.Itoa(*requestedWidth)
		return fmt.Sprintf("\"scale=%s:trunc(ow/%s/2)*2\"", widthParam, targetAr)
	}

	// If a fixed height was requested
	if requestedHeight != nil {
		heightParam := strconv.Itoa(*requestedHeight)
		return fmt.Sprintf("\"scale=trunc(oh*%s/%d)*%d:%s\"", targetAr, scaleVal, scaleVal, heightParam)
	}

	// If a max width was requested
	if requestedMaxWidth != nil {
		maxWidthParam := strconv.Itoa(*requestedMaxWidth)
		return fmt.Sprintf("\"scale=trunc(min(max(iw\\,ih*%s)\\,%s)/%d)*%d:trunc(ow/%s/2)*2\"",
			targetAr, maxWidthParam, scaleVal, scaleVal, targetAr)
	}

	// If a max height was requested
	if requestedMaxHeight != nil {
		maxHeightParam := strconv.Itoa(*requestedMaxHeight)
		return fmt.Sprintf("\"scale=trunc(oh*%s/%d)*%d:min(max(iw/%s\\,ih)\\,%s)\"",
			targetAr, scaleVal, scaleVal, targetAr, maxHeightParam)
	}

	return ""
}

func GetFixedSwScaleFilter(threedFormat *entities.Video3DFormat, requestedWidth, requestedHeight int) string {
	widthParam := strconv.Itoa(requestedWidth)
	heightParam := strconv.Itoa(requestedHeight)

	var filter string

	if threedFormat != nil {
		switch *threedFormat {
		case entities.HalfSideBySide:
			filter = `crop=iw/2:ih:0:0,scale=(iw*2):ih,setdar=dar=a,crop=min(iw\,ih*dar):min(ih\,iw/dar):(iw-min(iw\,iw*sar))/2:(ih - min (ih\,ih/sar))/2,setsar=sar=1,scale=%s:trunc(%s/dar/2)*2`
			// hsbs crop width in half,scale to correct size, set the display aspect,crop out any black bars we may have made the scale width to requestedWidth. Work out the correct height based on the display aspect it will maintain the aspect where -1 in this case (3d) may not.
		case entities.FullSideBySide:
			filter = `crop=iw/2:ih:0:0,setdar=dar=a,crop=min(iw\,ih*dar):min(ih\,iw/dar):(iw-min(iw\,iw*sar))/2:(ih - min (ih\,ih/sar))/2,setsar=sar=1,scale=%s:trunc(%s/dar/2)*2`
			// fsbs crop width in half,set the display aspect,crop out any black bars we may have made the scale width to requestedWidth.
		case entities.HalfTopAndBottom:
			filter = `crop=iw:ih/2:0:0,scale=(iw*2):ih),setdar=dar=a,crop=min(iw\,ih*dar):min(ih\,iw/dar):(iw-min(iw\,iw*sar))/2:(ih - min (ih\,ih/sar))/2,setsar=sar=1,scale=%s:trunc(%s/dar/2)*2`
			// htab crop height in half,scale to correct size, set the display aspect,crop out any black bars we may have made the scale width to requestedWidth
		case entities.FullTopAndBottom:
			filter = `crop=iw:ih/2:0:0,setdar=dar=a,crop=min(iw\,ih*dar):min(ih\,iw/dar):(iw-min(iw\,iw*sar))/2:(ih - min (ih\,ih/sar))/2,setsar=sar=1,scale=%s:trunc(%s/dar/2)*2`
			// ftab crop height in half, set the display aspect,crop out any black bars we may have made the scale width to requestedWidth
		}
	}

	// default
	if filter == "" {
		if requestedHeight > 0 {
			filter = "scale=trunc(%s/2)*2:trunc(%s/2)*2"
		} else {
			filter = "scale=%s:trunc(%s/a/2)*2"
		}
	}

	return fmt.Sprintf(filter, widthParam, heightParam)
}

func GetVideoColorBitDepth(state *EncodingJobInfo) int {
	if state.VideoStream != nil {
		if state.VideoStream.BitDepth != nil {
			return *state.VideoStream.BitDepth
		}

		switch state.VideoStream.PixelFormat {
		case "yuv420p", "yuvj420p", "yuv422p", "yuv444p":
			return 8
		case "yuv420p10le", "yuv422p10le", "yuv444p10le":
			return 10
		case "yuv420p12le", "yuv422p12le", "yuv444p12le":
			return 12
		}

		return 8
	}

	return 0
}

func (e *EncodingHelper) GetQsvHwVidDecoder(state *EncodingJobInfo, options *configuration.EncodingOptions, videoStream *entities.MediaStream, bitDepth int) string {
	isWindows := runtime.GOOS == "windows"
	isLinux := runtime.GOOS == "linux"

	if (!isWindows && !isLinux) || options.HardwareAccelerationType != entities.HardwareAccelerationType_QSV {
		return ""
	}

	isQsvOclSupported := e.mediaEncoder.SupportsHwaccel("qsv") && e.IsOpenclFullSupported()
	isIntelDx11OclSupported := isWindows && e.mediaEncoder.SupportsHwaccel("d3d11va") && isQsvOclSupported
	isIntelVaapiOclSupported := isLinux && e.IsVaapiSupported(state) && isQsvOclSupported
	hwSurface := (isIntelDx11OclSupported || isIntelVaapiOclSupported) && e.mediaEncoder.SupportsFilter("alphasrc")

	is8bitSwFormatsQsv := strings.EqualFold(videoStream.PixelFormat, "yuv420p") || strings.EqualFold(videoStream.PixelFormat, "yuvj420p")
	is8_10bitSwFormatsQsv := is8bitSwFormatsQsv || strings.EqualFold(videoStream.PixelFormat, "yuv420p10le")
	// TODO: add more 8/10bit and 4:4:4 formats for Qsv after finishing the ffcheck tool

	if is8bitSwFormatsQsv {
		switch strings.ToLower(videoStream.Codec) {
		case "avc", "h264":
			return e.GetHwaccelType(state, options, "h264", bitDepth, hwSurface) + e.GetHwDecoderName(options, "h264", "qsv", "h264", bitDepth)
		case "vc1":
			return e.GetHwaccelType(state, options, "vc1", bitDepth, hwSurface) + e.GetHwDecoderName(options, "vc1", "qsv", "vc1", bitDepth)
		case "vp8":
			return e.GetHwaccelType(state, options, "vp8", bitDepth, hwSurface) + e.GetHwDecoderName(options, "vp8", "qsv", "vp8", bitDepth)
		case "mpeg2video":
			return e.GetHwaccelType(state, options, "mpeg2video", bitDepth, hwSurface) + e.GetHwDecoderName(options, "mpeg2", "qsv", "mpeg2video", bitDepth)
		}
	}

	if is8_10bitSwFormatsQsv {
		switch strings.ToLower(videoStream.Codec) {
		case "hevc", "h265":
			return e.GetHwaccelType(state, options, "hevc", bitDepth, hwSurface) + e.GetHwDecoderName(options, "hevc", "qsv", "hevc", bitDepth)
		case "vp9":
			return e.GetHwaccelType(state, options, "vp9", bitDepth, hwSurface) + e.GetHwDecoderName(options, "vp9", "qsv", "vp9", bitDepth)
		case "av1":
			return e.GetHwaccelType(state, options, "av1", bitDepth, hwSurface) + e.GetHwDecoderName(options, "av1", "qsv", "av1", bitDepth)
		}
	}

	return ""
}

func (e *EncodingHelper) GetNvdecVidDecoder(state *EncodingJobInfo, options *configuration.EncodingOptions, videoStream *entities.MediaStream, bitDepth int) string {
	if (!IsWindows() && !IsLinux()) || options.HardwareAccelerationType != entities.HardwareAccelerationType_NVENC {
		return ""
	}

	hwSurface := e.IsCudaFullSupported() && e.mediaEncoder.SupportsFilter("alphasrc")
	is8bitSwFormatsNvdec := strings.EqualFold(videoStream.PixelFormat, "yuv420p") || strings.EqualFold(videoStream.PixelFormat, "yuvj420p")
	is8_10bitSwFormatsNvdec := is8bitSwFormatsNvdec || strings.EqualFold(videoStream.PixelFormat, "yuv420p10le")

	if is8bitSwFormatsNvdec {
		if strings.EqualFold(videoStream.Codec, "avc") || strings.EqualFold(videoStream.Codec, "h264") {
			return e.GetHwaccelType(state, options, "h264", bitDepth, hwSurface) + e.GetHwDecoderName(options, "h264", "cuvid", "h264", bitDepth)
		}
		if strings.EqualFold(videoStream.Codec, "mpeg2video") {
			return e.GetHwaccelType(state, options, "mpeg2video", bitDepth, hwSurface) + e.GetHwDecoderName(options, "mpeg2", "cuvid", "mpeg2video", bitDepth)
		}
		if strings.EqualFold(videoStream.Codec, "vc1") {
			return e.GetHwaccelType(state, options, "vc1", bitDepth, hwSurface) + e.GetHwDecoderName(options, "vc1", "cuvid", "vc1", bitDepth)
		}
		if strings.EqualFold(videoStream.Codec, "mpeg4") {
			return e.GetHwaccelType(state, options, "mpeg4", bitDepth, hwSurface) + e.GetHwDecoderName(options, "mpeg4", "cuvid", "mpeg4", bitDepth)
		}
		if strings.EqualFold(videoStream.Codec, "vp8") {
			return e.GetHwaccelType(state, options, "vp8", bitDepth, hwSurface) + e.GetHwDecoderName(options, "vp8", "cuvid", "vp8", bitDepth)
		}
	}

	if is8_10bitSwFormatsNvdec {
		if strings.EqualFold(videoStream.Codec, "hevc") || strings.EqualFold(videoStream.Codec, "h265") {
			return e.GetHwaccelType(state, options, "hevc", bitDepth, hwSurface) + e.GetHwDecoderName(options, "hevc", "cuvid", "hevc", bitDepth)
		}
		if strings.EqualFold(videoStream.Codec, "vp9") {
			return e.GetHwaccelType(state, options, "vp9", bitDepth, hwSurface) + e.GetHwDecoderName(options, "vp9", "cuvid", "vp9", bitDepth)
		}
		if strings.EqualFold(videoStream.Codec, "av1") {
			return e.GetHwaccelType(state, options, "av1", bitDepth, hwSurface) + e.GetHwDecoderName(options, "av1", "cuvid", "av1", bitDepth)
		}
	}

	return ""
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func IsLinux() bool {
	return runtime.GOOS == "linux"
}

func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

func (e *EncodingHelper) GetAmfVidDecoder(state *EncodingJobInfo, options *configuration.EncodingOptions, videoStream *entities.MediaStream, bitDepth int) string {
	if !IsWindows() || options.HardwareAccelerationType != entities.HardwareAccelerationType_AMF {
		return ""
	}

	hwSurface := e.mediaEncoder.SupportsHwaccel("d3d11va") && e.IsOpenclFullSupported() && e.mediaEncoder.SupportsFilter("alphasrc")
	is8bitSwFormatsAmf := strings.EqualFold(videoStream.PixelFormat, "yuv420p") || strings.EqualFold(videoStream.PixelFormat, "yuvj420p")
	is8_10bitSwFormatsAmf := is8bitSwFormatsAmf || strings.EqualFold(videoStream.PixelFormat, "yuv420p10le")

	if is8bitSwFormatsAmf {
		if strings.EqualFold(videoStream.Codec, "avc") || strings.EqualFold(videoStream.Codec, "h264") {
			return e.GetHwaccelType(state, options, "h264", bitDepth, hwSurface)
		}
		if strings.EqualFold(videoStream.Codec, "mpeg2video") {
			return e.GetHwaccelType(state, options, "mpeg2video", bitDepth, hwSurface)
		}
		if strings.EqualFold(videoStream.Codec, "vc1") {
			return e.GetHwaccelType(state, options, "vc1", bitDepth, hwSurface)
		}
	}

	if is8_10bitSwFormatsAmf {
		if strings.EqualFold(videoStream.Codec, "hevc") || strings.EqualFold(videoStream.Codec, "h265") {
			return e.GetHwaccelType(state, options, "hevc", bitDepth, hwSurface)
		}
		if strings.EqualFold(videoStream.Codec, "vp9") {
			return e.GetHwaccelType(state, options, "vp9", bitDepth, hwSurface)
		}
		if strings.EqualFold(videoStream.Codec, "av1") {
			return e.GetHwaccelType(state, options, "av1", bitDepth, hwSurface)
		}
	}

	return ""
}

func (e *EncodingHelper) GetVaapiVidDecoder(state *EncodingJobInfo, options *configuration.EncodingOptions, videoStream *entities.MediaStream, bitDepth int) string {
	isLinux := runtime.GOOS == "linux"
	if !isLinux || options.HardwareAccelerationType != entities.HardwareAccelerationType_VAAPI {
		return ""
	}

	hwSurface := e.IsVaapiSupported(state) &&
		e.IsVaapiFullSupported() &&
		e.IsOpenclFullSupported() &&
		e.mediaEncoder.SupportsFilter("alphasrc")

	is8bitSwFormatsVaapi := strings.EqualFold("yuv420p", videoStream.PixelFormat) ||
		strings.EqualFold("yuvj420p", videoStream.PixelFormat)
	is8_10bitSwFormatsVaapi := is8bitSwFormatsVaapi || strings.EqualFold("yuv420p10le", videoStream.PixelFormat)

	is8_10_12bitSwFormatsVaapi := is8_10bitSwFormatsVaapi ||
		strings.EqualFold("yuv422p", videoStream.PixelFormat) ||
		strings.EqualFold("yuv444p", videoStream.PixelFormat) ||
		strings.EqualFold("yuv422p10le", videoStream.PixelFormat) ||
		strings.EqualFold("yuv444p10le", videoStream.PixelFormat) ||
		strings.EqualFold("yuv420p12le", videoStream.PixelFormat) ||
		strings.EqualFold("yuv422p12le", videoStream.PixelFormat) ||
		strings.EqualFold("yuv444p12le", videoStream.PixelFormat)

	if is8bitSwFormatsVaapi {
		if strings.EqualFold("avc", videoStream.Codec) || strings.EqualFold("h264", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "h264", bitDepth, hwSurface)
		} else if strings.EqualFold("mpeg2video", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "mpeg2video", bitDepth, hwSurface)
		} else if strings.EqualFold("vc1", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "vc1", bitDepth, hwSurface)
		} else if strings.EqualFold("vp8", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "vp8", bitDepth, hwSurface)
		}
	}

	if is8_10bitSwFormatsVaapi {
		if strings.EqualFold("vp9", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "vp9", bitDepth, hwSurface)
		} else if strings.EqualFold("av1", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "av1", bitDepth, hwSurface)
		}
	}

	if is8_10_12bitSwFormatsVaapi {
		if strings.EqualFold("hevc", videoStream.Codec) || strings.EqualFold("h265", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "hevc", bitDepth, hwSurface)
		}
	}

	return ""
}

func (e *EncodingHelper) GetVideotoolboxVidDecoder(state *EncodingJobInfo, options *configuration.EncodingOptions, videoStream *entities.MediaStream, bitDepth int) string {
	isMacOS := runtime.GOOS == "darwin"
	if !isMacOS || options.HardwareAccelerationType != entities.HardwareAccelerationType_VideoToolbox {
		return ""
	}

	is8bitSwFormatsVt := strings.EqualFold("yuv420p", videoStream.PixelFormat) ||
		strings.EqualFold("yuvj420p", videoStream.PixelFormat)
	is8_10bitSwFormatsVt := is8bitSwFormatsVt || strings.EqualFold("yuv420p10le", videoStream.PixelFormat)

	const useHwSurface = false // Disable hw surface due to performance issues

	if is8bitSwFormatsVt {
		if strings.EqualFold("avc", videoStream.Codec) || strings.EqualFold("h264", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "h264", bitDepth, useHwSurface)
		} else if strings.EqualFold("vp8", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "vp8", bitDepth, useHwSurface)
		}
	}

	if is8_10bitSwFormatsVt {
		if strings.EqualFold("hevc", videoStream.Codec) || strings.EqualFold("h265", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "hevc", bitDepth, useHwSurface)
		} else if strings.EqualFold("vp9", videoStream.Codec) {
			return e.GetHwaccelType(state, options, "vp9", bitDepth, useHwSurface)
		}
	}

	return ""
}

func (e *EncodingHelper) IsVaapiVppTonemapAvailable(state *EncodingJobInfo, options *configuration.EncodingOptions) bool {
	if state.VideoStream == nil ||
		!options.EnableVppTonemapping ||
		GetVideoColorBitDepth(state) != 10 {
		return false
	}

	// Native VPP tonemapping may come to QSV in the future.

	return state.VideoStream.VideoRange == enums.VideoRangeHDR &&
		(state.VideoStream.VideoRangeType == enums.VideoRangeTypeHDR10 ||
			state.VideoStream.VideoRangeType == enums.VideoRangeTypeDOVIWithHDR10)
}

func (e *EncodingHelper) IsHwTonemapAvailable(state *EncodingJobInfo, options *configuration.EncodingOptions) bool {
	if state.VideoStream == nil ||
		!options.EnableTonemapping ||
		GetVideoColorBitDepth(state) != 10 {
		return false
	}

	if strings.EqualFold(state.VideoStream.Codec, "hevc") &&
		state.VideoStream.VideoRange == enums.VideoRangeHDR &&
		state.VideoStream.VideoRangeType == enums.VideoRangeTypeDOVI {
		// Only native SW decoder and HW accelerator can parse dovi rpu.
		vidDecoder := e.GetHardwareVideoDecoder(state, options)
		isSwDecoder := vidDecoder == ""
		isNvdecDecoder := strings.Contains(strings.ToLower(vidDecoder), "cuda")
		isVaapiDecoder := strings.Contains(strings.ToLower(vidDecoder), "vaapi")
		isD3d11vaDecoder := strings.Contains(strings.ToLower(vidDecoder), "d3d11va")
		isVideoToolBoxDecoder := strings.Contains(strings.ToLower(vidDecoder), "videotoolbox")
		return isSwDecoder || isNvdecDecoder || isVaapiDecoder || isD3d11vaDecoder || isVideoToolBoxDecoder
	}

	return state.VideoStream.VideoRange == enums.VideoRangeHDR &&
		(state.VideoStream.VideoRangeType == enums.VideoRangeTypeHDR10 ||
			state.VideoStream.VideoRangeType == enums.VideoRangeTypeHLG ||
			state.VideoStream.VideoRangeType == enums.VideoRangeTypeDOVIWithHDR10 ||
			state.VideoStream.VideoRangeType == enums.VideoRangeTypeDOVIWithHLG)
}

func GetHwDeinterlaceFilter(state EncodingJobInfo, options configuration.EncodingOptions, hwDeintSuffix string) string {
	doubleRateDeint := options.DeinterlaceDoubleRate && (state.VideoStream == nil || *state.VideoStream.AverageFrameRate <= 30)
	if strings.Contains(strings.ToLower(hwDeintSuffix), "cuda") {
		return fmt.Sprintf("yadif_cuda=%d:-1:0", boolToInt(doubleRateDeint))
	}

	if strings.Contains(strings.ToLower(hwDeintSuffix), "vaapi") {
		deintRate := "frame"
		if doubleRateDeint {
			deintRate = "field"
		}
		return fmt.Sprintf("deinterlace_vaapi=rate=%s", deintRate)
	}

	if strings.Contains(strings.ToLower(hwDeintSuffix), "qsv") {
		return "deinterlace_qsv=mode=2"
	}

	if strings.Contains(strings.ToLower(hwDeintSuffix), "videotoolbox") {
		return fmt.Sprintf("yadif_videotoolbox=%d:-1:0", boolToInt(doubleRateDeint))
	}

	return ""
}

/* add a parameter to the function */
/*
func (e *EncodingHelper) GetHwTonemapFilter(options configuration.EncodingOptions, hwTonemapSuffix, videoFormat string) string {
	if hwTonemapSuffix == "" {
		return ""
	}

	var args string
	algorithm := options.TonemappingAlgorithm

	if strings.EqualFold(hwTonemapSuffix, "vaapi") {
		args = "procamp_vaapi=b={1}:c={2},tonemap_vaapi=format={0}:p=bt709:t=bt709:m=bt709:extra_hw_frames=32"
		if videoFormat == "" {
			videoFormat = "nv12"
		}
		return fmt.Sprintf(args, videoFormat, options.VppTonemappingBrightness, options.VppTonemappingContrast)
	} else {
		args = "tonemap_{0}=format={1}:p=bt709:t=bt709:m=bt709:tonemap={2}:peak={3}:desat={4}"
		if strings.EqualFold(options.TonemappingMode, "max") || strings.EqualFold(options.TonemappingMode, "rgb") {
			args += ":tonemap_mode={5}"
		}
		if options.TonemappingParam != 0 {
			args += ":param={6}"
		}
		if strings.EqualFold(options.TonemappingRange, "tv") || strings.EqualFold(options.TonemappingRange, "pc") {
			args += ":range={7}"
		}

		if videoFormat == "" {
			videoFormat = "nv12"
		}
		return fmt.Sprintf(args,
			hwTonemapSuffix,
			videoFormat,
			algorithm,
			options.TonemappingPeak,
			options.TonemappingDesat,
			options.TonemappingMode,
			options.TonemappingParam,
			options.TonemappingRange)
	}
}
*/

func defaultString(str, defaultStr string) string {
	if str == "" {
		return defaultStr
	}
	return str
}

func conditional(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

func GetHwTonemapFilter(options configuration.EncodingOptions, hwTonemapSuffix, videoFormat string, forceFullRange bool) string {
	if strings.TrimSpace(hwTonemapSuffix) == "" {
		return ""
	}

	var args string
	algorithm := strings.ToLower(fmt.Sprintf("%d", options.TonemappingAlgorithm))
	mode := strings.ToLower(fmt.Sprintf("%d", options.TonemappingMode))
	rangeValue := entities.TonemappingRangePC
	if forceFullRange {
		rangeValue = entities.TonemappingRangePC
	} else {
		rangeValue = options.TonemappingRange
	}
	rangeString := strings.ToLower(fmt.Sprintf("%d", rangeValue))

	if strings.EqualFold(hwTonemapSuffix, "vaapi") {
		doVaVppProcamp := false
		procampParams := ""

		if options.VppTonemappingBrightness != 0 && options.VppTonemappingBrightness >= -100 && options.VppTonemappingBrightness <= 100 {
			procampParams += "procamp_vaapi=b=%d"
			doVaVppProcamp = true
		}

		if options.VppTonemappingContrast > 1 && options.VppTonemappingContrast <= 10 {
			if doVaVppProcamp {
				procampParams += ":c=%d"
			} else {
				procampParams += "procamp_vaapi=c=%d"
			}
			doVaVppProcamp = true
		}

		args = procampParams + "%s tonemap_vaapi=format=%s:p=bt709:t=bt709:m=bt709:extra_hw_frames=32"
		return fmt.Sprintf(args,
			options.VppTonemappingBrightness,
			options.VppTonemappingContrast,
			conditional(doVaVppProcamp, ",", ""),
			defaultString(videoFormat, "nv12"),
		)
	} else {
		args = "tonemap_%s=format=%s:p=bt709:t=bt709:m=bt709:tonemap=%s:peak=%d:desat=%d"

		useLegacyTonemapModes := true   /*e.mediaEncoder.EncoderVersion >= _minFFmpegOclCuTonemapMode && contains(_legacyTonemapModes, options.TonemappingMode) */
		useAdvancedTonemapModes := true /* e.mediaEncoder.EncoderVersion >= _minFFmpegAdvancedTonemapMode && contains(_advancedTonemapModes, options.TonemappingMode) */

		if useLegacyTonemapModes || useAdvancedTonemapModes {
			args += ":tonemap_mode=%s"
		}

		if options.TonemappingParam != 0 {
			args += ":param=%d"
		}

		if rangeValue == entities.TonemappingRangeTV || rangeValue == entities.TonemappingRangePC {
			args += ":range=%s"
		}
	}

	return fmt.Sprintf(args,
		hwTonemapSuffix,
		defaultString(videoFormat, "nv12"),
		algorithm,
		options.TonemappingPeak,
		options.TonemappingDesat,
		mode,
		options.TonemappingParam,
		rangeString,
	)
}

func (e *EncodingHelper) GetHwScaleFilter(hwScalePrefix, hwScaleSuffix, videoFormat string, swapOutputWandH bool, videoWidth, videoHeight, requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight *int) string {
	outWidth, outHeight := GetFixedOutputSize(videoWidth, videoHeight, requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight)

	isFormatFixed := videoFormat != ""
	isSizeFixed := videoWidth == nil || *outWidth != *videoWidth || videoHeight == nil || *outHeight != *videoHeight

	var swpOutW, swpOutH int
	if swapOutputWandH {
		swpOutW = *outHeight
		swpOutH = *outWidth
	} else {
		swpOutW = *outWidth
		swpOutH = *outHeight
	}

	var arg1, arg2 string
	if isSizeFixed {
		arg1 = fmt.Sprintf("=w=%d:h=%d", swpOutW, swpOutH)
	}
	if isFormatFixed {
		arg2 = "format=" + videoFormat
		if isSizeFixed {
			arg2 = ":" + arg2
		} else {
			arg2 = "=" + arg2
		}
	}

	if hwScaleSuffix != "" && (isSizeFixed || isFormatFixed) {
		return fmt.Sprintf("%s_%s%s%s", defaultString(hwScalePrefix, "scale"), hwScaleSuffix, arg1, arg2)
	}

	return ""
}

func GetAlphaSrcFilter(state EncodingJobInfo, videoWidth, videoHeight, requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight *int, framerate *float64) string {
	reqTicks := int64(0)
	if state.BaseRequest.StartTimeTicks != nil {
		reqTicks = *state.BaseRequest.StartTimeTicks
	}
	startTime := fmt.Sprintf("hh\\:mm\\:ss\\.fff", time.Duration(reqTicks)*time.Nanosecond)

	outWidth, outHeight := GetFixedOutputSize(videoWidth, videoHeight, requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight)

	if outWidth != nil && outHeight != nil {
		var frameRate float64
		if framerate != nil {
			frameRate = *framerate
		} else {
			frameRate = 25
		}
		return fmt.Sprintf("alphasrc=s=%dx%d:r=%.2f:start='%s'", *outWidth, *outHeight, frameRate, startTime)
	}

	return ""
}

func (e *EncodingHelper) GetRkmppVidDecoder(state *EncodingJobInfo, options *configuration.EncodingOptions, videoStream *entities.MediaStream, bitDepth int) string {
	isLinux := runtime.GOOS == "linux"

	if !isLinux || options.HardwareAccelerationType != entities.HardwareAccelerationType_RKMPP {
		return ""
	}

	var inW, inH, reqW, reqH, reqMaxW, reqMaxH *int
	if state.VideoStream != nil {
		inW = state.VideoStream.Width
		inH = state.VideoStream.Height
	}
	reqW = state.BaseRequest.Width
	reqH = state.BaseRequest.Height
	reqMaxW = state.BaseRequest.MaxWidth
	reqMaxH = state.BaseRequest.MaxHeight

	// rkrga RGA2e supports range from 1/16 to 16
	if !IsScaleRatioSupported(*inW, *inH, *reqW, *reqH, *reqMaxW, *reqMaxH, 16.0) {
		return ""
	}

	isRkmppOclSupported := e.IsRkmppFullSupported() && e.IsOpenclFullSupported()
	hwSurface := isRkmppOclSupported && e.mediaEncoder.SupportsFilter("alphasrc")

	// rkrga RGA3 supports range from 1/8 to 8
	isAfbcSupported := hwSurface && IsScaleRatioSupported(*inW, *inH, *reqW, *reqH, *reqMaxW, *reqMaxH, 8.0)

	// TODO: add more 8/10bit and 4:2:2 formats for Rkmpp after finishing the ffcheck tool
	is8bitSwFormatsRkmpp := strings.EqualFold(videoStream.PixelFormat, "yuv420p") || strings.EqualFold(videoStream.PixelFormat, "yuvj420p")
	is10bitSwFormatsRkmpp := strings.EqualFold(videoStream.PixelFormat, "yuv420p10le")
	is8_10bitSwFormatsRkmpp := is8bitSwFormatsRkmpp || is10bitSwFormatsRkmpp

	// nv15 and nv20 are bit-stream only formats
	if is10bitSwFormatsRkmpp && !hwSurface {
		return ""
	}

	if is8bitSwFormatsRkmpp {
		if strings.EqualFold(videoStream.Codec, "mpeg1video") {
			return e.GetHwaccelType(state, options, "mpeg1video", bitDepth, hwSurface)
		}
		if strings.EqualFold(videoStream.Codec, "mpeg2video") {
			return e.GetHwaccelType(state, options, "mpeg2video", bitDepth, hwSurface)
		}
		if strings.EqualFold(videoStream.Codec, "mpeg4") {
			return e.GetHwaccelType(state, options, "mpeg4", bitDepth, hwSurface)
		}
		if strings.EqualFold(videoStream.Codec, "vp8") {
			return e.GetHwaccelType(state, options, "vp8", bitDepth, hwSurface)
		}
	}

	if is8_10bitSwFormatsRkmpp {
		if strings.EqualFold(videoStream.Codec, "avc") || strings.EqualFold(videoStream.Codec, "h264") {
			accelType := e.GetHwaccelType(state, options, "h264", bitDepth, hwSurface)
			if accelType != "" && isAfbcSupported {
				return accelType + " -afbc rga"
			}
			return accelType
		}
		if strings.EqualFold(videoStream.Codec, "hevc") || strings.EqualFold(videoStream.Codec, "h265") {
			accelType := e.GetHwaccelType(state, options, "hevc", bitDepth, hwSurface)
			if accelType != "" && isAfbcSupported {
				return accelType + " -afbc rga"
			}
			return accelType
		}
		if strings.EqualFold(videoStream.Codec, "vp9") {
			accelType := e.GetHwaccelType(state, options, "vp9", bitDepth, hwSurface)
			if accelType != "" && isAfbcSupported {
				return accelType + " -afbc rga"
			}
			return accelType
		}
		if strings.EqualFold(videoStream.Codec, "av1") {
			return e.GetHwaccelType(state, options, "av1", bitDepth, hwSurface)
		}
	}

	return ""
}

func (e *EncodingHelper) GetHwaccelType(state *EncodingJobInfo, options *configuration.EncodingOptions, videoCodec string, bitDepth int, outputHwSurface bool) string {

	isWindows := runtime.GOOS == "windows"
	isLinux := runtime.GOOS == "linux"
	isMacOS := runtime.GOOS == "darwin"
	isD3d11Supported := isWindows && e.mediaEncoder.SupportsHwaccel("d3d11va")
	isVaapiSupported := isLinux && e.IsVaapiSupported(state)
	isCudaSupported := (isLinux || isWindows) && e.IsCudaFullSupported()
	isQsvSupported := (isLinux || isWindows) && e.mediaEncoder.SupportsHwaccel("qsv")
	isVideotoolboxSupported := isMacOS && e.mediaEncoder.SupportsHwaccel("videotoolbox")
	isRkmppSupported := isLinux && e.IsRkmppFullSupported()
	klog.Infof("HardwareDecodingCodecs: %+v %+v\n", options.HardwareDecodingCodecs, strings.ToLower(videoCodec))
	isCodecAvailable := mycontains(options.HardwareDecodingCodecs, strings.ToLower(videoCodec))

	ffmpegVersion := e.mediaEncoder.EncoderVersion()

	// Set the av1 codec explicitly to trigger hw accelerator, otherwise libdav1d will be used.
	isAv1 := version.Compare(*ffmpegVersion, minFFmpegImplictHwaccel) < 0 && strings.EqualFold(videoCodec, "av1")

	// Allow profile mismatch if decoding H.264 baseline with d3d11va and vaapi hwaccels.
	profileMismatch := strings.EqualFold(videoCodec, "h264") && strings.EqualFold(state.VideoStream.Profile, "baseline")

	// Disable the extra internal copy in nvdec. We already handle it in filter chain.
	nvdecNoInternalCopy := version.Compare(*ffmpegVersion, minFFmpegHwaUnsafeOutput) >= 0

	// Disable the extra internal copy in nvdec. We already handle it in filter chain.
	rotation := 0
	if state.VideoStream != nil && state.VideoStream.Rotation != nil {
		rotation = *state.VideoStream.Rotation
	}

	stripRotationData := rotation != 0 && version.Compare(*ffmpegVersion, minFFmpegDisplayRotationOption) >= 0

	stripRotationDataArgs := ""
	if stripRotationData {
		stripRotationDataArgs = " -display_rotation 0"
	}

	// VideoToolbox decoders have built-in SW fallback
	if isCodecAvailable && options.HardwareAccelerationType != entities.HardwareAccelerationType_VideoToolbox {
		if strings.EqualFold(videoCodec, "hevc") && mycontains(options.HardwareDecodingCodecs, "hevc") {

			if e.IsVideoStreamHevcRext(*state) {
				if bitDepth <= 10 && !options.EnableDecodingColorDepth10HevcRext {
					return ""
				}
				if bitDepth == 12 && !options.EnableDecodingColorDepth12HevcRext {
					return ""
				}

				if options.HardwareAccelerationType == entities.HardwareAccelerationType_VAAPI && !e.mediaEncoder.IsVaapiDeviceInteliHD() {
					return ""
				}
			} else if bitDepth == 10 && !options.EnableDecodingColorDepth10Hevc {
				return ""
			}
		}

		if strings.EqualFold(videoCodec, "vp9") && mycontains(options.HardwareDecodingCodecs, "vp9") && bitDepth == 10 && !options.EnableDecodingColorDepth10Vp9 {
			return ""
		}
	}

	// Intel qsv/d3d11va/vaapi
	if options.HardwareAccelerationType == entities.HardwareAccelerationType_QSV {
		if options.PreferSystemNativeHwDecoder {
			if isVaapiSupported && isCodecAvailable {
				tmp := " -hwaccel vaapi"
				if outputHwSurface {
					tmp += " -hwaccel_output_format vaapi -noautorotate" + stripRotationDataArgs
				}
				if profileMismatch {
					tmp += " -hwaccel_flags +allow_profile_mismatch"
				}
				if isAv1 {
					tmp += " -c:v av1"
				}
				return tmp
			}
			if isD3d11Supported && isCodecAvailable {
				tmp := " -hwaccel d3d11va"
				if outputHwSurface {
					tmp += " -hwaccel_output_format d3d11 -noautorotate" + stripRotationDataArgs
				}
				if profileMismatch {
					tmp += " -hwaccel_flags +allow_profile_mismatch"
				}
				tmp += " -threads 2"
				if isAv1 {
					tmp += " -c:v av1"
				}
				return tmp
			}
		} else {
			if isQsvSupported && isCodecAvailable {
				tmp := " -hwaccel qsv"
				if outputHwSurface {
					tmp += " -hwaccel_output_format qsv -noautorotate" + stripRotationDataArgs
				}
				return tmp
			}
		}
	}

	// Nvidia cuda
	if options.HardwareAccelerationType == entities.HardwareAccelerationType_NVENC {
		if isCudaSupported && isCodecAvailable {
			if options.EnableEnhancedNvdecDecoder {
				// set -threads 1 to nvdec decoder explicitly since it doesn't implement threading support.
				tmp := " -hwaccel cuda"
				if outputHwSurface {
					tmp += " -hwaccel_output_format cuda -noautorotate" + stripRotationDataArgs
				}
				if nvdecNoInternalCopy {
					tmp += " -hwaccel_flags +unsafe_output"
				}
				tmp += " -threads 1"
				if isAv1 {
					tmp += " -c:v av1"
				}
				return tmp

			}
			// cuvid decoder doesn't have threading issue.
			tmp := " -hwaccel cuda"
			if outputHwSurface {
				tmp += " -hwaccel_output_format cuda -noautorotate" + stripRotationDataArgs
			}
			return tmp
		}
	}

	// Amd d3d11va
	if options.HardwareAccelerationType == entities.HardwareAccelerationType_AMF {
		if isD3d11Supported && isCodecAvailable {
			tmp := " -hwaccel d3d11va"
			if outputHwSurface {
				tmp += " -hwaccel_output_format d3d11 -noautorotate" + stripRotationDataArgs
			}
			if profileMismatch {
				tmp += " -hwaccel_flags +allow_profile_mismatch"
			}
			if isAv1 {
				tmp += " -c:v av1"
			}
			return tmp
		}
	}

	// Vaapi
	if options.HardwareAccelerationType == entities.HardwareAccelerationType_VAAPI && isVaapiSupported && isCodecAvailable {
		klog.Infoln("VAAPIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIII")
		tmp := " -hwaccel vaapi"
		if outputHwSurface {
			tmp += " -hwaccel_output_format vaapi -noautorotate" + stripRotationDataArgs
		}
		if profileMismatch {
			tmp += " -hwaccel_flags +allow_profile_mismatch"
		}
		if isAv1 {
			tmp += " -c:v av1"
		}
		return tmp
	}

	// Apple videotoolbox
	if options.HardwareAccelerationType == entities.HardwareAccelerationType_VideoToolbox && isVideotoolboxSupported && isCodecAvailable {
		tmp := " -hwaccel videotoolbox"
		if outputHwSurface {
			tmp += " -hwaccel_output_format videotoolbox_vld"
		}
		tmp += " -noautorotate" + stripRotationDataArgs
		return tmp
	}

	// Rockchip rkmpp
	if options.HardwareAccelerationType == entities.HardwareAccelerationType_RKMPP && isRkmppSupported && isCodecAvailable {
		tmp := " -hwaccel rkmpp"
		if outputHwSurface {
			tmp += " -hwaccel_output_format drm_prime -noautorotate" + stripRotationDataArgs
		}
		return tmp
	}

	return ""
}

func mycontains(slice []string, value string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

func (e *EncodingHelper) GetHwDecoderName(options *configuration.EncodingOptions, decoderPrefix, decoderSuffix, videoCodec string, bitDepth int) string {
	if decoderPrefix == "" || decoderSuffix == "" {
		return ""
	}

	decoderName := decoderPrefix + "_" + decoderSuffix

	isCodecAvailable := e.mediaEncoder.SupportsDecoder(decoderName) && ContainsCodec(options.HardwareDecodingCodecs, videoCodec)
	if bitDepth == 10 && isCodecAvailable && options.HardwareAccelerationType != entities.HardwareAccelerationType_VideoToolbox {
		if strings.EqualFold(videoCodec, "hevc") && ContainsCodec(options.HardwareDecodingCodecs, "hevc") && !options.EnableDecodingColorDepth10Hevc {
			return ""
		}

		if strings.EqualFold(videoCodec, "vp9") && ContainsCodec(options.HardwareDecodingCodecs, "vp9") && !options.EnableDecodingColorDepth10Vp9 {
			return ""
		}
	}

	if strings.EqualFold(decoderSuffix, "cuvid") && options.EnableEnhancedNvdecDecoder {
		return ""
	}

	if strings.EqualFold(decoderSuffix, "qsv") && options.PreferSystemNativeHwDecoder {
		return ""
	}

	if strings.EqualFold(decoderSuffix, "rkmpp") {
		return ""
	}

	if isCodecAvailable {
		return " -c:v " + decoderName
	}

	return ""
}

func ContainsCodec(codecs []string, codec string) bool {
	for _, c := range codecs {
		if strings.EqualFold(c, codec) {
			return true
		}
	}
	return false
}

func IsScaleRatioSupported(
	videoWidth, videoHeight, requestedWidth, requestedHeight, requestedMaxWidth, requestedMaxHeight int,
	maxScaleRatio float64,
) bool {
	outWidth, outHeight := GetFixedOutputSize(
		&videoWidth, &videoHeight, &requestedWidth, &requestedHeight, &requestedMaxWidth, &requestedMaxHeight,
	)

	if videoWidth == 0 || videoHeight == 0 || *outWidth == 0 || *outHeight == 0 || maxScaleRatio < 1.0 {
		return false
	}

	minScaleRatio := 1.0 / maxScaleRatio
	scaleRatioW := float64(*outWidth) / float64(videoWidth)
	scaleRatioH := float64(*outHeight) / float64(videoHeight)

	if scaleRatioW < minScaleRatio || scaleRatioW > maxScaleRatio || scaleRatioH < minScaleRatio || scaleRatioH > maxScaleRatio {
		return false
	}

	return true
}

func (e *EncodingHelper) GetInputArgument(state *EncodingJobInfo, options *configuration.EncodingOptions, segmentContainer *string) string {
	var arg strings.Builder
	inputVidHwaccelArgs := e.GetInputVideoHwaccelArgs(state, options)
	if inputVidHwaccelArgs != "" {
		arg.WriteString(inputVidHwaccelArgs)
	}

	canvasArgs := e.GetGraphicalSubCanvasSize(*state)
	if canvasArgs != "" {
		arg.WriteString(canvasArgs)
	}

	if state.MediaSource.VideoType != nil && (*state.MediaSource.VideoType == entities.Dvd || *state.MediaSource.VideoType == entities.BluRay) {
		tmpConcatPath := filepath.Join(e.configurationManager.GetTranscodePath(), state.MediaSource.ID+".concat")
		e.mediaEncoder.GenerateConcatConfig(*state.MediaSource, tmpConcatPath)
		arg.WriteString(" -f concat -safe 0 -i \"" + tmpConcatPath + "\" ")
	} else {
		/*
			if state.InputProtocol == mediaprotocol.Http {
				arg.WriteString(" -headers \"" + "remote-accesstoken: " + state.RemoteAccessToken + "\"")
			}
		*/
		arg.WriteString(" -i " + e.mediaEncoder.GetInputPathArgument(*state))
		klog.Infoln("state: ", e.mediaEncoder.GetInputPathArgument(*state))
		//		arg.WriteString(" -i " + e.mediaEncoder.GetInputPathArgument(state.MediaPath, *state.MediaSource))
	}

	if state.SubtitleStream != nil &&
		state.SubtitleDeliveryMethod == dlna.Encode &&
		!state.SubtitleStream.IsTextSubtitleStream() &&
		state.SubtitleStream.IsExternal {
		subtitlePath := state.SubtitleStream.Path
		subtitleExtension := filepath.Ext(subtitlePath)

		if strings.EqualFold(subtitleExtension, ".sub") || strings.EqualFold(subtitleExtension, ".sup") {
			idxFile := strings.TrimSuffix(subtitlePath, filepath.Ext(subtitlePath)) + ".idx"
			if _, err := os.Stat(idxFile); err == nil {
				subtitlePath = idxFile
			}
		}

		seekSubParam := e.GetFastSeekCommandLineParameter(state, options, segmentContainer)
		if seekSubParam != "" {
			arg.WriteString(" " + seekSubParam)
		}

		if canvasArgs != "" {
			arg.WriteString(canvasArgs)
		}

		arg.WriteString(" -i file:\"" + subtitlePath + "\"")
	}

	if state.AudioStream != nil && state.AudioStream.IsExternal {
		seekAudioParam := e.GetFastSeekCommandLineParameter(state, options, segmentContainer)
		if seekAudioParam != "" {
			arg.WriteString(" " + seekAudioParam)
		}

		arg.WriteString(" -i \"" + state.AudioStream.Path + "\"")
	}

	isSwDecoder := e.GetHardwareVideoDecoder(state, options) == ""
	if !isSwDecoder {
		arg.WriteString(" -noautoscale")
	}

	return arg.String()
}

func (e *EncodingHelper) GetGraphicalSubCanvasSize(state EncodingJobInfo) string {
	// DVBSUB uses the fixed canvas size 720x576
	if state.SubtitleStream != nil &&
		state.SubtitleDeliveryMethod == dlna.Encode &&
		!state.SubtitleStream.IsTextSubtitleStream() &&
		state.SubtitleStream.Codec != "DVBSUB" {
		subtitleWidth := state.SubtitleStream.Width
		subtitleHeight := state.SubtitleStream.Height

		if subtitleWidth != nil && subtitleHeight != nil &&
			*subtitleWidth > 0 && *subtitleHeight > 0 {
			return fmt.Sprintf(" -canvas_size %dx%d", *subtitleWidth, *subtitleHeight)
		}
	}

	return ""
}

func (e *EncodingHelper) GetInputVideoHwaccelArgs(state *EncodingJobInfo, options *configuration.EncodingOptions) string {
	if !state.IsVideoRequest {
		return ""
	}

	vidEncoder := e.GetVideoEncoder(state, options)
	if IsCopyCodec(vidEncoder) {
		return ""
	}

	var args strings.Builder
	isWindows := runtime.GOOS == "windows"
	isLinux := runtime.GOOS == "linux"
	isMacOS := runtime.GOOS == "darwin"
	optHwaccelType := options.HardwareAccelerationType
	vidDecoder := e.GetHardwareVideoDecoder(state, options)
	isHwTonemapAvailable := e.IsHwTonemapAvailable(state, options)

	if optHwaccelType == entities.HardwareAccelerationType_VAAPI {
		if !isLinux || !e.mediaEncoder.SupportsHwaccel("vaapi") {
			return ""
		}

		isVaapiDecoder := strings.Contains(vidDecoder, "vaapi")
		isVaapiEncoder := strings.Contains(vidEncoder, "vaapi")
		if !isVaapiDecoder && !isVaapiEncoder {
			return ""
		}

		if e.mediaEncoder.IsVaapiDeviceInteliHD() {
			args.WriteString(e.GetVaapiDeviceArgs(options.VaapiDevice, "iHD", "", "", "", VaapiAlias))
		} else if e.mediaEncoder.IsVaapiDeviceInteli965() {
			// Only override i965 since it has lower priority than iHD in libva lookup.
			os.Setenv("LIBVA_DRIVER_NAME", "i965")
			os.Setenv("LIBVA_DRIVER_NAME_JELLYFIN", "i965")
			args.WriteString(e.GetVaapiDeviceArgs(options.VaapiDevice, "i965", "", "", "", VaapiAlias))
		}

		var filterDevArgs string
		doOclTonemap := isHwTonemapAvailable && e.IsOpenclFullSupported()

		if e.mediaEncoder.IsVaapiDeviceInteliHD() || e.mediaEncoder.IsVaapiDeviceInteli965() {
			if doOclTonemap && !isVaapiDecoder {
				args.WriteString(e.GetOpenclDeviceArgs(0, "", VaapiAlias, OpenclAlias))
				filterDevArgs = e.GetFilterHwDeviceArgs(OpenclAlias)
			}
		} else if e.mediaEncoder.IsVaapiDeviceAmd() {
			// Disable AMD EFC feature since it's still unstable in upstream Mesa.
			os.Setenv("AMD_DEBUG", "noefc")

			if e.IsVulkanFullSupported() &&
				e.mediaEncoder.IsVaapiDeviceSupportVulkanDrmInterop() { /*&&
				runtime.Version().Version >= minKernelVersionAmdVkFmtModifier {*/
				args.WriteString(e.GetDrmDeviceArgs(options.VaapiDevice, DrmAlias))
				args.WriteString(e.GetVaapiDeviceArgs("", "", "", "", DrmAlias, VaapiAlias))
				args.WriteString(e.GetVulkanDeviceArgs(0, "", DrmAlias, VulkanAlias))

				// libplacebo wants an explicitly set vulkan filter device.
				filterDevArgs = e.GetFilterHwDeviceArgs(VulkanAlias)
			} else {
				args.WriteString(e.GetVaapiDeviceArgs(options.VaapiDevice, "", "", "", "", VaapiAlias))
				filterDevArgs = e.GetFilterHwDeviceArgs(VaapiAlias)

				if doOclTonemap {
					// ROCm/ROCr OpenCL runtime
					args.WriteString(e.GetOpenclDeviceArgs(0, "Advanced Micro Devices", "", OpenclAlias))
					filterDevArgs = e.GetFilterHwDeviceArgs(OpenclAlias)
				}
			}
		} else if doOclTonemap {
			args.WriteString(e.GetOpenclDeviceArgs(0, "", "", OpenclAlias))
			filterDevArgs = e.GetFilterHwDeviceArgs(OpenclAlias)
		}

		args.WriteString(filterDevArgs)
	} else if optHwaccelType == entities.HardwareAccelerationType_QSV {
		if (!isLinux && !isWindows) || !e.mediaEncoder.SupportsHwaccel("qsv") {
			return ""
		}

		isD3d11vaDecoder := strings.Contains(vidDecoder, "d3d11va")
		isVaapiDecoder := strings.Contains(vidDecoder, "vaapi")
		isQsvDecoder := strings.Contains(vidDecoder, "qsv")
		isQsvEncoder := strings.Contains(vidEncoder, "qsv")
		isHwDecoder := isQsvDecoder || isVaapiDecoder || isD3d11vaDecoder
		if !isHwDecoder && !isQsvEncoder {
			return ""
		}

		args.WriteString(e.GetQsvDeviceArgs(QsvAlias))
		filterDevArgs := e.GetFilterHwDeviceArgs(QsvAlias)
		// child device used by qsv.
		if e.mediaEncoder.SupportsHwaccel("vaapi") || e.mediaEncoder.SupportsHwaccel("d3d11va") {
			if isHwTonemapAvailable && e.IsOpenclFullSupported() {
				srcAlias := VaapiAlias
				if !isLinux {
					srcAlias = D3d11vaAlias
				}
				args.WriteString(e.GetOpenclDeviceArgs(0, "", srcAlias, OpenclAlias))
				if !isHwDecoder {
					filterDevArgs = e.GetFilterHwDeviceArgs(OpenclAlias)
				}
			}
		}

		args.WriteString(filterDevArgs)
	} else if optHwaccelType == entities.HardwareAccelerationType_NVENC {

		if (!isLinux && !isWindows) || !e.IsCudaFullSupported() {
			return ""
		}

		isCuvidDecoder := strings.Contains(vidDecoder, "cuvid")
		isNvdecDecoder := strings.Contains(vidDecoder, "cuda")
		isNvencEncoder := strings.Contains(vidEncoder, "nvenc")
		isHwDecoder := isNvdecDecoder || isCuvidDecoder
		if !isHwDecoder && !isNvencEncoder {
			return ""
		}

		args.WriteString(e.GetCudaDeviceArgs(0, CudaAlias))
		args.WriteString(e.GetFilterHwDeviceArgs(CudaAlias))
	} else if optHwaccelType == entities.HardwareAccelerationType_AMF {
		if !isWindows || !e.mediaEncoder.SupportsHwaccel("d3d11va") {
			return ""
		}

		isD3d11vaDecoder := strings.Contains(vidDecoder, "d3d11va")
		isAmfEncoder := strings.Contains(vidEncoder, "amf")
		if !isD3d11vaDecoder && !isAmfEncoder {
			return ""
		}

		// no dxva video processor hw filter.
		args.WriteString(e.GetD3d11vaDeviceArgs(0, "0x1002", D3d11vaAlias))
		filterDevArgs := ""
		if e.IsOpenclFullSupported() {
			args.WriteString(e.GetOpenclDeviceArgs(0, "", D3d11vaAlias, OpenclAlias))
			filterDevArgs = e.GetFilterHwDeviceArgs(OpenclAlias)
		}

		args.WriteString(filterDevArgs)
	} else if optHwaccelType == entities.HardwareAccelerationType_VideoToolbox {
		if !isMacOS || !e.mediaEncoder.SupportsHwaccel("videotoolbox") {
			return ""
		}

		isVideotoolboxDecoder := strings.Contains(vidDecoder, "videotoolbox")
		isVideotoolboxEncoder := strings.Contains(vidEncoder, "videotoolbox")
		if !isVideotoolboxDecoder && !isVideotoolboxEncoder {
			return ""
		}

		// videotoolbox hw filter does not require device selection
		args.WriteString(e.GetVideoToolboxDeviceArgs(VideotoolboxAlias))
	} else if optHwaccelType == entities.HardwareAccelerationType_RKMPP {
		if !isLinux || !e.mediaEncoder.SupportsHwaccel("rkmpp") {
			return ""
		}

		isRkmppDecoder := strings.Contains(vidDecoder, "rkmpp")
		isRkmppEncoder := strings.Contains(vidEncoder, "rkmpp")
		if !isRkmppDecoder && !isRkmppEncoder {
			return ""
		}

		args.WriteString(e.GetRkmppDeviceArgs(RkmppAlias))

		var filterDevArgs string
		doOclTonemap := isHwTonemapAvailable && e.IsOpenclFullSupported()

		if doOclTonemap && !isRkmppDecoder {
			args.WriteString(e.GetOpenclDeviceArgs(0, "", RkmppAlias, OpenclAlias))
			filterDevArgs = e.GetFilterHwDeviceArgs(OpenclAlias)
		}

		args.WriteString(filterDevArgs)
	}

	if vidDecoder != "" {
		args.WriteString(vidDecoder)
	}

	// hw transpose filters should be added manually.
	//	args.WriteString(" -noautorotate")

	return args.String()
}

func (e *EncodingHelper) GetVaapiDeviceArgs(renderNodePath, driver, kernelDriver, vendorId, srcDeviceAlias, alias string) string {
	if alias == "" {
		alias = VaapiAlias
	}

	// Check if vendorId is non-empty and encoder version meets the minimum requirement
	haveVendorId := vendorId != "" && version.Compare(*e.mediaEncoder.EncoderVersion(), minFFmpegVaapiDeviceVendorId) >= 0

	// Priority: renderNodePath > vendorId > kernelDriver
	driverOpts := ""
	if _, err := os.Stat(renderNodePath); err == nil {
		driverOpts = renderNodePath
	} else if haveVendorId {
		driverOpts = ",vendor_id=" + vendorId
	} else if kernelDriver != "" {
		driverOpts = ",kernel_driver=" + kernelDriver
	}

	// 'driver' behaves similarly to env LIBVA_DRIVER_NAME
	if driver != "" {
		driverOpts += ",driver=" + driver
	}

	var options string
	if srcDeviceAlias == "" {
		if driverOpts == "" {
			options = ""
		} else {
			options = ":" + driverOpts
		}
	} else {
		options = "@" + srcDeviceAlias
	}

	return fmt.Sprintf(" -init_hw_device vaapi=%s%s", alias, options)
}

func (e *EncodingHelper) GetOpenclDeviceArgs(deviceIndex int, deviceVendorName, srcDeviceAlias, alias string) string {
	if alias == "" {
		alias = OpenclAlias
	}

	if deviceIndex < 0 {
		deviceIndex = 0
	}

	var vendorOpts string
	if deviceVendorName == "" {
		vendorOpts = ":0.0"
	} else {
		vendorOpts = fmt.Sprintf(":.%d,device_vendor=\"%s\"", deviceIndex, deviceVendorName)
	}

	var options string
	if srcDeviceAlias == "" {
		options = vendorOpts
	} else {
		options = "@" + srcDeviceAlias
	}

	return fmt.Sprintf(" -init_hw_device opencl=%s%s", alias, options)
}

func (e *EncodingHelper) GetFilterHwDeviceArgs(alias string) string {
	if alias == "" {
		return ""
	} else {
		return " -filter_hw_device " + alias
	}
}

func (e *EncodingHelper) GetDrmDeviceArgs(renderNodePath, alias string) string {
	if alias == "" {
		alias = DrmAlias
	}

	if renderNodePath == "" {
		renderNodePath = "/dev/dri/renderD128"
	}

	return fmt.Sprintf(" -init_hw_device drm=%s:%s", alias, renderNodePath)
}

func (e *EncodingHelper) GetVulkanDeviceArgs(deviceIndex int, deviceName, srcDeviceAlias, alias string) string {
	if alias == "" {
		alias = VulkanAlias
	}

	if deviceIndex < 0 {
		deviceIndex = 0
	}

	var vendorOpts string
	if deviceName == "" {
		vendorOpts = fmt.Sprintf(":%d", deviceIndex)
	} else {
		vendorOpts = fmt.Sprintf(":\"" + deviceName + "\"")
	}

	var options string
	if srcDeviceAlias == "" {
		options = vendorOpts
	} else {
		options = "@" + srcDeviceAlias
	}

	return fmt.Sprintf(" -init_hw_device vulkan=%s%s", alias, options)
}

func (e *EncodingHelper) GetQsvDeviceArgs(alias string) string {
	qsvAlias := QsvAlias
	if alias != "" {
		qsvAlias = alias
	}

	arg := " -init_hw_device qsv=" + qsvAlias

	if runtime.GOOS == "linux" {
		// derive qsv from vaapi device
		return e.GetVaapiDeviceArgs("", "iHD", "i915", "0x8086", "", VaapiAlias) + arg + "@" + VaapiAlias
	}

	if runtime.GOOS == "windows" {
		// derive qsv from d3d11va device
		return e.GetD3d11vaDeviceArgs(0, "0x8086", D3d11vaAlias) + arg + "@" + D3d11vaAlias
	}

	return ""
}

func (e *EncodingHelper) GetCudaDeviceArgs(deviceIndex int, alias string) string {
	if alias == "" {
		alias = CudaAlias
	}

	if deviceIndex < 0 {
		deviceIndex = 0
	}

	return fmt.Sprintf(" -init_hw_device cuda=%s:%d", alias, deviceIndex)
}

func (e *EncodingHelper) GetD3d11vaDeviceArgs(deviceIndex int, deviceVendorId string, alias string) string {
	if alias == "" {
		alias = D3d11vaAlias
	}

	if deviceIndex < 0 {
		deviceIndex = 0
	}

	var options string
	if deviceVendorId == "" {
		options = fmt.Sprintf("%d", deviceIndex)
	} else {
		options = fmt.Sprintf(",vendor=%s", deviceVendorId)
	}

	return fmt.Sprintf(" -init_hw_device d3d11va=%s:%s", alias, options)
}

func (e *EncodingHelper) GetVideoToolboxDeviceArgs(alias string) string {
	if alias == "" {
		alias = VideotoolboxAlias
	}

	// device selection in vt is not supported.
	return " -init_hw_device videotoolbox=" + alias
}

func (e *EncodingHelper) GetRkmppDeviceArgs(alias string) string {
	if alias == "" {
		alias = RkmppAlias
	}

	// device selection in rk is not supported.
	return " -init_hw_device rkmpp=" + alias
}

func (e *EncodingHelper) GetAudioVbrModeParam(encoder string, bitratePerChannel int) string {
	switch strings.ToLower(encoder) {
	case "libfdk_aac":
		switch {
		case bitratePerChannel < 32000:
			return " -vbr:a 1"
		case bitratePerChannel < 48000:
			return " -vbr:a 2"
		case bitratePerChannel < 64000:
			return " -vbr:a 3"
		case bitratePerChannel < 96000:
			return " -vbr:a 4"
		default:
			return " -vbr:a 5"
		}
	case "libmp3lame":
		switch {
		case bitratePerChannel < 48000:
			return " -qscale:a 8"
		case bitratePerChannel < 64000:
			return " -qscale:a 6"
		case bitratePerChannel < 88000:
			return " -qscale:a 4"
		case bitratePerChannel < 112000:
			return " -qscale:a 2"
		default:
			return " -qscale:a 0"
		}
	case "libvorbis":
		switch {
		case bitratePerChannel < 40000:
			return " -qscale:a 0"
		case bitratePerChannel < 56000:
			return " -qscale:a 2"
		case bitratePerChannel < 80000:
			return " -qscale:a 4"
		case bitratePerChannel < 112000:
			return " -qscale:a 6"
		default:
			return " -qscale:a 8"
		}
	default:
		return ""
	}
}

func (e *EncodingHelper) GetAudioBitStreamArguments(state *EncodingJobInfo, segmentContainer *string, mediaSourceContainer string) string {
	bitStreamArgs := ""
	segmentFormat := strings.TrimPrefix(GetSegmentFileExtension(segmentContainer), ".")

	// Apply aac_adtstoasc bitstream filter when media source is in mpegts.
	if strings.EqualFold(segmentFormat, "mp4") &&
		(strings.EqualFold(mediaSourceContainer, "ts") ||
			strings.EqualFold(mediaSourceContainer, "aac") ||
			strings.EqualFold(mediaSourceContainer, "hls")) {
		bitStreamArgs = e.GetBitStreamArgs(state, entities.MediaStreamTypeAudio)
		if bitStreamArgs != "" {
			bitStreamArgs = " " + bitStreamArgs
		}
	}

	return bitStreamArgs
}

func (e *EncodingHelper) GetBitStreamArgs(state *EncodingJobInfo, streamType entities.MediaStreamType) string {
	if state == nil {
		return ""
	}

	var stream *entities.MediaStream
	switch streamType {
	case entities.MediaStreamTypeAudio:
		stream = state.AudioStream
	case entities.MediaStreamTypeVideo:
		stream = state.VideoStream
	default:
		stream = state.VideoStream
	}

	// TODO: This is auto inserted into the mpegts mux so it might not be needed.
	// https://www.ffmpeg.org/ffmpeg-bitstream-filters.html#h264_005fmp4toannexb
	if IsH264(*stream) {
		return "-bsf:v h264_mp4toannexb"
	}

	if IsAAC(*stream) {
		// Convert adts header(mpegts) to asc header(mp4).
		return "-bsf:a aac_adtstoasc"
	}

	if IsH265(*stream) {
		filter := "-bsf:v hevc_mp4toannexb"

		// The following checks are not complete because the copy would be rejected
		// if the encoder cannot remove required metadata.
		// And if bsf is used, we must already be using copy codec.
		switch ShouldRemoveDynamicHdrMetadata(state) {
		case DynamicHdrMetadataRemovalPlanNone:
			break
		case DynamicHdrMetadataRemovalPlanRemoveDovi:
			if e.mediaEncoder.SupportsBitStreamFilterWithOption(BitStreamFilterOptionTypeHevcMetadataRemoveDovi) {
				filter += ",hevc_metadata=remove_dovi=1"
			} else {
				filter += ",dovi_rpu=strip=1"
			}
		case DynamicHdrMetadataRemovalPlanRemoveHdr10Plus:
			filter += ",hevc_metadata=remove_hdr10plus=1"
		}

		return filter
	}

	if IsAv1(*stream) {
		switch ShouldRemoveDynamicHdrMetadata(state) {
		case DynamicHdrMetadataRemovalPlanNone:
			return ""
		case DynamicHdrMetadataRemovalPlanRemoveDovi:
			if e.mediaEncoder.SupportsBitStreamFilterWithOption(BitStreamFilterOptionTypeAv1MetadataRemoveDovi) {
				return "-bsf:v av1_metadata=remove_dovi=1"
			}
			return "-bsf:v dovi_rpu=strip=1"
		case DynamicHdrMetadataRemovalPlanRemoveHdr10Plus:
			return "-bsf:v av1_metadata=remove_hdr10plus=1"
		}
	}

	return ""
}

func _GetBitStreamArgs(stream entities.MediaStream) string {
	// TODO This is auto inserted into the mpegts mux so it might not be needed.
	// https://www.ffmpeg.org/ffmpeg-bitstream-filters.html#h264_005fmp4toannexb
	if IsH264(stream) {
		return "-bsf:v h264_mp4toannexb"
	}

	if IsH265(stream) {
		return "-bsf:v hevc_mp4toannexb"
	}

	if IsAAC(stream) {
		// Convert adts header(mpegts) to asc header(mp4).
		return "-bsf:a aac_adtstoasc"
	}

	return ""
}

func IsH264(stream entities.MediaStream) bool {
	codec := stream.Codec
	if codec == "" {
		return false
	}
	return strings.Contains(strings.ToLower(codec), "264") ||
		strings.Contains(strings.ToLower(codec), "avc")
}

func IsH265(stream entities.MediaStream) bool {
	codec := stream.Codec
	if codec == "" {
		return false
	}
	return strings.Contains(strings.ToLower(codec), "265") ||
		strings.Contains(strings.ToLower(codec), "hevc")
}

func IsAv1(stream entities.MediaStream) bool {
	codec := stream.Codec
	if codec == "" {
		return false
	}
	return strings.Contains(strings.ToLower(codec), "av1")
}

func IsAAC(stream entities.MediaStream) bool {
	codec := stream.Codec
	if codec == "" {
		return false
	}
	return strings.Contains(strings.ToLower(codec), "aac")
}

func (e *EncodingHelper) GetAudioFilterParam(state *EncodingJobInfo, encodingOptions *configuration.EncodingOptions) string {
	var channels int
	if state.OutputAudioChannels != nil {
		channels = *state.OutputAudioChannels
	}

	filters := []string{}

	if channels == 2 && state.AudioStream != nil && state.AudioStream.Channels != nil && *state.AudioStream.Channels > 2 {
		h := DownMixAlgorithmsHelper{}
		klog.Infoln(encodingOptions.DownMixStereoAlgorithm, h.InferChannelLayout(*state.AudioStream))
		downMixFilterString, hasDownMixFilter := AlgorithmFilterStrings[AlgorithmFilterKey{encodingOptions.DownMixStereoAlgorithm, h.InferChannelLayout(*state.AudioStream)}]

		if hasDownMixFilter {
			filters = append(filters, downMixFilterString)
		}

		if encodingOptions.DownMixAudioBoost != 1 {
			filters = append(filters, "volume="+strconv.FormatFloat(encodingOptions.DownMixAudioBoost, 'f', -1, 64))
		}
	}

	isCopyingTimestamps := state.CopyTimestamps || state.TranscodingType != Progressive
	if state.SubtitleStream != nil && state.SubtitleStream.IsTextSubtitleStream() && ShouldEncodeSubtitle(*state) && !isCopyingTimestamps {
		var seconds float64
		if state.StartTimeTicks != nil {
			seconds = float64(*state.StartTimeTicks) / float64(time.Second)
		}
		filters = append(filters, fmt.Sprintf("asetpts=PTS-%s/TB", strconv.FormatFloat(seconds, 'f', 0, 64)))
	}

	if len(filters) > 0 {
		return " -af \"" + strings.Join(filters, ",") + "\""
	}

	return ""
}

func parseKernelVersion(versionStr string) (version.Version, error) {
	parts := strings.Split(strings.TrimSpace(versionStr), ".")
	if len(parts) < 3 {
		return version.Version{}, fmt.Errorf("invalid version format")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return version.Version{}, err
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return version.Version{}, err
	}

	patchParts := strings.Split(parts[2], "-")
	patch, err := strconv.Atoi(patchParts[0])
	if err != nil {
		return version.Version{}, err
	}

	return version.Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

func getKernelVersion() (version.Version, error) {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return version.Version{}, err
	}

	return parseKernelVersion(string(output))
}

var KernelVersion version.Version

func init() {
	if v, err := getKernelVersion(); err == nil {
		KernelVersion = v
	}
}

func (e *EncodingHelper) GetEncoderParam(preset *entities.EncoderPreset, defaultPreset entities.EncoderPreset, encodingOptions *configuration.EncodingOptions, videoEncoder string, isLibX265 bool) string {

	param := ""
	encoderPreset := defaultPreset
	if preset != nil {
		encoderPreset = *preset
	}

	if strings.EqualFold(videoEncoder, "libx264") || isLibX265 {
		if encoderPreset != entities.Auto {
			param += " -preset " + encoderPreset.String()
		} else {
			param += " -preset " + defaultPreset.String()
		}
		klog.Infof("%+v\n", encodingOptions)
		encodeCrf := encodingOptions.H264Crf
		if isLibX265 {
			encodeCrf = encodingOptions.H265Crf
		}

		if encodeCrf >= 0 && encodeCrf <= 51 {
			param += " -crf " + strconv.Itoa(encodeCrf)
		} else {
			defaultCrf := "23"
			if isLibX265 {
				defaultCrf = "28"
			}
			param += " -crf " + defaultCrf
		}
	} else if strings.EqualFold(videoEncoder, "libsvtav1") {
		// Default to use the recommended preset 10.
		// Omit presets < 5, which are too slow for on the fly encoding.
		// https://gitlab.com/AOMediaCodec/SVT-AV1/-/blob/master/Docs/Ffmpeg.md
		switch encodingOptions.EncoderPreset {
		case entities.VerySlow:
			param += " -preset 5"
		case entities.Slower:
			param += " -preset 6"
		case entities.Slow:
			param += " -preset 7"
		case entities.Medium:
			param += " -preset 8"
		case entities.Fast:
			param += " -preset 9"
		case entities.Faster:
			param += " -preset 10"
		case entities.VeryFast:
			param += " -preset 11"
		case entities.SuperFast:
			param += " -preset 12"
		case entities.UltraFast:
			param += " -preset 13"
		default:
			param += " -preset 10"
		}
	} else if strings.EqualFold(videoEncoder, "h264_vaapi") ||
		strings.EqualFold(videoEncoder, "hevc_vaapi") ||
		strings.EqualFold(videoEncoder, "av1_vaapi") {
		// -compression_level is not reliable on AMD.
		if e.mediaEncoder.IsVaapiDeviceInteliHD() {
			switch encodingOptions.EncoderPreset {
			case entities.VerySlow:
				param += " -compression_level 1"
			case entities.Slower:
				param += " -compression_level 2"
			case entities.Slow:
				param += " -compression_level 3"
			case entities.Medium:
				param += " -compression_level 4"
			case entities.Fast:
				param += " -compression_level 5"
			case entities.Faster:
				param += " -compression_level 6"
			case entities.VeryFast:
				param += " -compression_level 7"
			case entities.SuperFast:
				param += " -compression_level 7"
			case entities.UltraFast:
				param += " -compression_level 7"
			}
		}
	} else if strings.EqualFold(videoEncoder, "h264_qsv") || strings.EqualFold(videoEncoder, "hevc_qsv") || strings.EqualFold(videoEncoder, "av1_qsv") {
		validPresets := []entities.EncoderPreset{entities.VerySlow, entities.Slower, entities.Slow, entities.Medium, entities.Fast, entities.Faster, entities.VeryFast}
		if contains(validPresets, encodingOptions.EncoderPreset, true) {
			param += " -preset " + encodingOptions.EncoderPreset.String()
		} else {
			param += " -preset veryfast"
		}
		/*
			if strings.EqualFold(videoEncoder, "h264_qsv") {
				param += " -look_ahead 0"
			}
		*/
	} else if strings.EqualFold(videoEncoder, "h264_nvenc") || strings.EqualFold(videoEncoder, "hevc_nvenc") || strings.EqualFold(videoEncoder, "av1_nvenc") {
		switch encodingOptions.EncoderPreset {
		case entities.VerySlow:
			param += " -preset p7"
		case entities.Slower:
			param += " -preset p6"
		case entities.Slow:
			param += " -preset p5"
		case entities.Medium:
			param += " -preset p4"
		case entities.Fast:
			param += " -preset p3"
		case entities.Faster:
			param += " -preset p2"
			//		case entities.VeryFast, entities.SuperFast, entities.UltraFast:
			//			param += " -preset p1"
		default:
			param += " -preset p1"
		}
	} else if strings.EqualFold(videoEncoder, "h264_amf") || strings.EqualFold(videoEncoder, "hevc_amf") || strings.EqualFold(videoEncoder, "av1_amf") {
		switch encodingOptions.EncoderPreset {
		case entities.VerySlow, entities.Slower, entities.Slow:
			param += " -quality quality"
		case entities.Medium:
			param += " -quality balanced"
			//		case entities.Fast, entities.Faster, entities.VeryFast, entities.SuperFast, entities.UltraFast:
			//			param += " -quality speed"
		default:
			param += " -quality speed"
		}
		if strings.EqualFold(videoEncoder, "hevc_amf") || strings.EqualFold(videoEncoder, "av1_amf") {
			param += " -header_insertion_mode gop"
		}
		if strings.EqualFold(videoEncoder, "hevc_amf") {
			param += " -gops_per_idr 1"
		}
	} else if strings.EqualFold(videoEncoder, "h264_videotoolbox") || strings.EqualFold(videoEncoder, "hevc_videotoolbox") {
		switch encodingOptions.EncoderPreset {
		case entities.VerySlow, entities.Slower, entities.Slow, entities.Medium:
			param += " -prio_speed 0"
			//		case entities.Fast, entities.Faster, entities.VeryFast, entities.SuperFast, entities.UltraFast:
			//			param += " -prio_speed 1"
		default:
			param += " -prio_speed 1"
		}
		/*
			} else if strings.EqualFold(videoEncoder, "libvpx") {
				profileScore := 0
				if isVc1 {
					profileScore++
				}
				profileScore = int(math.Min(float64(profileScore), 2))
				param += fmt.Sprintf(" -speed 16 -quality good -profile:v %d -slices 8 -crf 10 -qmin 0 -qmax 50", profileScore)
			} else if strings.EqualFold(videoEncoder, "libvpx-vp9") {
				switch encodingOptions.EncoderPreset {
				case entities.VerySlow:
					param += " -deadline best -cpu-used 0"
				case entities.Slower:
					param += " -deadline best -cpu-used 2"
				case entities.Slow:
					param += " -deadline best -cpu-used 3"
				case entities.Medium:
					param += " -deadline good -cpu-used 0"
				case entities.Fast:
					param += " -deadline good -cpu-used 1"
				case entities.Faster:
					param += " -deadline good -cpu-used 2"
				case entities.VeryFast:
					param += " -deadline good -cpu-used 3"
				case entities.SuperFast:
					param += " -deadline good -cpu-used 4"
				case entities.UltraFast:
					param += " -deadline good -cpu-used 5"
				default:
					param += " -deadline good -cpu-used 1"
				}
				h265Crf := encodingOptions.H265Crf
				defaultVp9Crf := 31
				if h265Crf >= 0 && h265Crf <= 51 {
					const h265ToVp9CrfConversionFactor = 1.12
					vp9Crf := int(float64(h265Crf) * h265ToVp9CrfConversionFactor)
					vp9Crf = int(math.Max(math.Min(float64(vp9Crf), 63), 0))
					param += fmt.Sprintf(" -crf %d", vp9Crf)
				} else {
					param += fmt.Sprintf(" -crf %d", defaultVp9Crf)
				}
				param += " -row-mt 1 -profile 1"
			} else if strings.EqualFold(videoEncoder, "mpeg4") {
				param += " -mbd rd -flags +mv4+aic -trellis 2 -cmp 2 -subcmp 2 -bf 2"
			} else if strings.EqualFold(videoEncoder, "wmv2") { // asf/wmv
				param += " -qmin 2"
			} else if strings.EqualFold(videoEncoder, "msmpeg4") {
				param += " -mbd 2"
		*/
	}

	return param
}

func (e *EncodingHelper) GetVideoQualityParam(state *EncodingJobInfo, videoEncoder string, encodingOptions *configuration.EncodingOptions, defaultPreset entities.EncoderPreset) string {
	var param string

	// Tutorials: Enable Intel GuC / HuC firmware loading for Low Power Encoding.
	// https://01.org/group/43/downloads/firmware
	// https://wiki.archlinux.org/title/intel_graphics#Enable_GuC_/_HuC_firmware_loading
	// Intel Low Power Encoding can save unnecessary CPU-GPU synchronization,
	// which will reduce overhead in performance intensive tasks such as 4k transcoding and tonemapping.
	intelLowPowerHwEncoding := false

	// Workaround for linux 5.18 to 6.1.3 i915 hang at cost of performance.
	// https://github.com/intel/media-driver/issues/1456
	enableWaFori915Hang := false

	if encodingOptions.HardwareAccelerationType == entities.HardwareAccelerationType_VAAPI {
		isIntelVaapiDriver := e.mediaEncoder.IsVaapiDeviceInteliHD() || e.mediaEncoder.IsVaapiDeviceInteli965()

		if strings.EqualFold(videoEncoder, "h264_vaapi") {
			intelLowPowerHwEncoding = encodingOptions.EnableIntelLowPowerH264HwEncoder && isIntelVaapiDriver
		} else if strings.EqualFold(videoEncoder, "hevc_vaapi") {
			intelLowPowerHwEncoding = encodingOptions.EnableIntelLowPowerHevcHwEncoder && isIntelVaapiDriver
		}
	} else if encodingOptions.HardwareAccelerationType == entities.HardwareAccelerationType_QSV {
		if runtime.GOOS == "linux" {
			ver := KernelVersion
			isFixedKernel60 := ver.Major == 6 && ver.Minor == 0 && version.Compare(ver, minFixedKernel60i915Hang) >= 0
			isUnaffectedKernel := version.Compare(ver, minKerneli915Hang) < 0 || version.Compare(ver, maxKerneli915Hang) > 0
			if !(isUnaffectedKernel || isFixedKernel60) {
				vidDecoder := e.GetHardwareVideoDecoder(state, encodingOptions)
				isIntelDecoder := strings.Contains(strings.ToLower(vidDecoder), "qsv") ||
					strings.Contains(strings.ToLower(vidDecoder), "vaapi")
				doOclTonemap := e.mediaEncoder.SupportsHwaccel("qsv") &&
					e.IsVaapiSupported(state) &&
					e.IsOpenclFullSupported() &&
					!e.IsVaapiVppTonemapAvailable(state, encodingOptions) &&
					e.IsHwTonemapAvailable(state, encodingOptions)

				enableWaFori915Hang = isIntelDecoder && doOclTonemap
			}

		}

		if strings.EqualFold(videoEncoder, "h264_qsv") {
			intelLowPowerHwEncoding = encodingOptions.EnableIntelLowPowerH264HwEncoder
		} else if strings.EqualFold(videoEncoder, "hevc_qsv") {
			intelLowPowerHwEncoding = encodingOptions.EnableIntelLowPowerHevcHwEncoder
		} else {
			enableWaFori915Hang = false
		}
	}

	if intelLowPowerHwEncoding {
		param += " -low_power 1"
	}

	if enableWaFori915Hang {
		param += " -async_depth 1"
	}

	//	isVc1 := strings.EqualFold(state.VideoStream.Codec, "vc1")
	isLibX265 := strings.EqualFold(videoEncoder, "libx265")
	encodingPreset := encodingOptions.EncoderPreset

	param += e.GetEncoderParam(&encodingPreset, defaultPreset, encodingOptions, videoEncoder, isLibX265)
	param += e.GetVideoBitrateParam(*state, videoEncoder)

	framerate := e.GetFramerateParam(state)
	if framerate != nil {
		param += fmt.Sprintf(" -r %f", *framerate)
	}

	targetVideoCodec := state.ActualOutputVideoCodec()
	if strings.EqualFold(targetVideoCodec, "h265") || strings.EqualFold(targetVideoCodec, "hevc") {
		targetVideoCodec = "hevc"
	}

	profile := ""
	profiles := state.GetRequestedProfiles(targetVideoCodec)
	if len(profiles) > 0 {
		profile = strings.TrimSpace(profiles[0])
	}

	// We only transcode to HEVC 8-bit for now, force Main Profile.
	if strings.Contains(strings.ToLower(profile), "main10") || strings.Contains(strings.ToLower(profile), "mainstill") {
		profile = "main"
	}

	// Extended Profile is not supported by any known h264 encoders, force Main Profile.
	if strings.Contains(strings.ToLower(profile), "extended") {
		profile = "main"
	}

	// Only libx264 support encoding H264 High 10 Profile, otherwise force High Profile.
	if !strings.EqualFold(videoEncoder, "libx264") && strings.Contains(strings.ToLower(profile), "high10") {
		profile = "high"
	}

	// We only need Main profile of AV1 encoders.
	if strings.Contains(strings.ToLower(videoEncoder), "av1") && (strings.Contains(strings.ToLower(profile), "high") || strings.Contains(strings.ToLower(profile), "professional")) {
		profile = "main"
	}

	// h264_vaapi does not support Baseline profile, force Constrained Baseline in this case,
	// which is compatible (and ugly).
	if strings.EqualFold(videoEncoder, "h264_vaapi") && strings.Contains(strings.ToLower(profile), "baseline") {
		profile = "constrained_baseline"
	}

	// libx264, h264_{qsv,nvenc,rkmpp} does not support Constrained Baseline profile, force Baseline in this case.
	if (strings.EqualFold(videoEncoder, "libx264") ||
		strings.EqualFold(videoEncoder, "h264_qsv") ||
		strings.EqualFold(videoEncoder, "h264_nvenc") ||
		strings.EqualFold(videoEncoder, "h264_rkmpp")) &&
		strings.Contains(strings.ToLower(profile), strings.ToLower("baseline")) {
		profile = "baseline"
	}

	// libx264, h264_{qsv,nvenc,vaapi,rkmpp} does not support Constrained High profile, force High in this case.
	if (strings.EqualFold(videoEncoder, "libx264") ||
		strings.EqualFold(videoEncoder, "h264_qsv") ||
		strings.EqualFold(videoEncoder, "h264_nvenc") ||
		strings.EqualFold(videoEncoder, "h264_vaapi") ||
		strings.EqualFold(videoEncoder, "h264_rkmpp")) &&
		strings.Contains(strings.ToLower(profile), strings.ToLower("high")) {
		profile = "high"
	}

	if strings.EqualFold(videoEncoder, "h264_amf") &&
		strings.Contains(strings.ToLower(profile), strings.ToLower("baseline")) {
		profile = "constrained_baseline"
	}

	if strings.EqualFold(videoEncoder, "h264_amf") &&
		strings.Contains(strings.ToLower(profile), strings.ToLower("constrainedhigh")) {
		profile = "constrained_high"
	}

	if strings.EqualFold(videoEncoder, "h264_videotoolbox") &&
		strings.Contains(strings.ToLower(profile), strings.ToLower("constrainedbaseline")) {
		profile = "constrained_baseline"
	}

	if strings.EqualFold(videoEncoder, "h264_videotoolbox") &&
		strings.Contains(strings.ToLower(profile), strings.ToLower("constrainedhigh")) {
		profile = "constrained_high"
	}

	if profile != "" {
		// Currently there's no profile option in av1_nvenc encoder
		if !(strings.EqualFold(videoEncoder, "av1_nvenc") ||
			strings.EqualFold(videoEncoder, "h264_v4l2m2m")) {
			param += " -profile:v:0 " + profile
		}
	}

	level := state.GetRequestedLevel(targetVideoCodec)

	if level != "" {
		level = NormalizeTranscodingLevel(state, level)

		// libx264, QSV, AMF can adjust the given level to match the output.
		switch {
		case strings.EqualFold(videoEncoder, "h264_qsv") || strings.EqualFold(videoEncoder, "libx264"):
			param += " -level " + level
		case strings.EqualFold(videoEncoder, "hevc_qsv"):
			// hevc_qsv use -level 51 instead of -level 153.
			if hevcLevel, err := strconv.ParseFloat(level, 64); err == nil {
				param += " -level " + strconv.FormatFloat(hevcLevel/3, 'f', -1, 64)
			}
		case strings.EqualFold(videoEncoder, "av1_qsv") || strings.EqualFold(videoEncoder, "libsvtav1"):
			// libsvtav1 and av1_qsv use -level 60 instead of -level 16
			// https://aomedia.org/av1/specification/annex-a/
			if av1Level, err := strconv.Atoi(level); err == nil {
				x := 2 + (av1Level >> 2)
				y := av1Level & 3
				res := (x * 10) + y
				param += " -level " + strconv.Itoa(res)
			}
		case strings.EqualFold(videoEncoder, "h264_amf") || strings.EqualFold(videoEncoder, "hevc_amf") || strings.EqualFold(videoEncoder, "av1_amf"):
			param += " -level " + level
		case strings.EqualFold(videoEncoder, "h264_nvenc") || strings.EqualFold(videoEncoder, "hevc_nvenc") || strings.EqualFold(videoEncoder, "av1_nvenc"):
			// level option may cause NVENC to fail.
			// NVENC cannot adjust the given level, just throw an error.
		case strings.EqualFold(videoEncoder, "h264_vaapi") || strings.EqualFold(videoEncoder, "hevc_vaapi") || strings.EqualFold(videoEncoder, "av1_vaapi"):
			// level option may cause corrupted frames on AMD VAAPI.
			if e.mediaEncoder.IsVaapiDeviceInteliHD() || e.mediaEncoder.IsVaapiDeviceInteli965() {
				param += " -level " + level
			}
		case strings.EqualFold(videoEncoder, "h264_rkmpp") || strings.EqualFold(videoEncoder, "hevc_rkmpp"):
			param += " -level " + level
		case !strings.EqualFold(videoEncoder, "libx265"):
			param += " -level " + level
		}
	}

	if strings.EqualFold(videoEncoder, "libx264") {
		//		param += " -x264opts:0 subme=0:me_range=4:rc_lookahead=10:me=dia:no_chroma_me:8x8dct=0:partitions=none"
		param += " -x264opts:0 subme=0:me_range=16:rc_lookahead=10:me=hex:open_gop=0"
	}

	if strings.EqualFold(videoEncoder, "libx265") {
		// libx265 only accept level option in -x265-params.
		// level option may cause libx265 to fail.
		// libx265 cannot adjust the given level, just throw an error.
		//		param += " -x265-params:0 no-info=1"
		param += " -x265-params:0 no-scenecut=1:no-open-gop=1:no-info=1"
		if encodingOptions.EncoderPreset < entities.UltraFast {
			// The following params are slower than the ultrafast preset, don't use when ultrafast is selected.
			param += ":subme=3:merange=25:rc-lookahead=10:me=star:ctu=32:max-tu-size=32:min-cu-size=16:rskip=2:rskip-edge-threshold=2:no-sao=1:no-strong-intra-smoothing=1"
		}
	}

	if strings.EqualFold(videoEncoder, "libsvtav1") && version.Compare(*e.mediaEncoder.EncoderVersion(), minFFmpegSvtAv1Params) >= 0 {
		param += " -svtav1-params:0 rc=1:tune=0:film-grain=0:enable-overlays=1:enable-tf=0"
	}

	/* Access unit too large: 8192 < 20880 error */
	if strings.EqualFold(videoEncoder, "h264_vaapi") || strings.EqualFold(videoEncoder, "hevc_vaapi") && version.Compare(*e.mediaEncoder.EncoderVersion(), minFFmpegVaapiH26xEncA53CcSei) >= 0 {
		param += " -sei -a53_cc"
	}

	return param
}

/*
func contains(slice []string, item string, ignoreCase bool) bool {
    for _, i := range slice {
        if (ignoreCase && strings.EqualFold(i, item)) || (!ignoreCase && i == item) {
            return true
        }
    }
    return false
}
*/

/*
duplicate
func (e *EncodingHelper) GetVideoBitrateParamValue(request BaseEncodingJobOptions, videoStream *entities.MediaStream, outputVideoCodec string) int {
    bitrate := request.VideoBitRate

    if videoStream != nil {
        isUpscaling := request.Height != nil &&
            videoStream.Height != nil &&
            *request.Height > *videoStream.Height &&
            request.Width != nil &&
            videoStream.Width != nil &&
            *request.Width > *videoStream.Width

        // Don't allow bitrate increases unless upscaling
        if !isUpscaling && bitrate != nil && videoStream.BitRate != nil {
            bitrate = GetMinBitrate(*videoStream.BitRate, *bitrate)
        }

        if bitrate != nil {
            inputVideoCodec := videoStream.Codec
            bitrate = ScaleBitrate(*bitrate, inputVideoCodec, outputVideoCodec)

            // If a max bitrate was requested, don't let the scaled bitrate exceed it
            if request.VideoBitRate != nil {
                bitrate = MinInt(*bitrate, *request.VideoBitRate)
            }
        }
    }

    // Cap the max target bitrate to intMax/2 to satisfy the bufsize=bitrate*2.
    if bitrate == nil {
        return math.MaxInt / 2
    }
    return MinInt(*bitrate, math.MaxInt/2)
}
*/
/*

func (e *EncodingHelper) GetMinBitrate(sourceBitrate, requestedBitrate int) int {
    // these values were chosen from testing to improve low bitrate streams
    if sourceBitrate <= 2000000 {
        sourceBitrate = int(float64(sourceBitrate) * 2.5)
    } else if sourceBitrate <= 3000000 {
        sourceBitrate *= 2
    }

    bitrate := MinInt(sourceBitrate, requestedBitrate)
    return bitrate
}
*/

/*
func ScaleBitrate(bitrate int, inputVideoCodec, outputVideoCodec string) int {
	inputScaleFactor := GetVideoBitrateScaleFactor(inputVideoCodec)
	outputScaleFactor := GetVideoBitrateScaleFactor(outputVideoCodec)

	// Don't scale the real bitrate lower than the requested bitrate
	scaleFactor := math.Max(outputScaleFactor/inputScaleFactor, 1)

	if bitrate <= 500000 {
		scaleFactor = math.Max(scaleFactor, 4)
	} else if bitrate <= 1000000 {
		scaleFactor = math.Max(scaleFactor, 3)
	} else if bitrate <= 2000000 {
		scaleFactor = math.Max(scaleFactor, 2.5)
	} else if bitrate <= 3000000 {
		scaleFactor = math.Max(scaleFactor, 2)
	}

	return int(scaleFactor * float64(bitrate))
}

func GetVideoBitrateScaleFactor(codec string) float64 {
	// hevc & vp9 - 40% more efficient than h.264
	if strings.EqualFold(codec, "h265") || strings.EqualFold(codec, "hevc") || strings.EqualFold(codec, "vp9") {
		return 0.6
	}

	// av1 - 50% more efficient than h.264
	if strings.EqualFold(codec, "av1") {
		return 0.5
	}

	return 1.0
}
*/

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (e *EncodingHelper) GetVideoBitrateParam(state EncodingJobInfo, videoCodec string) string {
	if state.OutputVideoBitrate == nil {
		return ""
	}

	bitrate := int(*state.OutputVideoBitrate)

	// Bit rate under 1000k is not allowed in h264_qsv
	if strings.EqualFold(videoCodec, "h264_qsv") {
		bitrate = mmax(bitrate, 1000)
	}

	bufsize := bitrate * 2

	switch {
	case strings.EqualFold(videoCodec, "libvpx") || strings.EqualFold(videoCodec, "libvpx-vp9"):
		// When crf is used with vpx, b:v becomes a max rate
		// https://trac.ffmpeg.org/wiki/Encode/VP8
		// https://trac.ffmpeg.org/wiki/Encode/VP9
		return fmt.Sprintf(" -maxrate:v %d -bufsize:v %d -b:v %d", bitrate, bufsize, bitrate)
	case strings.EqualFold(videoCodec, "msmpeg4"):
		return fmt.Sprintf(" -b:v %d", bitrate)
	case strings.EqualFold(videoCodec, "libsvtav1"):
		return fmt.Sprintf(" -b:v %d -bufsize %d", bitrate, bufsize)
	case strings.EqualFold(videoCodec, "libx264") || strings.EqualFold(videoCodec, "libx265"):
		return fmt.Sprintf(" -maxrate %d -bufsize %d", bitrate, bufsize)
	case strings.EqualFold(videoCodec, "h264_amf") || strings.EqualFold(videoCodec, "hevc_amf") || strings.EqualFold(videoCodec, "av1_amf"):
		// Override the too high default qmin 18 in transcoding preset
		return fmt.Sprintf(" -rc cbr -qmin 0 -qmax 32 -b:v %d -maxrate %d -bufsize %d", bitrate, bitrate, bufsize)
	case strings.EqualFold(videoCodec, "h264_vaapi") || strings.EqualFold(videoCodec, "hevc_vaapi") || strings.EqualFold(videoCodec, "av1_vaapi"):
		// VBR in i965 driver may result in pixelated output.
		if e.mediaEncoder.IsVaapiDeviceInteli965() {
			return fmt.Sprintf(" -rc_mode CBR -b:v %d -maxrate %d -bufsize %d", bitrate, bitrate, bufsize)
		}
		return fmt.Sprintf(" -rc_mode VBR -b:v %d -maxrate %d -bufsize %d", bitrate, bitrate, bufsize)
	case strings.EqualFold(videoCodec, "h264_videotoolbox") || strings.EqualFold(videoCodec, "hevc_videotoolbox"):
		// The `maxrate` and `bufsize` options can potentially lead to performance regression
		// and even encoder hangs, especially when the value is very high.
		return fmt.Sprintf(" -b:v %d -qmin -1 -qmax -1", bitrate)
	default:
		return fmt.Sprintf(" -b:v %d -maxrate %d -bufsize %d", bitrate, bitrate, bufsize)
	}
}

func NormalizeTranscodingLevel(state *EncodingJobInfo, level string) string {
	requestLevel, err := strconv.ParseFloat(level, 64)
	if err == nil {
		if strings.EqualFold(state.ActualOutputVideoCodec(), "av1") {
			// Transcode to level 5.3 (15) and lower for maximum compatibility.
			// https://en.wikipedia.org/wiki/AV1#Levels
			if requestLevel < 0 || requestLevel >= 15 {
				return "15"
			}
		} else if strings.EqualFold(state.ActualOutputVideoCodec(), "hevc") || strings.EqualFold(state.ActualOutputVideoCodec(), "h265") {
			// Transcode to level 5.0 and lower for maximum compatibility.
			// Level 5.0 is suitable for up to 4k 30fps hevc encoding, otherwise let the encoder to handle it.
			// https://en.wikipedia.org/wiki/High_Efficiency_Video_Coding_tiers_and_levels
			// MaxLumaSampleRate = 3840*2160*30 = 248832000 < 267386880.
			if requestLevel < 0 || requestLevel >= 150 {
				return "150"
			}
		} else if strings.EqualFold(state.ActualOutputVideoCodec(), "h264") {
			// Transcode to level 5.1 and lower for maximum compatibility.
			// h264 4k 30fps requires at least level 5.1 otherwise it will break on safari fmp4.
			// https://en.wikipedia.org/wiki/Advanced_Video_Coding#Levels
			if requestLevel < 0 || requestLevel >= 51 {
				return "51"
			}
		}
	}
	return level
}

/*
	func GetProgressiveVideoFullCommandLine(state EncodingJobInfo, encodingOptions EncodingOptions, defaultPreset EncoderPreset) string {
	    videoCodec := GetVideoEncoder(state, encodingOptions)
	    format := ""
	    outputPath := state.OutputFilePath

	    // Check the file extension and context
	    if strings.EqualFold(filepath.Ext(outputPath), ".mp4") && state.BaseRequest.Context == Streaming {
	        format = " -f mp4 -movflags frag_keyframe+empty_moov+delay_moov"
	    }

	    threads := GetNumberOfThreads(state, encodingOptions, videoCodec)
	    inputModifier := GetInputModifier(state, encodingOptions, nil)

	    command := fmt.Sprintf(
	        "%s %s%s %s %s -map_metadata -1 -map_chapters -1 -threads %s %s%s%s -y \"%s\"",
	        inputModifier,
	        GetInputArgument(state, encodingOptions, nil),
	        "",
	        GetMapArgs(state),
	        GetProgressiveVideoArguments(state, encodingOptions, videoCodec, defaultPreset),
	        threads,
	        GetProgressiveVideoAudioArguments(state, encodingOptions),
	        GetSubtitleEmbedArguments(state),
	        format,
	        outputPath,
	    )

	    return strings.TrimSpace(command)
	}

	func GetProgressiveVideoArguments(state EncodingJobInfo, encodingOptions EncodingOptions, videoCodec string, defaultPreset EncoderPreset) string {
	    var args strings.Builder
	    args.WriteString(fmt.Sprintf("-codec:v:0 %s", videoCodec))

	    if state.BaseRequest.EnableMpegtsM2TsMode {
	        args.WriteString(" -mpegts_m2ts_mode 1")
	    }

	    if IsCopyCodec(videoCodec) {
	        if state.VideoStream != nil &&
	            strings.EqualFold(state.OutputContainer, "ts") &&
	            !strings.EqualFold(state.VideoStream.NalLengthSize, "0") {

	            bitStreamArgs := GetBitStreamArgs(state.VideoStream)
	            if bitStreamArgs != "" {
	                args.WriteString(" " + bitStreamArgs)
	            }
	        }

	        if state.RunTimeTicks != nil && state.BaseRequest.CopyTimestamps {
	            args.WriteString(" -copyts -avoid_negative_ts disabled -start_at_zero")
	        }

	        if state.RunTimeTicks == nil {
	            args.WriteString(" -fflags +genpts")
	        }
	    } else {
	        keyFrameArg := fmt.Sprintf(" -force_key_frames \"expr:gte(t,n_forced*%d)\"", 5)
	        args.WriteString(keyFrameArg)

	        hasGraphicalSubs := state.SubtitleStream != nil && !state.SubtitleStream.IsTextSubtitleStream && ShouldEncodeSubtitle(state)

	        videoProcessParam := GetVideoProcessingFilterParam(state, encodingOptions, videoCodec)
	        negativeMapArgs := GetNegativeMapArgsByFilters(state, videoProcessParam)

	        args.WriteString(negativeMapArgs)
	        args.WriteString(videoProcessParam)

	        hasCopyTs := strings.Contains(strings.ToLower(videoProcessParam), "copyts")

	        if state.RunTimeTicks != nil && state.BaseRequest.CopyTimestamps {
	            if !hasCopyTs {
	                args.WriteString(" -copyts")
	            }

	            args.WriteString(" -avoid_negative_ts disabled")

	            if !(state.SubtitleStream != nil && state.SubtitleStream.IsExternal && !state.SubtitleStream.IsTextSubtitleStream) {
	                args.WriteString(" -start_at_zero")
	            }
	        }

	        qualityParam := GetVideoQualityParam(state, videoCodec, encodingOptions, defaultPreset)
	        if qualityParam != "" {
	            args.WriteString(" " + strings.TrimSpace(qualityParam))
	        }
	    }

	    if state.OutputVideoSync != "" {
	        args.WriteString(GetVideoSyncOption(state.OutputVideoSync, "encoder_version")) // Replace "encoder_version" with actual version
	    }

	    args.WriteString(GetOutputFFlags(state))

	    return args.String()
	}
*/
func (e *EncodingHelper) GetAmdVaapiFullVidFiltersPrefered(
	state EncodingJobInfo,
	options configuration.EncodingOptions,
	vidDecoder string,
	vidEncoder string,
) ([]string, []string, []string) {
	return nil, nil, nil
	/*
		inW := state.VideoStream.Width
		inH := state.VideoStream.Height
		reqW := state.BaseRequest.Width
		reqH := state.BaseRequest.Height
		reqMaxW := state.BaseRequest.MaxWidth
		reqMaxH := state.BaseRequest.MaxHeight
		threeDFormat := state.MediaSource.Video3DFormat

		isVaapiDecoder := strings.Contains(strings.ToLower(vidDecoder), "vaapi")
		isVaapiEncoder := strings.Contains(strings.ToLower(vidEncoder), "vaapi")
		isSwDecoder := vidDecoder == ""
		isSwEncoder := !isVaapiEncoder
		isMjpegEncoder := strings.Contains(strings.ToLower(vidEncoder), "mjpeg")

		doDeintH264 := state.DeInterlace("h264", true) || state.DeInterlace("avc", true)
		doDeintHevc := state.DeInterlace("h265", true) || state.DeInterlace("hevc", true)
		doDeintH2645 := doDeintH264 || doDeintHevc

		hasSubs := state.SubtitleStream != nil && ShouldEncodeSubtitle(state) // Implement this function
		hasTextSubs := hasSubs && state.SubtitleStream.IsTextSubtitleStream
		hasGraphicalSubs := hasSubs && !state.SubtitleStream.IsTextSubtitleStream
		hasAssSubs := hasSubs && (strings.EqualFold(state.SubtitleStream.Codec, "ass") || strings.EqualFold(state.SubtitleStream.Codec, "ssa"))

		rotation := state.VideoStream.Rotation
		tranposeDir := ""
		if rotation != 0 {
			tranposeDir = GetVideoTransposeDirection(state) // Implement this function
		}
		doVkTranspose := isVaapiDecoder && tranposeDir != ""
		swapWAndH := abs(rotation) == 90 && (isSwDecoder || (isVaapiDecoder && doVkTranspose))

		swpInW := inW
		swpInH := inH
		if swapWAndH {
			swpInW, swpInH = inH, inW
		}

		// Main filters for video stream
		mainFilters := []string{
			GetOverwriteColorPropertiesParam(state, doVkTonemap), // Implement this function
		}

		if isSwDecoder {
			if doDeintH2645 {
				swDeintFilter := GetSwDeinterlaceFilter(state, options) // Implement this function
				mainFilters = append(mainFilters, swDeintFilter)
			}

			if doVkTonemap || hasSubs {
				mainFilters = append(mainFilters, "hwupload=derive_device=vulkan", "format=vulkan")
			} else {
				swScaleFilter := GetSwScaleFilter(state, options, vidEncoder, swpInW, swpInH, threeDFormat, reqW, reqH, reqMaxW, reqMaxH) // Implement this function
				mainFilters = append(mainFilters, swScaleFilter, "format=nv12")
			}
		} else if isVaapiDecoder {
			if doVkTranspose || doVkTonemap || hasSubs {
				// Add additional logic based on your conditions
				mainFilters = append(mainFilters, "hwmap=derive_device=drm", "format=drm_prime")
			} else {
				if doDeintH2645 {
					deintFilter := GetHwDeinterlaceFilter(state, options, "vaapi") // Implement this function
					mainFilters = append(mainFilters, deintFilter)
				}
				hwScaleFilter := GetHwScaleFilter("scale", "vaapi", "nv12", false, inW, inH, reqW, reqH, reqMaxW, reqMaxH) // Implement this function
				mainFilters = append(mainFilters, hwScaleFilter)
			}
		}

		// Handle vk transpose
		if doVkTranspose {
			if strings.EqualFold(tranposeDir, "reversal") {
				mainFilters = append(mainFilters, "flip_vulkan")
			} else {
				mainFilters = append(mainFilters, fmt.Sprintf("transpose_vulkan=dir=%s", tranposeDir))
			}
		}

		// vk libplacebo
		if doVkTonemap || hasSubs {
			libplaceboFilter := GetLibplaceboFilter(options, "bgra", doVkTonemap, swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH, isMjpegEncoder) // Implement this function
			mainFilters = append(mainFilters, libplaceboFilter, "format=vulkan")
		}

		if doVkTonemap && !hasSubs {
			mainFilters = append(mainFilters, "hwmap=derive_device=vaapi", "format=vaapi", "scale_vaapi=format=nv12")
			if doDeintH2645 {
				deintFilter := GetHwDeinterlaceFilter(state, options, "vaapi")
				mainFilters = append(mainFilters, deintFilter)
			}
		}

		if !hasSubs {
			if isSwEncoder && (doVkTonemap || isVaapiDecoder) {
				mainFilters = append(mainFilters, "hwdownload", "format=nv12")
			}
			if isSwDecoder && isVaapiEncoder && !doVkTonemap {
				mainFilters = append(mainFilters, "hwupload_vaapi")
			}
		}

		// Subtitle and overlay filters
		subFilters := []string{}
		overlayFilters := []string{}
		if hasSubs {
			if hasGraphicalSubs {
				subW := state.SubtitleStream.Width
				subH := state.SubtitleStream.Height
				subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, reqMaxH)
				subFilters = append(subFilters, subPreProcFilters, "format=bgra")
			} else if hasTextSubs {
				framerate := state.VideoStream.RealFrameRate
				subFramerate := 10
				if hasAssSubs {
					subFramerate = min(framerate, 60) // Implement min function
				}
				alphaSrcFilter := GetAlphaSrcFilter(state, swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH, subFramerate)
				subTextSubtitlesFilter := e.GetTextSubtitlesFilter(state, true, true)                                  // Implement this function
				subFilters = append(subFilters, alphaSrcFilter, "format=bgra", subTextSubtitlesFilter)
			}

			subFilters = append(subFilters, "hwupload=derive_device=vulkan", "format=vulkan")
			overlayFilters = append(overlayFilters, "overlay_vulkan=eof_action=pass:repeatlast=0")

			if isSwEncoder {
				overlayFilters = append(overlayFilters, "scale_vulkan=format=nv12", "hwdownload", "format=nv12")
			} else if isVaapiEncoder {
				overlayFilters = append(overlayFilters, "hwmap=derive_device=vaapi", "format=vaapi", "scale_vaapi=format=nv12")
				if doDeintH2645 {
					deintFilter := GetHwDeinterlaceFilter(state, options, "vaapi") // Implement this function
					overlayFilters = append(overlayFilters, deintFilter)
				}
			}
		}

		return mainFilters, subFilters, overlayFilters
	*/
}

func (e *EncodingHelper) GetVaapiLimitedVidFiltersPrefered(
	state EncodingJobInfo,
	options configuration.EncodingOptions,
	vidDecoder string,
	vidEncoder string,
) ([]string, []string, []string) {
	return nil, nil, nil
	/*
		inW := state.VideoStream.Width
		inH := state.VideoStream.Height
		reqW := state.BaseRequest.Width
		reqH := state.BaseRequest.Height
		reqMaxW := state.BaseRequest.MaxWidth
		reqMaxH := state.BaseRequest.MaxHeight
		threeDFormat := state.MediaSource.Video3DFormat

		isVaapiDecoder := strings.Contains(strings.ToLower(vidDecoder), "vaapi")
		isVaapiEncoder := strings.Contains(strings.ToLower(vidEncoder), "vaapi")
		isSwDecoder := vidDecoder == ""
		isSwEncoder := !isVaapiEncoder
		isMjpegEncoder := strings.Contains(strings.ToLower(vidEncoder), "mjpeg")
		isVaInVaOut := isVaapiDecoder && isVaapiEncoder
		isi965Driver := e.mediaEncoder.IsVaapiDeviceInteli965 // Assume these checks are defined
		// isAmdDriver := e.mediaEncoder.IsVaapiDeviceAmd

		doDeintH264 := state.DeInterlace("h264", true) || state.DeInterlace("avc", true)
		doDeintHevc := state.DeInterlace("h265", true) || state.DeInterlace("hevc", true)
		doDeintH2645 := doDeintH264 || doDeintHevc
		doOclTonemap := IsHwTonemapAvailable(state, options) // Implement this function

		hasSubs := state.SubtitleStream != nil && ShouldEncodeSubtitle(state) // Implement this function
		hasTextSubs := hasSubs && state.SubtitleStream.IsTextSubtitleStream
		hasGraphicalSubs := hasSubs && !state.SubtitleStream.IsTextSubtitleStream

		rotation := state.VideoStream.Rotation
		swapWAndH := int(math.Abs(float64(rotation))) == 90 && isSwDecoder
		swpInW := inW
		swpInH := inH
		if swapWAndH {
			swpInW, swpInH = inH, inW
		}

		// Main filters for video stream
		mainFilters := []string{
			GetOverwriteColorPropertiesParam(state, doOclTonemap), // Implement this function
		}

		outFormat := ""
		if isSwDecoder {
			// INPUT sw surface(memory)
			if doDeintH2645 {
				swDeintFilter := GetSwDeinterlaceFilter(state, options) // Implement this function
				mainFilters = append(mainFilters, swDeintFilter)
			}

			//outFormat = if doOclTonemap { "yuv420p10le" } else { "nv12" }
			var outFormat string
			if doOclTonemap {
				outFormat = "yuv420p10le"
			} else {
				outFormat = "nv12"
			}
			swScaleFilter := GetSwScaleFilter(state, options, vidEncoder, swpInW, swpInH, threeDFormat, reqW, reqH, reqMaxW, reqMaxH) // Implement this function
			if isMjpegEncoder && !doOclTonemap {
				swScaleFilter = fmt.Sprintf("%s:out_range=pc", swScaleFilter)
			}

			mainFilters = append(mainFilters, swScaleFilter, "format="+outFormat)

			if doOclTonemap {
				mainFilters = append(mainFilters, "hwupload=derive_device=opencl")
			}
		} else if isVaapiDecoder {
			// INPUT vaapi surface(vram)
			if doDeintH2645 {
				deintFilter := GetHwDeinterlaceFilter(state, options, "vaapi") // Implement this function
				mainFilters = append(mainFilters, deintFilter)
			}

			//outFormat = if doOclTonemap { "" } else { "nv12" }
			var outFormat string
			if doOclTonemap {
				outFormat = "yuv420p10le"
			} else {
				outFormat = "nv12"
			}
			hwScaleFilter := GetHwScaleFilter("scale", "vaapi", outFormat, false, inW, inH, reqW, reqH, reqMaxW, reqMaxH) // Implement this function

			if isMjpegEncoder {
				hwScaleFilter += fmt.Sprintf("%s:out_range=pc:mode=hq", hwScaleFilter)
			}

			if hwScaleFilter != "" {
				hwScaleFilter += ":extra_hw_frames=24"
			}

			mainFilters = append(mainFilters, hwScaleFilter)
		}

		// OpenCL tonemap handling
		if doOclTonemap && isVaapiDecoder {
			if isi965Driver {
				mainFilters = append(mainFilters, "hwmap=derive_device=opencl")
			} else {
				mainFilters = append(mainFilters, "hwdownload", "format=p010le", "hwupload=derive_device=opencl")
			}
		}

		if doOclTonemap {
			tonemapFilter := GetHwTonemapFilter(options, "opencl", "nv12", isMjpegEncoder) // Implement this function
			mainFilters = append(mainFilters, tonemapFilter)
		}

		if doOclTonemap && isVaInVaOut && isi965Driver {
			mainFilters = append(mainFilters, "hwmap=derive_device=vaapi:reverse=1", "format=vaapi")
		}

		memoryOutput := false
		isUploadForOclTonemap := doOclTonemap && (isSwDecoder || (isVaapiDecoder && !isi965Driver))
		isHwmapNotUsable := hasGraphicalSubs || isUploadForOclTonemap
		isHwmapForSubs := hasSubs && isVaapiDecoder
		isHwUnmapForTextSubs := hasTextSubs && isVaInVaOut && !isUploadForOclTonemap

		if (isVaapiDecoder && isSwEncoder) || isUploadForOclTonemap || isHwmapForSubs {
			memoryOutput = true
			// OUTPUT nv12 surface(memory)
			// prefer hwmap to hwdownload on opencl/vaapi.
			var hwMapAction string
			if isHwmapNotUsable {
				hwMapAction = "hwdownload"
			} else {
				hwMapAction = "hwmap"
			}
			mainFilters = append(mainFilters, hwMapAction)
			mainFilters = append(mainFilters, "format=nv12")
		}

		if isSwDecoder && isVaapiEncoder {
			memoryOutput = true
		}

		if memoryOutput && hasTextSubs {
			textSubtitlesFilter := GetTextSubtitlesFilter(state, false, false) // Implement this function
			mainFilters = append(mainFilters, textSubtitlesFilter)
		}

		if isHwUnmapForTextSubs {
			mainFilters = append(mainFilters, "hwmap", "format=vaapi")
		} else if memoryOutput && isVaapiEncoder && !hasGraphicalSubs {
			mainFilters = append(mainFilters, "hwupload_vaapi")
		}

		// Subtitle and overlay filters
		subFilters := []string{}
		overlayFilters := []string{}
		if memoryOutput {
			if hasGraphicalSubs {
				subW := state.SubtitleStream.Width
				subH := state.SubtitleStream.Height
				subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, reqMaxH) // Implement this function
				subFilters = append(subFilters, subPreProcFilters)
				overlayFilters = append(overlayFilters, "overlay=eof_action=pass:repeatlast=0")

				if isVaapiEncoder {
					overlayFilters = append(overlayFilters, "hwupload_vaapi")
				}
			}
		}

		return mainFilters, subFilters, overlayFilters
	*/
}

func (e *EncodingHelper) GetIntelQsvVaapiVidFiltersPrefered(
	state EncodingJobInfo,
	options configuration.EncodingOptions,
	vidDecoder string,
	vidEncoder string,
) ([]string, []string, []string) {
	inW := state.VideoStream.Width
	inH := state.VideoStream.Height
	reqW := state.BaseRequest.Width
	reqH := state.BaseRequest.Height
	reqMaxW := state.BaseRequest.MaxWidth
	reqMaxH := state.BaseRequest.MaxHeight
	threeDFormat := state.MediaSource.Video3DFormat

	isVaapiDecoder := strings.Contains(strings.ToLower(vidDecoder), "vaapi")
	isQsvDecoder := strings.Contains(strings.ToLower(vidDecoder), "qsv")
	isQsvEncoder := strings.Contains(strings.ToLower(vidEncoder), "qsv")
	isHwDecoder := isVaapiDecoder || isQsvDecoder
	isSwDecoder := vidDecoder == ""
	isSwEncoder := !isQsvEncoder
	isMjpegEncoder := strings.Contains(strings.ToLower(vidEncoder), "mjpeg")
	isQsvInQsvOut := isHwDecoder && isQsvEncoder

	doDeintH264 := state.DeInterlace("h264", true) || state.DeInterlace("avc", true)
	doDeintHevc := state.DeInterlace("h265", true) || state.DeInterlace("hevc", true)
	doVaVppTonemap := e.IsIntelVppTonemapAvailable(&state, options)
	doOclTonemap := !doVaVppTonemap && e.IsHwTonemapAvailable(&state, &options)
	doTonemap := doVaVppTonemap || doOclTonemap
	doDeintH2645 := doDeintH264 || doDeintHevc

	hasSubs := state.SubtitleStream != nil && ShouldEncodeSubtitle(state)
	hasTextSubs := hasSubs && state.SubtitleStream.IsTextSubtitleStream()
	hasGraphicalSubs := hasSubs && !state.SubtitleStream.IsTextSubtitleStream()
	hasAssSubs := hasSubs && (strings.EqualFold(state.SubtitleStream.Codec, "ass") || strings.EqualFold(state.SubtitleStream.Codec, "ssa"))
	var subW, subH *int
	if state.SubtitleStream != nil {
		subW = state.SubtitleStream.Width
		subH = state.SubtitleStream.Height
	}

	rotation := 0
	if state.VideoStream != nil {
		if state.VideoStream.Rotation != nil {
			rotation = *state.VideoStream.Rotation
		}
	}
	transposeDir := ""
	if rotation != 0 {
		transposeDir = e.GetVideoTransposeDirection(state)
	}
	doVppTranspose := transposeDir != ""
	swapWAndH := int(math.Abs(float64(rotation))) == 90 && (isSwDecoder || (isVaapiDecoder || isQsvDecoder) && doVppTranspose)
	swpInW := inW
	swpInH := inH
	if swapWAndH {
		swpInW, swpInH = inH, inW
	}

	// Main filters for video stream
	mainFilters := []string{
		e.GetOverwriteColorPropertiesParam(state, doTonemap),
	}

	if isSwDecoder {
		// INPUT sw surface(memory)
		if doDeintH2645 {
			swDeintFilter := GetSwDeinterlaceFilter(state, options)
			mainFilters = append(mainFilters, swDeintFilter)
		}

		outFormat := "nv12"
		if doOclTonemap {
			outFormat = "yuv420p10le"
		} else if hasGraphicalSubs {
			outFormat = "yuv420p"
		}

		swScaleFilter := GetSwScaleFilter(&state, &options, vidEncoder, swpInW, swpInH, threeDFormat, reqW, reqH, reqMaxW, reqMaxH)
		if isMjpegEncoder && !doOclTonemap {
			if swScaleFilter == "" {
				swScaleFilter = "scale=out_range=pc"
			} else {
				swScaleFilter += ":out_range=pc"
			}
		}

		mainFilters = append(mainFilters, swScaleFilter, fmt.Sprintf("format=%s", outFormat))

		if doOclTonemap {
			mainFilters = append(mainFilters, "hwupload=derive_device=opencl")
		}
	} else if isVaapiDecoder || isQsvDecoder {
		hwFilterSuffix := "vaapi"
		if isQsvDecoder {
			hwFilterSuffix = "qsv"
		}
		isRext := e.IsVideoStreamHevcRext(state)
		doVppFullRangeOut := isMjpegEncoder && version.Compare(*e.mediaEncoder.EncoderVersion(), minFFmpegQsvVppOutRangeOption) >= 0
		doVppScaleModeHq := isMjpegEncoder && version.Compare(*e.mediaEncoder.EncoderVersion(), minFFmpegQsvVppScaleModeOption) >= 0

		// INPUT vaapi/qsv surface(vram)
		if doDeintH2645 {
			deintFilter := GetHwDeinterlaceFilter(state, options, hwFilterSuffix) // Implement this function
			mainFilters = append(mainFilters, deintFilter)
		}

		if isVaapiDecoder && doVppTranspose {
			mainFilters = append(mainFilters, fmt.Sprintf("transpose_vaapi=dir=%s", transposeDir))
		}

		outFormat := "nv12"
		if doTonemap {
			if (isQsvDecoder && doVppTranspose) || isRext {
				outFormat = "p010"
			} else {
				outFormat = ""
			}
		}

		swapOutputWandH := isQsvDecoder && doVppTranspose && swapWAndH
		hwScalePrefix := "scale"
		if isQsvDecoder {
			hwScalePrefix = "vpp"
		}
		hwScaleFilter := e.GetHwScaleFilter(hwScalePrefix, hwFilterSuffix, outFormat, swapOutputWandH, swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH) // Implement this function

		if hwScaleFilter != "" && isQsvDecoder && doVppTranspose {
			hwScaleFilter += fmt.Sprintf(":transpose=%s", transposeDir)
		}

		if hwScaleFilter != "" && isMjpegEncoder {
			if (isQsvDecoder && !doVppFullRangeOut) || doOclTonemap {
				// Do nothing
			} else {
				hwScaleFilter += ":out_range=pc"
			}
			if isQsvDecoder {
				if doVppScaleModeHq {
					hwScaleFilter += ":scale_mode=hq"
				}
			} else {
				hwScaleFilter += ":mode=hq"
			}
		}

		if hwScaleFilter != "" && isVaapiDecoder {
			hwScaleFilter += ":extra_hw_frames=24"
		}

		mainFilters = append(mainFilters, hwScaleFilter)
	}

	// vaapi vpp tonemap
	if doVaVppTonemap && isHwDecoder {
		if isQsvDecoder {
			mainFilters = append(mainFilters, "hwmap=derive_device=vaapi", "format=vaapi")
		}

		tonemapFilter := GetHwTonemapFilter(options, "vaapi", "nv12", isMjpegEncoder) // Implement this function
		mainFilters = append(mainFilters, tonemapFilter)

		if isQsvDecoder {
			mainFilters = append(mainFilters, "hwmap=derive_device=qsv", "format=qsv")
		}
	}

	if doOclTonemap && isHwDecoder {
		mainFilters = append(mainFilters, "hwmap=derive_device=opencl:mode=read")
	}

	// ocl tonemap
	if doOclTonemap {
		tonemapFilter := GetHwTonemapFilter(options, "opencl", "nv12", isMjpegEncoder) // Implement this function
		mainFilters = append(mainFilters, tonemapFilter)
	}

	memoryOutput := false
	isUploadForOclTonemap := isSwDecoder && doOclTonemap
	isHwmapUsable := isSwEncoder && (doOclTonemap || isVaapiDecoder)
	if (isHwDecoder && isSwEncoder) || isUploadForOclTonemap {
		memoryOutput = true

		mainFilters = append(mainFilters, func() string {
			if isHwmapUsable {
				return "hwmap=mode=read"
			} else {
				return "hwdownload"
			}
		}())
		mainFilters = append(mainFilters, "format=nv12")
	}

	if isSwDecoder && isQsvEncoder {
		memoryOutput = true
	}

	if memoryOutput {
		if hasTextSubs {
			textSubtitlesFilter := e.GetTextSubtitlesFilter(state, false, false) // Implement this function
			mainFilters = append(mainFilters, textSubtitlesFilter)
		}
	}

	if isQsvInQsvOut {
		if doOclTonemap {
			mainFilters = append(mainFilters, "hwmap=derive_device=qsv:mode=write:reverse=1:extra_hw_frames=16", "format=qsv")
		} else if isVaapiDecoder {
			mainFilters = append(mainFilters, "hwmap=derive_device=qsv", "format=qsv")
		}
	}

	// Subtitle and overlay filters
	subFilters := []string{}
	overlayFilters := []string{}
	if isQsvInQsvOut {
		if hasSubs {
			reqMaxHeight := 1080
			if hasGraphicalSubs {
				subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, &reqMaxHeight)
				subFilters = append(subFilters, subPreProcFilters, "format=bgra")
			} else if hasTextSubs {
				framerate := state.VideoStream.RealFrameRate
				subFramerate := 10.0 // Default
				if hasAssSubs {
					subFramerate = math.Min(float64(*framerate), 60)
				}

				alphaSrcFilter := GetAlphaSrcFilter(state, swpInW, swpInH, reqW, reqH, reqMaxW, &reqMaxHeight, &subFramerate)
				subTextSubtitlesFilter := e.GetTextSubtitlesFilter(state, true, true) // Implement this function
				subFilters = append(subFilters, alphaSrcFilter, "format=bgra", subTextSubtitlesFilter)
			}

			subFilters = append(subFilters, "hwupload=derive_device=qsv:extra_hw_frames=64")

			overlayW, overlayH := GetFixedOutputSize(swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH) // Implement this function
			overlaySize := ""
			if overlayW != nil && overlayH != nil {
				overlaySize = fmt.Sprintf(":w=%d:h=%d", *overlayW, *overlayH)
			}
			overlayQsvFilter := fmt.Sprintf("overlay_qsv=eof_action=pass:repeatlast=0%s", overlaySize)
			overlayFilters = append(overlayFilters, overlayQsvFilter)
		}
	} else if memoryOutput {
		if hasGraphicalSubs {
			subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, reqMaxH) // Implement this function
			subFilters = append(subFilters, subPreProcFilters)
			overlayFilters = append(overlayFilters, "overlay=eof_action=pass:repeatlast=0")
		}
	}

	return mainFilters, subFilters, overlayFilters
}

func (e *EncodingHelper) GetIntelQsvDx11VidFiltersPrefered(
	state EncodingJobInfo,
	options configuration.EncodingOptions,
	vidDecoder string,
	vidEncoder string,
) ([]string, []string, []string) {
	inW := state.VideoStream.Width
	inH := state.VideoStream.Height
	reqW := state.BaseRequest.Width
	reqH := state.BaseRequest.Height
	reqMaxW := state.BaseRequest.MaxWidth
	reqMaxH := state.BaseRequest.MaxHeight
	threeDFormat := state.MediaSource.Video3DFormat

	isD3d11vaDecoder := strings.Contains(strings.ToLower(vidDecoder), "d3d11va")
	isQsvDecoder := strings.Contains(strings.ToLower(vidDecoder), "qsv")
	isQsvEncoder := strings.Contains(strings.ToLower(vidEncoder), "qsv")
	isHwDecoder := isD3d11vaDecoder || isQsvDecoder
	isSwDecoder := vidDecoder == ""
	isSwEncoder := !isQsvEncoder
	isMjpegEncoder := strings.Contains(strings.ToLower(vidEncoder), "mjpeg")
	isQsvInQsvOut := isHwDecoder && isQsvEncoder

	doDeintH264 := state.DeInterlace("h264", true) || state.DeInterlace("avc", true)
	doDeintHevc := state.DeInterlace("h265", true) || state.DeInterlace("hevc", true)
	doDeintH2645 := doDeintH264 || doDeintHevc
	doVppTonemap := e.IsIntelVppTonemapAvailable(&state, options)
	doOclTonemap := !doVppTonemap && e.IsHwTonemapAvailable(&state, &options)
	doTonemap := doVppTonemap || doOclTonemap

	hasSubs := state.SubtitleStream != nil && ShouldEncodeSubtitle(state) // Implement this function
	hasTextSubs := hasSubs && state.SubtitleStream.IsTextSubtitleStream()
	hasGraphicalSubs := hasSubs && !state.SubtitleStream.IsTextSubtitleStream()
	hasAssSubs := hasSubs && (strings.EqualFold(state.SubtitleStream.Codec, "ass") || strings.EqualFold(state.SubtitleStream.Codec, "ssa"))
	subW := state.SubtitleStream.Width
	subH := state.SubtitleStream.Height

	rotation := 0
	if state.VideoStream != nil && state.VideoStream.Rotation != nil {
		rotation = *state.VideoStream.Rotation
	}
	tranposeDir := ""
	if rotation != 0 {
		tranposeDir = e.GetVideoTransposeDirection(state)
	}
	doVppTranspose := tranposeDir != ""
	swapWAndH := int(math.Abs(float64(rotation))) == 90 && (isSwDecoder || (isD3d11vaDecoder || isQsvDecoder) && doVppTranspose)
	swpInW := inW
	swpInH := inH
	if swapWAndH {
		swpInW, swpInH = inH, inW
	}

	// Main filters for video stream
	mainFilters := []string{
		e.GetOverwriteColorPropertiesParam(state, doTonemap), // Implement this function
	}

	if isSwDecoder {
		// INPUT sw surface(memory)
		if doDeintH2645 {
			swDeintFilter := GetSwDeinterlaceFilter(state, options) // Implement this function
			mainFilters = append(mainFilters, swDeintFilter)
		}

		outFormat := "nv12"
		if doOclTonemap {
			outFormat = "yuv420p10le"
		} else if hasGraphicalSubs {
			outFormat = "yuv420p"
		}

		swScaleFilter := GetSwScaleFilter(&state, &options, vidEncoder, swpInW, swpInH, threeDFormat, reqW, reqH, reqMaxW, reqMaxH) // Implement this function
		if isMjpegEncoder && !doOclTonemap {
			if swScaleFilter == "" {
				swScaleFilter = "scale=out_range=pc"
			} else {
				swScaleFilter += ":out_range=pc"
			}
		}

		mainFilters = append(mainFilters, swScaleFilter, fmt.Sprintf("format=%s", outFormat))

		if doOclTonemap {
			mainFilters = append(mainFilters, "hwupload=derive_device=opencl")
		}
	} else if isD3d11vaDecoder || isQsvDecoder {
		isRext := e.IsVideoStreamHevcRext(state) // Implement this function
		twoPassVppTonemap := false
		doVppFullRangeOut := isMjpegEncoder && version.Compare(*e.mediaEncoder.EncoderVersion(), minFFmpegQsvVppOutRangeOption) >= 0
		doVppScaleModeHq := isMjpegEncoder && version.Compare(*e.mediaEncoder.EncoderVersion(), minFFmpegQsvVppScaleModeOption) >= 0
		doVppProcamp := false
		procampParams := ""
		procampParamsString := ""

		if doVppTonemap {
			if isRext {
				twoPassVppTonemap = true
			}

			if options.VppTonemappingBrightness != 0 && options.VppTonemappingBrightness >= -100 && options.VppTonemappingBrightness <= 100 {
				procampParamsString += ":brightness=%d"
				twoPassVppTonemap, doVppProcamp = true, true
			}

			if options.VppTonemappingContrast > 1 && options.VppTonemappingContrast <= 10 {
				procampParamsString += ":contrast=%d"
				twoPassVppTonemap, doVppProcamp = true, true
			}

			if doVppProcamp {
				procampParamsString += ":procamp=1:async_depth=2"
				procampParams = fmt.Sprintf(procampParamsString, options.VppTonemappingBrightness, options.VppTonemappingContrast)
			}
		}

		outFormat := "nv12"
		if doOclTonemap {
			if doVppTranspose || isRext {
				outFormat = "p010"
			} else {
				outFormat = ""
			}
		}
		if twoPassVppTonemap {
			outFormat = "p010"
		}

		swapOutputWandH := doVppTranspose && swapWAndH
		hwScaleFilter := e.GetHwScaleFilter("vpp", "qsv", outFormat, swapOutputWandH, swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH)

		if hwScaleFilter != "" && doVppTranspose {
			hwScaleFilter += fmt.Sprintf(":transpose=%s", tranposeDir)
		}

		if hwScaleFilter != "" && isMjpegEncoder {
			if doVppFullRangeOut && !doOclTonemap {
				hwScaleFilter += ":out_range=pc"
			}
			if doVppScaleModeHq {
				hwScaleFilter += ":scale_mode=hq"
			}
		}

		if hwScaleFilter != "" && doVppTonemap {
			hwScaleFilter += procampParams
			if !twoPassVppTonemap {
				hwScaleFilter += ":tonemap=1"
			}
		}

		if isD3d11vaDecoder {
			if hwScaleFilter != "" || doDeintH2645 {
				mainFilters = append(mainFilters, "hwmap=derive_device=qsv")
			}
		}

		// hw deint
		if doDeintH2645 {
			deintFilter := GetHwDeinterlaceFilter(state, options, "qsv") // Implement this function
			mainFilters = append(mainFilters, deintFilter)
		}

		// hw transpose & scale & tonemap
		mainFilters = append(mainFilters, hwScaleFilter)

		// hw tonemap
		if doVppTonemap && twoPassVppTonemap {
			mainFilters = append(mainFilters, "vpp_qsv=tonemap=1:format=nv12:async_depth=2")
		}

		if doVppTonemap {
			mainFilters = append(mainFilters, e.GetOverwriteColorPropertiesParam(state, false)) // Implement this function
		}
	}

	if doOclTonemap && isHwDecoder {
		mainFilters = append(mainFilters, "hwmap=derive_device=opencl:mode=read")
	}

	// hw tonemap
	if doOclTonemap {
		tonemapFilter := GetHwTonemapFilter(options, "opencl", "nv12", isMjpegEncoder) // Implement this function
		mainFilters = append(mainFilters, tonemapFilter)
	}

	memoryOutput := false
	isUploadForOclTonemap := isSwDecoder && doOclTonemap
	isHwmapUsable := isSwEncoder && doOclTonemap
	if (isHwDecoder && isSwEncoder) || isUploadForOclTonemap {
		memoryOutput = true

		// OUTPUT nv12 surface(memory)
		// prefer hwmap to hwdownload on opencl.
		// qsv hwmap is not fully implemented for the time being.
		var filter string
		if isHwmapUsable {
			filter = "hwmap=mode=read"
		} else {
			filter = "hwdownload"
		}
		mainFilters = append(mainFilters, filter)
		mainFilters = append(mainFilters, "format=nv12")
	}

	if isSwDecoder && isQsvEncoder {
		memoryOutput = true
	}

	if memoryOutput {
		if hasTextSubs {
			textSubtitlesFilter := e.GetTextSubtitlesFilter(state, false, false)
			mainFilters = append(mainFilters, textSubtitlesFilter)
		}
	}

	if isQsvInQsvOut && doOclTonemap {
		mainFilters = append(mainFilters, "hwmap=derive_device=qsv:mode=write:reverse=1", "format=qsv")
	}

	// Subtitle and overlay filters
	subFilters := []string{}
	overlayFilters := []string{}
	if isQsvInQsvOut {
		if hasSubs {
			if hasGraphicalSubs {
				reqMaxH := 1080
				subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, &reqMaxH)
				subFilters = append(subFilters, subPreProcFilters, "format=bgra")
			} else if hasTextSubs {
				framerate := state.VideoStream.RealFrameRate
				subFramerate := 10.0
				if hasAssSubs {
					if framerate != nil {
						subFramerate = math.Min((float64)(*framerate), 60)
					} else {
						subFramerate = 25
					}
				}
				requestedMaxHeight := 1080
				alphaSrcFilter := GetAlphaSrcFilter(state, swpInW, swpInH, reqW, reqH, reqMaxW, &requestedMaxHeight, &subFramerate)
				subTextSubtitlesFilter := e.GetTextSubtitlesFilter(state, true, true)
				subFilters = append(subFilters, alphaSrcFilter, "format=bgra", subTextSubtitlesFilter)
			}

			subFilters = append(subFilters, "hwupload=derive_device=qsv:extra_hw_frames=64")

			overlayW, overlayH := GetFixedOutputSize(swpInW, swpInH, reqW, reqH, reqMaxW, reqMaxH) // Implement this function
			overlaySize := ""
			if overlayW != nil && overlayH != nil {
				overlaySize = fmt.Sprintf(":w=%d:h=%d", *overlayW, *overlayH)
			}
			overlayQsvFilter := fmt.Sprintf("overlay_qsv=eof_action=pass:repeatlast=0%s", overlaySize)
			overlayFilters = append(overlayFilters, overlayQsvFilter)
		}
	} else if memoryOutput {
		if hasGraphicalSubs {
			reqMaxH := 1080
			subPreProcFilters := GetGraphicalSubPreProcessFilters(swpInW, swpInH, subW, subH, reqW, reqH, reqMaxW, &reqMaxH) // Implement this function
			subFilters = append(subFilters, subPreProcFilters)
			overlayFilters = append(overlayFilters, "overlay=eof_action=pass:repeatlast=0")
		}
	}

	return mainFilters, subFilters, overlayFilters
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}
func (e *EncodingHelper) IsIntelVppTonemapAvailable(state *EncodingJobInfo, options configuration.EncodingOptions) bool {
	if state.VideoStream == nil ||
		!options.EnableVppTonemapping ||
		GetVideoColorBitDepth(state) < 10 {
		return false
	}

	// Prefer 'tonemap_vaapi' over 'vpp_qsv' on Windows for supporting Gen9/KBLx.
	// 'vpp_qsv' requires VPL, which is only supported on Gen12/TGLx and newer.
	if isWindows() &&
		options.HardwareAccelerationType == entities.HardwareAccelerationType_QSV &&
		version.Compare(*e.mediaEncoder.EncoderVersion(), minFFmpegQsvVppTonemapOption) < 0 {
		return false
	}

	return state.VideoStream.VideoRange == enums.VideoRangeHDR &&
		(state.VideoStream.VideoRangeType == enums.VideoRangeTypeHDR10 ||
			state.VideoStream.VideoRangeType == enums.VideoRangeTypeDOVIWithHDR10)
}

func ShouldEncodeSubtitle(state EncodingJobInfo) bool {
	return state.SubtitleDeliveryMethod == dlna.Encode ||
		(state.BaseRequest.AlwaysBurnInSubtitleWhenTranscoding && !IsCopyCodec(state.OutputVideoCodec))
}

func (e *EncodingHelper) IsVideoStreamHevcRext(state EncodingJobInfo) bool {
	videoStream := state.VideoStream
	if videoStream == nil {
		return false
	}

	return strings.EqualFold(videoStream.Codec, "hevc") &&
		(strings.EqualFold(videoStream.Profile, "Rext") ||
			strings.EqualFold(videoStream.PixelFormat, "yuv420p12le") ||
			strings.EqualFold(videoStream.PixelFormat, "yuv422p") ||
			strings.EqualFold(videoStream.PixelFormat, "yuv422p10le") ||
			strings.EqualFold(videoStream.PixelFormat, "yuv422p12le") ||
			strings.EqualFold(videoStream.PixelFormat, "yuv444p") ||
			strings.EqualFold(videoStream.PixelFormat, "yuv444p10le") ||
			strings.EqualFold(videoStream.PixelFormat, "yuv444p12le"))
}

func (e *EncodingHelper) GetVideoTransposeDirection(state EncodingJobInfo) string {
	var rotation int
	if state.VideoStream != nil && state.VideoStream.Rotation != nil {
		rotation = *state.VideoStream.Rotation
	} else {
		rotation = 0
	}

	switch rotation {
	case 90:
		return "cclock"
	case 180:
		return "reversal"
	case -90:
		return "clock"
	case -180:
		return "reversal"
	default:
		return ""
	}
}

// GetVideoBitrateScaleFactor returns the scale factor for a given codec.
func GetVideoBitrateScaleFactor(codec string) float64 {
	// hevc & vp9 - 40% more efficient than h.264
	if strings.EqualFold(codec, "h265") ||
		strings.EqualFold(codec, "hevc") ||
		strings.EqualFold(codec, "vp9") {
		return 0.6
	}

	// av1 - 50% more efficient than h.264
	if strings.EqualFold(codec, "av1") {
		return 0.5
	}

	return 1.0
}

func ScaleBitrate(bitrate int, inputVideoCodec, outputVideoCodec string) int {
	inputScaleFactor := GetVideoBitrateScaleFactor(inputVideoCodec)
	outputScaleFactor := GetVideoBitrateScaleFactor(outputVideoCodec)

	// Don't scale the real bitrate lower than the requested bitrate
	scaleFactor := math.Max(outputScaleFactor/inputScaleFactor, 1.0)

	switch {
	case bitrate <= 500000:
		scaleFactor = math.Max(scaleFactor, 4.0)
	case bitrate <= 1000000:
		scaleFactor = math.Max(scaleFactor, 3.0)
	case bitrate <= 2000000:
		scaleFactor = math.Max(scaleFactor, 2.5)
	case bitrate <= 3000000:
		scaleFactor = math.Max(scaleFactor, 2.0)
	case bitrate >= 30000000:
		// Don't scale beyond 30Mbps, it is hardly visually noticeable for most codecs
		// with our prefer speed encoding and will cause extremely high bitrate to be
		// used for av1->h264 transcoding that will overload clients and encoders
		scaleFactor = 1.0
	}

	return int(scaleFactor * float64(bitrate))
}

type DynamicHdrMetadataRemovalPlan string

const (
	DynamicHdrMetadataRemovalPlanNone            DynamicHdrMetadataRemovalPlan = "None"
	DynamicHdrMetadataRemovalPlanRemoveDovi      DynamicHdrMetadataRemovalPlan = "RemoveDovi"
	DynamicHdrMetadataRemovalPlanRemoveHdr10Plus DynamicHdrMetadataRemovalPlan = "RemoveHdr10Plus"
)

func ShouldRemoveDynamicHdrMetadata(state *EncodingJobInfo) DynamicHdrMetadataRemovalPlan {
	videoStream := state.VideoStream
	if videoStream == nil || videoStream.VideoRange != enums.VideoRangeHDR {
		return DynamicHdrMetadataRemovalPlanNone
	}

	requestedRangeTypes := state.GetRequestedRangeTypes(videoStream.Codec)
	if len(requestedRangeTypes) == 0 {
		return DynamicHdrMetadataRemovalPlanNone
	}

	requestHasHDR10 := containsCaseInsensitive(requestedRangeTypes, string(enums.VideoRangeTypeHDR10))
	requestHasDOVI := containsCaseInsensitive(requestedRangeTypes, string(enums.VideoRangeTypeDOVI))
	requestHasDOVIwithEL := containsCaseInsensitive(requestedRangeTypes, string(enums.VideoRangeTypeDOVIWithEL))
	requestHasDOVIwithELHDR10plus := containsCaseInsensitive(requestedRangeTypes, string(enums.VideoRangeTypeDOVIWithELHDR10Plus))

	shouldRemoveHdr10Plus := false
	// Case 1: Client supports HDR10, does not support DOVI with EL but EL presents
	shouldRemoveDovi := (!requestHasDOVIwithEL && requestHasHDR10) && videoStream.VideoRangeType == enums.VideoRangeTypeDOVIWithEL

	// Case 2: Client supports DOVI, does not support broken DOVI config
	// Client does not report DOVI support should be allowed to copy bad data for remuxing as HDR10 players would not crash
	shouldRemoveDovi = shouldRemoveDovi || (requestHasDOVI && videoStream.VideoRangeType == enums.VideoRangeTypeDOVIInvalid)

	// Special case: we have a video with both EL and HDR10+
	// If the client supports EL but not in the case of coexistence with HDR10+, remove HDR10+ for compatibility reasons.
	// Otherwise, remove DOVI if the client is not a DOVI player
	if videoStream.VideoRangeType == enums.VideoRangeTypeDOVIWithELHDR10Plus {
		shouldRemoveHdr10Plus = requestHasDOVIwithEL && !requestHasDOVIwithELHDR10plus
		shouldRemoveDovi = shouldRemoveDovi || !shouldRemoveHdr10Plus
	}

	if shouldRemoveDovi {
		return DynamicHdrMetadataRemovalPlanRemoveDovi
	}

	// If the client is a Dolby Vision Player, remove the HDR10+ metadata to avoid playback issues
	shouldRemoveHdr10Plus = shouldRemoveHdr10Plus || (requestHasDOVI && videoStream.VideoRangeType == enums.VideoRangeTypeDOVIWithHDR10Plus)
	if shouldRemoveHdr10Plus {
		return DynamicHdrMetadataRemovalPlanRemoveHdr10Plus
	}
	return DynamicHdrMetadataRemovalPlanNone
}

func containsCaseInsensitive(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func (e *EncodingHelper) CanEncoderRemoveDynamicHdrMetadata(plan DynamicHdrMetadataRemovalPlan, videoStream *entities.MediaStream) bool {
	switch plan {
	case DynamicHdrMetadataRemovalPlanRemoveDovi:
		return e.mediaEncoder.SupportsBitStreamFilterWithOption(BitStreamFilterOptionTypeDoviRpuStrip) ||
			(IsH265(*videoStream) && e.mediaEncoder.SupportsBitStreamFilterWithOption(BitStreamFilterOptionTypeHevcMetadataRemoveDovi)) ||
			(IsAv1(*videoStream) && e.mediaEncoder.SupportsBitStreamFilterWithOption(BitStreamFilterOptionTypeAv1MetadataRemoveDovi))
	case DynamicHdrMetadataRemovalPlanRemoveHdr10Plus:
		return (IsH265(*videoStream) && e.mediaEncoder.SupportsBitStreamFilterWithOption(BitStreamFilterOptionTypeHevcMetadataRemoveHdr10Plus)) ||
			(IsAv1(*videoStream) && e.mediaEncoder.SupportsBitStreamFilterWithOption(BitStreamFilterOptionTypeAv1MetadataRemoveHdr10Plus))
	default:
		return true
	}
}

func GetVideoSyncOption(videoSync string, encoderVersion version.Version) string {
	if videoSync == "" {
		return ""
	}

	if version.Compare(encoderVersion, version.Version{Major: 5, Minor: 1}) >= 0 {
		vsync, err := strconv.Atoi(videoSync)
		if err == nil {
			switch vsync {
			case -1:
				return " -fps_mode auto"
			case 0:
				return " -fps_mode passthrough"
			case 1:
				return " -fps_mode cfr"
			case 2:
				return " -fps_mode vfr"
			default:
				return ""
			}
		}
		return ""
	}

	// -vsync is deprecated in FFmpeg 5.1 and will be removed in the future.
	return fmt.Sprintf(" -vsync %s", videoSync)
}
