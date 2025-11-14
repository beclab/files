package helpers

import (
	"context"
	"math"
	//	"net"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	//	"time"

	"files/pkg/media/api/models/streamingdtos"
	"files/pkg/media/jellyfin/data/enums"
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/utils"
	//	"files/pkg/media/mediabrowser/model/net"
	cc "files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/controller/mediaencoding/transcodemanager"
	"files/pkg/media/mediabrowser/controller/streaming"
)

type DynamicHlsHelper struct {
	//	libraryManager             ILibraryManager
	//	userManager                IUserManager
	//	mediaSourceManager         IMediaSourceManager
	serverConfigurationManager cc.IServerConfigurationManager
	mediaEncoder               mediaencoding.IMediaEncoder
	transcodeManager           transcodemanager.ITranscodeManager
	//	networkManager             INetworkManager
	logger *utils.Logger
	//	httpContextAccessor        IHttpContextAccessor
	encodingHelper mediaencoding.EncodingHelper
	// trickplayManager           ITrickplayManager
}

func NewDynamicHlsHelper(
	//	libraryManager ILibraryManager,
	//	userManager IUserManager,
	//	mediaSourceManager IMediaSourceManager,
	serverConfigurationManager cc.IServerConfigurationManager,
	mediaEncoder mediaencoding.IMediaEncoder,
	transcodeManager transcodemanager.ITranscodeManager,
	//	networkManager INetworkManager,
	logger *utils.Logger,
	//	httpContextAccessor IHttpContextAccessor,
	encodingHelper mediaencoding.EncodingHelper,
	// trickplayManager ITrickplayManager,
) *DynamicHlsHelper {
	return &DynamicHlsHelper{
		//		libraryManager:             libraryManager,
		//		userManager:                userManager,
		//		mediaSourceManager:         mediaSourceManager,
		serverConfigurationManager: serverConfigurationManager,
		mediaEncoder:               mediaEncoder,
		transcodeManager:           transcodeManager,
		//		networkManager:             networkManager,
		logger: logger,
		//		httpContextAccessor:        httpContextAccessor,
		encodingHelper: encodingHelper,
		//		trickplayManager:           trickplayManager,
	}
}

func (h *DynamicHlsHelper) GetMasterHlsPlaylist(
	r *http.Request,
	transcodingJobType mediaencoding.TranscodingJobType,
	//streamingRequest streaming.StreamingRequestDto,
	request interface{},
	enableAdaptiveBitrateStreaming bool,
) (string, error) {
	streamingRequest, _ := request.(*streamingdtos.HlsVideoRequestDto)
	//	isHeadRequest := h.httpContextAccessor.HttpContext().Request.Method == http.MethodHead
	isHeadRequest := false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return h.getMasterPlaylistInternal(
		r,
		streamingRequest.VideoRequestDto,
		isHeadRequest,
		enableAdaptiveBitrateStreaming,
		transcodingJobType,
		ctx,
	)
}

func (d *DynamicHlsHelper) getMasterPlaylistInternal(
	r *http.Request,
	//	streamingRequest *streaming.StreamingRequestDto,
	request interface{},
	isHeadRequest bool,
	enableAdaptiveBitrateStreaming bool,
	transcodingJobType mediaencoding.TranscodingJobType,
	ctx context.Context,
) (string, error) {
	/*
	   if ctx.Request.Context() == nil {
	       return nil, errors.New("resource not found: context")
	   }
	*/
	state, err := GetStreamingState(
		request,
		r,
		//		d.mediaSourceManager,
		//		d.userManager,
		//		d.libraryManager,
		d.serverConfigurationManager,
		d.mediaEncoder,
		d.encodingHelper,
		d.transcodeManager,
		transcodingJobType,
		ctx,
	)
	if err != nil {
		return "", err
	}

	//	ctx.Writer.Header().Set(HeaderNames.Expires, "0")
	if isHeadRequest {
		//return ctx.Writer.Write([]byte{}), MimeTypes.GetMimeType("playlist.m3u8")
		return "", nil
	}

	var totalBitrate int
	if state.OutputAudioBitrate != nil {
		totalBitrate += *state.OutputAudioBitrate
	}
	if state.OutputVideoBitrate != nil {
		totalBitrate += *state.OutputVideoBitrate
	}

	builder := strings.Builder{}
	builder.WriteString("#EXTM3U\n")

	isLiveStream := state.IsSegmentedLiveStream()

	queryString := r.URL.Query().Encode()

	// from universal audio service
	if state.Request.SegmentContainer != nil && *state.Request.SegmentContainer != "" && !strings.Contains(strings.ToLower(queryString), strings.ToLower("SegmentContainer")) {
		queryString += "&SegmentContainer=" + *state.Request.SegmentContainer
	}

	// from universal audio service
	if state.Request.TranscodeReasons != "" && !strings.Contains(strings.ToLower(queryString), strings.ToLower("TranscodeReasons=")) {
		queryString += "&TranscodeReasons=" + state.Request.TranscodeReasons
	}

	// Main stream
	playlistUrl := "live.m3u8"
	if !isLiveStream {
		playlistUrl = "main.m3u8"
	}
	playlistUrl += "?" + queryString

	subtitleStreams := make([]entities.MediaStream, 0)
	for _, stream := range state.MediaSource.MediaStreams {
		if stream.IsTextSubtitleStream() {
			subtitleStreams = append(subtitleStreams, stream)
		}
	}

	subtitleGroup := ""
	if len(subtitleStreams) > 0 && (state.SubtitleDeliveryMethod == dlna.Hls || state.VideoRequest.EnableSubtitlesInManifest) {
		subtitleGroup = "subs"
	} else if state.SubtitleStream != nil && state.SubtitleDeliveryMethod == dlna.Encode {
		subtitleGroup = ""
	}

	if subtitleGroup != "" {
		d.addSubtitles(state, subtitleStreams, &builder /*, ctx.GetString(HeaderNames.Authorization)*/)
	}

	basicPlaylist := d.appendPlaylist(&builder, state, playlistUrl, totalBitrate, subtitleGroup)

	if state.VideoStream != nil && state.VideoRequest != nil {
		encodingOptions := d.serverConfigurationManager.GetEncodingOptions()

		// Provide SDR HEVC entrance for backward compatibility.
		if encodingOptions.AllowHevcEncoding && !encodingOptions.AllowAv1Encoding && mediaencoding.IsCopyCodec(state.OutputVideoCodec) && state.VideoStream.VideoRange == enums.VideoRangeHDR && strings.EqualFold(state.ActualOutputVideoCodec(), "hevc") {
			requestedVideoProfiles := state.GetRequestedProfiles("hevc")
			if requestedVideoProfiles != nil && len(requestedVideoProfiles) > 0 {
				// Force HEVC Main Profile and disable video stream copy.
				state.OutputVideoCodec = "hevc"
				sdrVideoUrl := d.replaceProfile(playlistUrl, "hevc", strings.Join(requestedVideoProfiles, ","), "main") + "&AllowVideoStreamCopy=false"
				// HACK: Use the same bitrate so that the client can choose by other attributes, such as color range.
				d.appendPlaylist(&builder, state, sdrVideoUrl, totalBitrate, subtitleGroup)
				// Restore the video codec
				state.OutputVideoCodec = "copy"
			}
		}

		// Provide Level 5.0 entrance for backward compatibility.
		if encodingOptions.AllowHevcEncoding && !encodingOptions.AllowAv1Encoding && mediaencoding.IsCopyCodec(state.OutputVideoCodec) && state.VideoStream.Level != nil && *state.VideoStream.Level > 150 && state.VideoStream.VideoRange == enums.VideoRangeSDR && strings.EqualFold(state.ActualOutputVideoCodec(), "hevc") {
			playlistCodecsField := strings.Builder{}
			d.appendPlaylistCodecsField(&playlistCodecsField, state)

			// Force the video level to 5.0.
			originalLevel := state.VideoStream.Level
			var level float64 = 150
			state.VideoStream.Level = &level
			newPlaylistCodecsField := strings.Builder{}
			d.appendPlaylistCodecsField(&newPlaylistCodecsField, state)

			// Restore the video level.
			state.VideoStream.Level = originalLevel
			newPlaylist := d.replacePlaylistCodecsField(basicPlaylist, &playlistCodecsField, &newPlaylistCodecsField)
			builder.WriteString(newPlaylist)
		}
	}

	fmt.Printf("--> %+v\n", state)
	fmt.Printf("%+v\n", isLiveStream)
	fmt.Printf("%+v\n", enableAdaptiveBitrateStreaming)
	if d.enableAdaptiveBitrateStreaming(state, isLiveStream, enableAdaptiveBitrateStreaming /*, r.RemoteAddr*/) {
		fmt.Println("vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv")
		requestedVideoBitrate := 0
		if state.VideoRequest != nil {
			if state.VideoRequest.VideoBitRate != nil {
				requestedVideoBitrate = *state.VideoRequest.VideoBitRate
			}
		}

		// By default, vary by just 200k
		variation := d.getBitrateVariation(totalBitrate)

		newBitrate := totalBitrate - variation
		variantUrl := d.replaceVideoBitrate(playlistUrl, requestedVideoBitrate, requestedVideoBitrate-variation)
		d.appendPlaylist(&builder, state, variantUrl, newBitrate, subtitleGroup)

		variation *= 2
		newBitrate = totalBitrate - variation
		variantUrl = d.replaceVideoBitrate(playlistUrl, requestedVideoBitrate, requestedVideoBitrate-variation)
		d.appendPlaylist(&builder, state, variantUrl, newBitrate, subtitleGroup)
	}

	/*
	   if !isLiveStream && (state.VideoRequest != nil && state.VideoRequest.EnableTrickplay) {
	       sourceID, _ := uuid.Parse(state.Request.MediaSourceId)
	       trickplayResolutions, err := c.trickplayManager.GetTrickplayResolutions(sourceID)
	       if err == nil {
	           c.addTrickplay(ctx, state, trickplayResolutions, &builder, ctx.GetString(HeaderNames.Authorization))
	       }
	   }
	*/

	//return ctx.Writer.Write([]byte(builder.String())), net.GetMimeType("playlist.m3u8")
	return builder.String(), nil
}

func (d *DynamicHlsHelper) appendPlaylist(builder *strings.Builder, state *streaming.StreamState, url string, bitrate int, subtitleGroup string) *strings.Builder {
	playlistBuilder := &strings.Builder{}
	playlistBuilder.WriteString("#EXT-X-STREAM-INF:BANDWIDTH=")
	playlistBuilder.WriteString(strconv.Itoa(bitrate))
	playlistBuilder.WriteString(",AVERAGE-BANDWIDTH=")
	playlistBuilder.WriteString(strconv.Itoa(bitrate))

	d.appendPlaylistVideoRangeField(playlistBuilder, state)
	d.appendPlaylistCodecsField(playlistBuilder, state)
	d.appendPlaylistResolutionField(playlistBuilder, state)
	d.appendPlaylistFramerateField(playlistBuilder, state)

	if subtitleGroup != "" {
		playlistBuilder.WriteString(",SUBTITLES=\"")
		playlistBuilder.WriteString(subtitleGroup)
		playlistBuilder.WriteString("\"")
	}

	playlistBuilder.WriteString("\n")
	playlistBuilder.WriteString(url)
	builder.WriteString(playlistBuilder.String())

	return playlistBuilder
}

func (d *DynamicHlsHelper) appendPlaylistVideoRangeField(builder *strings.Builder, state *streaming.StreamState) {
	if state.VideoStream != nil && state.VideoStream.VideoRange != enums.VideoRangeUnknown {
		videoRange := state.VideoStream.VideoRange
		videoRangeType := state.VideoStream.VideoRangeType
		if mediaencoding.IsCopyCodec(state.OutputVideoCodec) {
			if videoRange == enums.VideoRangeSDR {
				builder.WriteString(",VIDEO-RANGE=SDR")
			}
			if videoRange == enums.VideoRangeHDR {
				if videoRangeType == enums.VideoRangeTypeHLG {
					builder.WriteString(",VIDEO-RANGE=HLG")
				} else {
					builder.WriteString(",VIDEO-RANGE=PQ")
				}
			}
		} else {
			// Currently we only encode to SDR.
			builder.WriteString(",VIDEO-RANGE=SDR")
		}
	}
}

func (d *DynamicHlsHelper) appendPlaylistCodecsField(builder *strings.Builder, state *streaming.StreamState) {
	// Video
	var videoCodecs string
	videoCodecLevel := d.getOutputVideoCodecLevel(state)
	if state.ActualOutputVideoCodec() != "" && videoCodecLevel != nil {
		videoCodecs = d.getPlaylistVideoCodecs(state, state.ActualOutputVideoCodec(), *videoCodecLevel)
	}

	// Audio
	var audioCodecs string
	if state.ActualOutputAudioCodec() != "" {
		audioCodecs = d.getPlaylistAudioCodecs(state)
	}

	var codecs strings.Builder
	if videoCodecs != "" {
		codecs.WriteString(videoCodecs)
	}

	if videoCodecs != "" && audioCodecs != "" {
		codecs.WriteString(",")
	}

	if audioCodecs != "" {
		codecs.WriteString(audioCodecs)
	}

	if codecs.Len() > 1 {
		builder.WriteString(",CODECS=\"")
		builder.WriteString(codecs.String())
		builder.WriteString("\"")
	}
}

func (d *DynamicHlsHelper) appendPlaylistResolutionField(builder *strings.Builder, state *streaming.StreamState) {
	if state.OutputWidth != nil && state.OutputHeight != nil {
		builder.WriteString(",RESOLUTION=")
		builder.WriteString(strconv.Itoa(*state.OutputWidth))
		builder.WriteString("x")
		builder.WriteString(strconv.Itoa(*state.OutputHeight()))
	}
}

// AppendPlaylistFramerateField appends a FRAME-RATE field containing the framerate of the output stream.
func (d *DynamicHlsHelper) appendPlaylistFramerateField(builder *strings.Builder, state *streaming.StreamState) {
	var framerate *float64
	if state.TargetFramerate() != nil {
		//framerate = FloatPtr(math.Round(*state.TargetFramerate(), 3))
		fr := math.Round(float64(*state.TargetFramerate()))
		framerate = &fr
	} else if state.VideoStream != nil && state.VideoStream.RealFrameRate != nil {
		//framerate = math.d(*state.VideoStream.RealFrameRate, 3))
		fr := math.Round(float64(*state.VideoStream.RealFrameRate))
		framerate = &fr
	}

	if framerate != nil {
		builder.WriteString(",FRAME-RATE=")
		builder.WriteString(strconv.FormatFloat(*framerate, 'f', 3, 64))
	}
}

func (d *DynamicHlsHelper) enableAdaptiveBitrateStreaming(state *streaming.StreamState, isLiveStream bool, enableAdaptiveBitrateStreaming bool /*, ipAddress net.IP, networkManager NetworkManager*/) bool {
	// Within the local network this will likely do more harm than good.
	/*
		if networkManager.IsInLocalNetwork(ipAddress) {
			return false
		}
	*/
	if !enableAdaptiveBitrateStreaming {
		return false
	}

	if isLiveStream || state.MediaPath == "" {
		// Opening live streams is so slow it's not even worth it
		return false
	}

	if mediaencoding.IsCopyCodec(state.OutputVideoCodec) {
		return false
	}

	if mediaencoding.IsCopyCodec(state.OutputAudioCodec) {
		return false
	}

	if !state.IsOutputVideo {
		return false
	}

	// Having problems in android
	return false
	// return state.VideoRequest.VideoBitRate != nil
}

func FloatPtr(f float64) *float64 {
	return &f
}

func (d *DynamicHlsHelper) addSubtitles(state *streaming.StreamState, subtitles []entities.MediaStream, builder *strings.Builder /*, user ClaimsPrincipal*/) {
	if state.SubtitleDeliveryMethod == dlna.Drop {
		return
	}

	var selectedIndex *int
	if state.SubtitleStream == nil || state.SubtitleDeliveryMethod != dlna.Hls {
		selectedIndex = nil
	} else {
		selectedIndex = &state.SubtitleStream.Index
	}

	const format = "#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"subs\",NAME=\"%s\",DEFAULT=%s,FORCED=%s,AUTOSELECT=YES,URI=\"%s\",LANGUAGE=\"%s\""

	for _, stream := range subtitles {
		name := stream.DisplayTitle

		var isDefault, isForced string
		if selectedIndex != nil && *selectedIndex == stream.Index {
			isDefault = "YES"
		} else {
			isDefault = "NO"
		}

		if stream.IsForced {
			isForced = "YES"
		} else {
			isForced = "NO"
		}

		//		url := fmt.Sprintf("%s/Subtitles/%d/subtitles.m3u8?SegmentLength=30&api_key=%s",
		//			state.Request.MediaSourceId, stream.Index, user.GetToken())
		url := fmt.Sprintf("%s/Subtitles/%d/subtitles.m3u8?SegmentLength=30",
			state.Request.MediaSourceID, stream.Index)

		line := fmt.Sprintf(format, name, isDefault, isForced, url, coalesce(&stream.Language, "Unknown"))
		builder.WriteString(line)
		builder.WriteString("\n")
	}
}

func coalesce(s *string, defaultValue string) string {
	if s != nil {
		return *s
	}
	return defaultValue
}

/*
func (d *DynamicHlsHelper) AddTrickplay(state StreamState, trickplayResolutions map[int]TrickplayInfo, builder *strings.Builder, user ClaimsPrincipal) {
	const playlistFormat = "#EXT-X-IMAGE-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"jpeg\",URI=\"%s\""

	for width, trickplayInfo := range trickplayResolutions {
		url := fmt.Sprintf("Trickplay/%d/tiles.m3u8?MediaSourceId=%s&api_key=%s",

		line := fmt.Sprintf(format, name, isDefault, isForced, url, coalesce(stream.Language, "Unknown"))
		builder.WriteString(line)
		builder.WriteString("\n")
	}
}

func coalesce(s *string, defaultValue string) string {
	if s != nil {
		return *s
	}
	return defaultValue
}

/*
func (d *DynamicHlsHelper) AddTrickplay(state StreamState, trickplayResolutions map[int]TrickplayInfo, builder *strings.Builder, user ClaimsPrincipal) {
	const playlistFormat = "#EXT-X-IMAGE-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"jpeg\",URI=\"%s\""

	for width, trickplayInfo := range trickplayResolutions {
		url := fmt.Sprintf("Trickplay/%d/tiles.m3u8?MediaSourceId=%s&api_key=%s",
			width, state.Request.MediaSourceId, user.GetToken())

		line := fmt.Sprintf(playlistFormat,
			trickplayInfo.Bandwidth,
			trickplayInfo.Width,
			trickplayInfo.Height,
			url)

		builder.WriteString(line)
		builder.WriteString("\n")
	}
}
*/

func (d *DynamicHlsHelper) getOutputVideoCodecLevel(state *streaming.StreamState) *int {
	var levelString string
	if mediaencoding.IsCopyCodec(state.OutputVideoCodec) && state.VideoStream != nil && state.VideoStream.Level != nil {
		levelString = fmt.Sprintf("%d", *state.VideoStream.Level)
	} else {
		switch state.ActualOutputVideoCodec() {
		case "h264":
			levelString = state.GetRequestedLevel("h264")
			if levelString == "" {
				levelString = "41"
			}
			levelString = mediaencoding.NormalizeTranscodingLevel(&state.EncodingJobInfo, levelString)
		case "h265", "hevc":
			levelString = state.GetRequestedLevel("h265")
			if levelString == "" {
				levelString = state.GetRequestedLevel("hevc")
			}
			if levelString == "" {
				levelString = "120"
			}
			levelString = mediaencoding.NormalizeTranscodingLevel(&state.EncodingJobInfo, levelString)
		case "av1":
			levelString = state.GetRequestedLevel("av1")
			if levelString == "" {
				levelString = "19"
			}
			levelString = mediaencoding.NormalizeTranscodingLevel(&state.EncodingJobInfo, levelString)
		}
	}

	var parsedLevel int
	if _, err := fmt.Sscanf(levelString, "%d", &parsedLevel); err == nil {
		return &parsedLevel
	}
	return nil
}

func (d *DynamicHlsHelper) getOutputVideoCodecProfile(state *streaming.StreamState, codec string) string {
	var profileString string
	if mediaencoding.IsCopyCodec(state.OutputVideoCodec) && state.VideoStream != nil && state.VideoStream.Profile != "" {
		profileString = state.VideoStream.Profile
	} else if codec != "" {
		profiles := state.GetRequestedProfiles(codec)
		if len(profiles) > 0 {
			profileString = profiles[0]
		}
		if state.ActualOutputVideoCodec() == "h264" {
			if profileString == "" {
				profileString = "high"
			}
		} else if state.ActualOutputVideoCodec() == "h265" || state.ActualOutputVideoCodec() == "hevc" || state.ActualOutputVideoCodec() == "av1" {
			if profileString == "" {
				profileString = "main"
			}
		}
	}
	return profileString
}

func (d *DynamicHlsHelper) getPlaylistAudioCodecs(state *streaming.StreamState) string {
	switch state.ActualOutputAudioCodec() {
	case "aac":
		profile := state.GetRequestedProfiles("aac")
		if len(profile) > 0 {
			return GetAACString(profile[0])
		}
	case "mp3":
		return GetMP3String()
	case "ac3":
		return GetAC3String()
	case "eac3":
		return GetEAC3String()
	case "flac":
		return GetFLACString()
	case "alac":
		return GetALACString()
	case "opus":
		return GetOPUSString()
	}
	return ""
}

func (d *DynamicHlsHelper) getPlaylistVideoCodecs(state *streaming.StreamState, codec string, level int) string {
	if level == 0 {
		// This is 0 when there's no requested level in the device profile
		// and the source is not encoded in H.26X or AV1
		d.logger.Error("Got invalid level when building CODECS field for HLS master playlist")
		return ""
	}

	switch codec {
	case "h264":
		profile := d.getOutputVideoCodecProfile(state, "h264")
		return GetH264String(profile, level)
	case "h265", "hevc":
		profile := d.getOutputVideoCodecProfile(state, "hevc")
		return GetH265String(profile, level)
	case "av1":
		profile := d.getOutputVideoCodecProfile(state, "av1")

		// Currently we only transcode to 8 bits AV1
		bitDepth := 8
		if mediaencoding.IsCopyCodec(state.OutputVideoCodec) && state.VideoStream != nil && state.VideoStream.BitDepth != nil {
			bitDepth = *state.VideoStream.BitDepth
		}

		return GetAv1String(profile, level, false, bitDepth)
	}

	return ""
}

func (d *DynamicHlsHelper) getBitrateVariation(bitrate int) int {
	// By default, vary by just 50k
	variation := 50000

	switch {
	case bitrate >= 10000000:
		variation = 2000000
	case bitrate >= 5000000:
		variation = 1500000
	case bitrate >= 3000000:
		variation = 1000000
	case bitrate >= 2000000:
		variation = 500000
	case bitrate >= 1000000:
		variation = 300000
	case bitrate >= 600000:
		variation = 200000
	case bitrate >= 400000:
		variation = 100000
	}

	return variation
}

func (d *DynamicHlsHelper) replaceVideoBitrate(url string, oldValue, newValue int) string {
	return strings.ReplaceAll(
		url,
		fmt.Sprintf("videobitrate=%d", oldValue),
		fmt.Sprintf("videobitrate=%d", newValue),
	)
}

func (d *DynamicHlsHelper) replaceProfile(url, codec, oldValue, newValue string) string {
	profileStr := fmt.Sprintf("%s-profile=", codec)
	return strings.ReplaceAll(
		url,
		profileStr+oldValue,
		profileStr+newValue,
	)
}

func (d *DynamicHlsHelper) replacePlaylistCodecsField(playlist, oldValue, newValue *strings.Builder) string {
	return strings.ReplaceAll(
		playlist.String(),
		oldValue.String(),
		newValue.String(),
	)
}
