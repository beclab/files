package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"files/pkg/common"
	"files/pkg/models"

	cc "files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/controller/mediaencoding/transcodemanager"
	"files/pkg/media/mediabrowser/controller/streaming"
	"files/pkg/media/mediabrowser/mediaencoding/encoder"

	"files/pkg/media/mediabrowser/model/configuration"
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/entities"
	ioo "files/pkg/media/mediabrowser/model/io"

	"files/pkg/media/api/helpers"
	"files/pkg/media/api/models/streamingdtos"
	"files/pkg/media/utils"
	"files/pkg/media/utils/version"

	"files/pkg/media/jellyfin/mediaencoding/hls/playlist"
)

const (
	DefaultVodEncoderPreset   = entities.VeryFast
	DefaultEventEncoderPreset = entities.SuperFast
	transcodingJobType        = mediaencoding.Hls
	prefix                    = "/Seahub"
	prefixExternal            = "/External"
)

var (
	minFFmpegFlacInMp4        = version.Version{6, 0, 0}
	minFFmpegX265BframeInFmp4 = version.Version{7, 0, 1}
)

type DynamicHlsController struct {
	logger                      *utils.Logger
	mediaEncoder                mediaencoding.IMediaEncoder
	encodingHelper              mediaencoding.EncodingHelper
	transcodeManager            transcodemanager.ITranscodeManager
	fileSystem                  ioo.IFileSystem
	dynamicHlsPlaylistGenerator playlist.IDynamicHlsPlaylistGenerator
	encodingOptions             *configuration.EncodingOptions
	serverConfigurationManager  cc.IServerConfigurationManager
	dynamicHlsHelper            *helpers.DynamicHlsHelper
}

func NewDynamicHlsController(logger *utils.Logger, mediaEncoder mediaencoding.IMediaEncoder, transcodeManager transcodemanager.ITranscodeManager, encodingHelper mediaencoding.EncodingHelper, fileSystem ioo.IFileSystem, dynamicHlsPlaylistGenerator playlist.IDynamicHlsPlaylistGenerator, serverConfigurationManager cc.IServerConfigurationManager, dynamicHlsHelper *helpers.DynamicHlsHelper) *DynamicHlsController {
	return &DynamicHlsController{
		logger:                      logger,
		mediaEncoder:                mediaEncoder,
		transcodeManager:            transcodeManager,
		encodingHelper:              encodingHelper,
		fileSystem:                  fileSystem,
		dynamicHlsPlaylistGenerator: dynamicHlsPlaylistGenerator,
		serverConfigurationManager:  serverConfigurationManager,
		encodingOptions:             serverConfigurationManager.GetEncodingOptions(),
		dynamicHlsHelper:            dynamicHlsHelper,
	}
}

func (d *DynamicHlsController) GetMasterHlsVideoPlaylist(w http.ResponseWriter, r *http.Request) {
	var err error
	vars := mux.Vars(r)
	fmt.Println(vars)
	itemId, err := uuid.Parse(vars["itemId"])
	if err != nil {
		d.logger.Warnf("itemId: %v\n", err)
	} else {
		d.logger.Debugf("itemId: %v\n", itemId)
	}

	dataDir := os.Getenv("MEDIA_SERVER_DATA_DIR")
	playPath := r.URL.Query().Get("PlayPath")
	playPath = filepath.Clean(playPath)
	if filepath.IsAbs(playPath) {
		playPath = dataDir + playPath
		fmt.Println(playPath)
	} else {
		http.Error(w, "invalid PlayPath", http.StatusBadRequest)
		return
	}

	static := r.URL.Query().Get("static")
	params := r.URL.Query().Get("params")
	tag := r.URL.Query().Get("tag")
	deviceProfileId := r.URL.Query().Get("deviceProfileId")
	fmt.Println(deviceProfileId)
	playSessionId := r.URL.Query().Get("PlaySessionId")
	segmentContainer := r.URL.Query().Get("SegmentContainer")
	segmentLength, _ := strconv.Atoi(r.URL.Query().Get("SegmentLength"))
	minSegments, _ := strconv.Atoi(r.URL.Query().Get("MinSegments"))
	mediaSourceId := r.URL.Query().Get("MediaSourceId")
	deviceId := r.URL.Query().Get("DeviceId")
	audioCodec := r.URL.Query().Get("AudioCodec")
	enableAutoStreamCopy := r.URL.Query().Get("EnableAutoStreamCopy") == "true"
	allowVideoStreamCopy := r.URL.Query().Get("AllowVideoStreamCopy") == "true"
	allowAudioStreamCopy := r.URL.Query().Get("AllowAudioStreamCopy") == "true"
	breakOnNonKeyFrames := r.URL.Query().Get("BreakOnNonKeyFrames") == "true"
	audioSampleRate, _ := strconv.Atoi(r.URL.Query().Get("AudioSampleRate"))
	maxAudioBitDepth, _ := strconv.Atoi(r.URL.Query().Get("MaxAudioBitDepth"))
	audioBitRate, _ := strconv.Atoi(r.URL.Query().Get("AudioBitrate"))
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
	copyTimestamps := r.URL.Query().Get("CopyTimestamps") == "true"
	startTimeTicks, _ := strconv.ParseInt(r.URL.Query().Get("StartTimeTicks"), 10, 64)
	var width *int
	if _, ok := r.URL.Query()["Width"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("Width"))
		if err != nil {
			width = &tmp
		}
	}
	var height *int
	if _, ok := r.URL.Query()["height"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("height"))
		if err != nil {
			height = &tmp
		}
	}
	var maxWidth *int
	if _, ok := r.URL.Query()["MaxWidth"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("MaxWidth"))
		if err != nil {
			maxWidth = &tmp
		}
	}
	var maxHeight *int
	if _, ok := r.URL.Query()["MaxHeight"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("MaxHeight"))
		if err != nil {
			maxHeight = &tmp
		}
	}
	videoBitRate, _ := strconv.Atoi(r.URL.Query().Get("VideoBitrate"))
	subtitleStreamIndex, _ := strconv.Atoi(r.URL.Query().Get("SubtitleStreamIndex"))
	//subtitleMethod, _ := strconv.Atoi(r.URL.Query().Get("SubtitleMethod"))
	subtitleMethod, _ := dlna.ParseSubtitleDeliveryMethod(r.URL.Query().Get("SubtitleMethod"))
	fmt.Println(r.URL.Query().Get("SubtitleMethod"), "-----------------------9526--------------------------------------", subtitleMethod)
	maxRefFrames, _ := strconv.Atoi(r.URL.Query().Get("MaxRefFrames"))
	maxVideoBitDepth, _ := strconv.Atoi(r.URL.Query().Get("MaxVideoBitDepth"))
	requireAvc := r.URL.Query().Get("RequireAvc") == "true"
	deInterlace := r.URL.Query().Get("DeInterlace") == "true"
	requireNonAnamorphic := r.URL.Query().Get("RequireNonAnamorphic") == "true"
	transcodingMaxAudioChannels, _ := strconv.Atoi(r.URL.Query().Get("TranscodingMaxAudioChannels"))
	cpuCoreLimit, _ := strconv.Atoi(r.URL.Query().Get("CpuCoreLimit"))
	liveStreamId := r.URL.Query().Get("LiveStreamId")
	enableMpegtsM2TsMode := r.URL.Query().Get("EnableMpegtsM2TsMode") == "true"
	videoCodec := r.URL.Query().Get("VideoCodec")
	validationRegex := regexp.MustCompile(mediaencoding.ValidationRegex)
	if videoCodec != "" {
		matched := validationRegex.MatchString(videoCodec)
		if !matched {
			fmt.Println("videcodec not match")
		}
	}

	subtitleCodec := r.URL.Query().Get("SubtitleCodec")
	if subtitleCodec != "" {
		matched := validationRegex.MatchString(subtitleCodec)
		if !matched {
			fmt.Println("subtitlecodec not match")
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
	enableAdaptiveBitrateStreaming := r.URL.Query().Get("EnableAdaptiveBitrateStreaming") != "false"
	enableTrickplay := r.URL.Query().Get("EnableTrickplay") != "false"

	streamingRequest := &streamingdtos.HlsVideoRequestDto{
		VideoRequestDto: &streaming.VideoRequestDto{
			StreamingRequestDto: &streaming.StreamingRequestDto{
				BaseEncodingJobOptions: &mediaencoding.BaseEncodingJobOptions{
					PlayPath: playPath,
					Id:       itemId,
					Static:   static == "true",
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
					Width:                       width,
					Height:                      height,
					MaxWidth:                    maxWidth,
					MaxHeight:                   maxHeight,
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
			EnableTrickplay: enableTrickplay,
		},
		EnableAdaptiveBitrateStreaming: enableAdaptiveBitrateStreaming,
	}

	playlist, err := d.dynamicHlsHelper.GetMasterHlsPlaylist(r, transcodingJobType, streamingRequest, enableAdaptiveBitrateStreaming)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	_, err = w.Write([]byte(playlist))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type StreamOptions map[string]string

func GetFromQuery(r *http.Request, dest interface{}) error {
	if v, ok := dest.(*StreamOptions); ok {
		*v = make(StreamOptions)
		for key, values := range r.URL.Query() {
			(*v)[key] = values[0]
		}
		return nil
	}
	return &json.UnmarshalTypeError{
		Value: "map[string]string",
		Type:  destType(dest),
	}
}

func destType(dest interface{}) reflect.Type {
	if t := reflect.TypeOf(dest); t.Kind() == reflect.Ptr {
		return t.Elem()
	} else {
		return t
	}
}
func pathCommon(playPath, bflName string) (string, error) {
	log.Printf("pathCommon: %v %v", playPath, bflName)
	if utils.IsTestEnv() {
		return playPath, nil
	}
	fileParam, err := models.CreateFileParam(bflName, playPath)
	if err != nil {
		log.Printf("parse url error: %v\n", err)
		return "", errors.New("parse url error")
	}

	formalizedPath := ""
	if fileParam.FileType == common.Sync {
		seafileServiceName := os.Getenv("SEAFILE_SERVICE")
		formalizedPath = "http://" + seafileServiceName + fileParam.Path
	} else if fileParam.FileType == common.GoogleDrive {
		formalizedPath = "https://www.googleapis.com/drive/v3/files" + fileParam.Path + "?alt=media"
	} else if fileParam.FileType == common.AwsS3 {
		authToken, err := utils.GetAuthToken(bflName)
		if err != nil {
			fmt.Println(err)
			return "", err
		}
		fmt.Println(authToken)
		accountResp, err := utils.GetToken(bflName, fileParam.Extend, fileParam.FileType, authToken)
		if err != nil {
			return "", err
		}

		s3URL := accountResp.RawData.Endpoint
		re := regexp.MustCompile(`https://([a-zA-Z0-9\-]+)\.s3\.([a-z0-9\-]+)\.amazonaws\.com/?`)
		matches := re.FindStringSubmatch(s3URL)
		if len(matches) != 3 {
			return "", fmt.Errorf("invalid S3 URL format: %s", s3URL)
		}
		bucket := matches[1]
		region := matches[2]

		// Load credentials (from env, shared creds file, or static)
		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accountResp.Name, accountResp.RawData.AccessToken, "")),
		)
		if err != nil {
			log.Println(err)
			return "", err
		}

		// Create S3 client
		client := s3.NewFromConfig(cfg)

		// Create a presign client
		presignClient := s3.NewPresignClient(client)
		// Generate pre-signed URL for GET (download)

		presignResult, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(strings.TrimPrefix(fileParam.Path, "/")),
		}, s3.WithPresignExpires(3600*time.Second)) // Expires in 1 hour
		if err != nil {
			log.Printf("Failed to generate pre-signed URL: %v", err)
			return "", err
		}

		fmt.Println("Pre-signed URL for download:", presignResult.URL)

		formalizedPath = presignResult.URL

	} else if fileParam.FileType == common.DropBox {
		formalizedPath = "https://content.dropboxapi.com/2/files/download?arg="
		type DropboxAPIArg struct {
			Path string `json:"path"`
		}
		dropboxApiArg := DropboxAPIArg{Path: fileParam.Path}
		jsonData, err := json.Marshal(dropboxApiArg)
		if err != nil {
			fmt.Println("marshal error:", err)
			return "", err
		}
		formalizedPath += url.QueryEscape(string(jsonData))
	} else {
		/*
			playPath = filepath.Clean(fileParam.Path)
			if !filepath.IsAbs(playPath) {
				fmt.Println("invalid PlayPath")
				http.Error(w, "invalid PlayPath", http.StatusBadRequest)
				return
			}
		*/

		resUri, err := fileParam.GetResourceUri()
		if err != nil {
			log.Printf("get path error %v\n", err)
			return "", errors.New("get path error")
		}
		formalizedPath = resUri + "/" + filepath.Clean(fileParam.Path)

	}

	log.Printf("pathCommon: %v", formalizedPath)

	return formalizedPath, nil
}

func (d *DynamicHlsController) GetHlsVideoSegment(w http.ResponseWriter, r *http.Request) {
	var err error
	vars := mux.Vars(r)
	fmt.Println(vars)
	itemId, err := uuid.Parse(vars["itemId"])
	if err != nil {
		d.logger.Warnf("itemId: %v\n", err)
	} else {
		d.logger.Debugf("itemId: %v\n", itemId)
	}
	playlistId := vars["playlistId"]
	fmt.Println(playlistId)
	segmentId, _ := strconv.Atoi(vars["segmentId"])
	container := vars["container"]

	/*
		// Extract parameters from the request
		playPath := r.URL.Query().Get("PlayPath")
		if strings.HasPrefix(playPath, prefix) {
			seafileServiceName := os.Getenv("SEAFILE_SERVICE")
			playPath = "http://" + seafileServiceName + playPath[len(prefix):]
		} else if strings.HasPrefix(playPath, prefixExternal) {
			dataDir := os.Getenv("MEDIA_SERVER_DATA_DIR")
			playPath = dataDir + playPath
		} else {
			playPath = filepath.Clean(playPath)
			if !filepath.IsAbs(playPath) {
				fmt.Println("invalid PlayPath")
				http.Error(w, "invalid PlayPath", http.StatusBadRequest)
				return
			}

			if _, ok := r.Header[utils.BFL_HEADER]; !ok {
				log.Printf("%s does not exist\n", utils.BFL_HEADER)
				http.Error(w, "bfl header not exist", http.StatusBadRequest)
				return
			}

			bflNames := r.Header[utils.BFL_HEADER]

			playPath, err = utils.GetEffectPlayPath(playPath, bflNames[0])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		fmt.Println(playPath)
	*/

	if _, ok := r.Header[common.REQUEST_HEADER_OWNER]; !ok {
		log.Printf("%s does not exist\n", common.REQUEST_HEADER_OWNER)
		http.Error(w, "bfl header not exist", http.StatusBadRequest)
		return
	}

	bflName := r.Header[common.REQUEST_HEADER_OWNER][0]
	playPath, err := pathCommon(r.URL.Query().Get("PlayPath"), bflName)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
		return
	}

	runtimeTicks, _ := strconv.ParseInt(r.URL.Query().Get("runtimeTicks"), 10, 64)
	actualSegmentLengthTicks, _ := strconv.ParseInt(r.URL.Query().Get("actualSegmentLengthTicks"), 10, 64)
	static, _ := strconv.ParseBool(r.URL.Query().Get("Static"))
	params := r.URL.Query().Get("Params")
	tag := r.URL.Query().Get("Tag")
	//deviceProfileId := r.URL.Query().Get("deviceProfileId")
	playSessionId := r.URL.Query().Get("PlaySessionId")
	var segmentContainer *string
	if tmp := r.URL.Query().Get("SegmentContainer"); tmp != "" {
		segmentContainer = &tmp
	}

	var segmentLength *int
	if _, ok := r.URL.Query()["SegmentLength"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("SegmentLength"))
		if err != nil {
			segmentLength = &tmp
		}
	}
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
	audioBitRate, _ := strconv.Atoi(r.URL.Query().Get("AudioBitrate"))
	var audioChannels *int
	if _, ok := r.URL.Query()["AudioChannels"]; ok {
		tmp, _ := strconv.Atoi(r.URL.Query().Get("AudioChannels"))
		if err != nil {
			audioChannels = &tmp
		}

	}
	var maxAudioChannels *int
	if _, ok := r.URL.Query()["MaxAudioChannels"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("MaxAudioChannels"))
		if err != nil {
			maxAudioChannels = &tmp
		}
	}
	profile := r.URL.Query().Get("Profile")
	level := r.URL.Query().Get("Level")
	var framerate *float32
	if _, ok := r.URL.Query()["Framerate"]; ok {
		f64, err := strconv.ParseFloat(r.URL.Query().Get("Framerate"), 32)
		if err != nil {
			f32 := float32(f64)
			framerate = &f32
		}
	}

	var maxFramerate *float32
	if _, ok := r.URL.Query()["MaxFramerate"]; ok {
		f64, err := strconv.ParseFloat(r.URL.Query().Get("MaxFramerate"), 32)
		if err != nil {
			f32 := float32(f64)
			maxFramerate = &f32
		}
	}
	copyTimestamps, _ := strconv.ParseBool(r.URL.Query().Get("CopyTimestamps"))
	startTimeTicks, _ := strconv.ParseInt(r.URL.Query().Get("StartTimeTicks"), 10, 64)
	var width *int
	if _, ok := r.URL.Query()["Width"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("Width"))
		if err != nil {
			width = &tmp
		}
	}
	var height *int
	if _, ok := r.URL.Query()["Height"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("Height"))
		if err != nil {
			height = &tmp
		}
	}
	var maxWidth *int
	if _, ok := r.URL.Query()["MaxWidth"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("MaxWidth"))
		if err != nil {
			maxWidth = &tmp
		}
	}
	var maxHeight *int
	if _, ok := r.URL.Query()["MaxHeight"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("MaxHeight"))
		if err != nil {
			maxHeight = &tmp
		}
	}

	videoBitRate, _ := strconv.Atoi(r.URL.Query().Get("VideoBitrate"))
	subtitleStreamIndex, _ := strconv.Atoi(r.URL.Query().Get("SubtitleStreamIndex"))
	//	subtitleMethod, _ := strconv.Atoi(r.URL.Query().Get("SubtitleMethod"))
	subtitleMethod, _ := dlna.ParseSubtitleDeliveryMethod(r.URL.Query().Get("SubtitleMethod"))
	fmt.Println(r.URL.Query().Get("SubtitleMethod"), "-----------------------9528--------------------------------------", subtitleMethod)
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
	subtitleCodec := r.URL.Query().Get("SubtitleCodec")
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

	var streamOptions StreamOptions
	if err := GetFromQuery(r, &streamOptions); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	streamingRequest := &streaming.VideoRequestDto{
		StreamingRequestDto: &streaming.StreamingRequestDto{
			BaseEncodingJobOptions: &mediaencoding.BaseEncodingJobOptions{
				PlayPath:                    playPath,
				Id:                          itemId,
				Container:                   container,
				Static:                      static,
				MediaSourceID:               mediaSourceId,
				DeviceID:                    deviceId,
				AudioCodec:                  audioCodec,
				EnableAutoStreamCopy:        enableAutoStreamCopy,
				AllowAudioStreamCopy:        allowAudioStreamCopy,
				AllowVideoStreamCopy:        allowVideoStreamCopy,
				BreakOnNonKeyFrames:         breakOnNonKeyFrames,
				AudioSampleRate:             &audioSampleRate,
				MaxAudioChannels:            maxAudioChannels,
				AudioBitRate:                &audioBitRate,
				MaxAudioBitDepth:            &maxAudioBitDepth,
				AudioChannels:               audioChannels,
				Profile:                     profile,
				Level:                       level,
				Framerate:                   framerate,
				MaxFramerate:                maxFramerate,
				CopyTimestamps:              copyTimestamps,
				StartTimeTicks:              &startTimeTicks,
				Width:                       width,
				Height:                      height,
				MaxWidth:                    maxWidth,
				MaxHeight:                   maxHeight,
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
			},
			Params:                   &params,
			Tag:                      &tag,
			PlaySessionID:            &playSessionId,
			SegmentContainer:         segmentContainer,
			SegmentLength:            segmentLength,
			MinSegments:              &minSegments,
			CurrentRuntimeTicks:      runtimeTicks,
			ActualSegmentLengthTicks: actualSegmentLengthTicks,
		},
		HasFixedResolution:        false,
		EnableSubtitlesInManifest: false,
		EnableTrickplay:           false,
	}

	result, err := d.GetDynamicSegment(r, streamingRequest, segmentId)
	if err != nil {
		fmt.Println(err)
		return
	}
	/*
			fmt.Println(result)
		    	w.Header().Set("Content-Type", "application/x-mpegURL")
		    	_, _ = w.Write([]byte(result))
	*/
	result.ServeHTTP(w, r)
}

// func GetDynamicSegment(w http.ResponseWriter, r *http.Request, streamingRequest StreamingRequestDto, segmentId int) error {
// func GetDynamicSegment(r *http.Request, streamingRequest streaming.StreamingRequestDto, segmentId int) (string, error) {
func (d *DynamicHlsController) GetDynamicSegment(r *http.Request, request interface{}, segmentId int) (http.Handler, error) {
	streamingRequest, _ := request.(*streaming.VideoRequestDto)
	if *streamingRequest.StartTimeTicks > 0 {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), errors.New("StartTimeTicks is not allowed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, "userId", "root")
	defer cancel()

	state, _ := helpers.GetStreamingState(
		request,
		//streamingRequest,
		r,
		//		_mediaSourceManager,
		//		_userManager,
		//		_libraryManager,
		d.serverConfigurationManager,
		d.mediaEncoder,
		d.encodingHelper,
		d.transcodeManager,
		transcodingJobType,
		ctx,
	)
	fmt.Printf("state: %+v\n", state)
	fmt.Printf("state BaseRequest: %+v\n", state.BaseRequest)
	fmt.Println("______________________________")
	fmt.Printf("SegmentContainer: %s\n", state.Request.SegmentContainer)

	playlistPath := d.changeExtension(state.OutputFilePath, ".m3u8")
	fmt.Println(state.OutputFilePath, playlistPath, segmentId)

	segmentPath, _ := d.getSegmentPath(state, playlistPath, segmentId)
	segmentExtension := mediaencoding.GetSegmentFileExtension(state.Request.SegmentContainer)
	var job *mediaencoding.TranscodingJob
	if _, err := os.Stat(segmentPath); err == nil {
		job = d.transcodeManager.OnTranscodeBeginRequest(playlistPath, transcodingJobType)
		fmt.Printf("returning %s [it exists, try 1]\n", segmentPath)
		return d.GetSegmentResult(state, playlistPath, segmentPath, segmentExtension, segmentId, job, ctx)
	}

	// Acquire lock on the playlist path
	unlock, err := d.transcodeManager.LockAsync(playlistPath, ctx)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), err
	}
	defer unlock()

	if _, err := os.Stat(segmentPath); err == nil {
		job = d.transcodeManager.OnTranscodeBeginRequest(playlistPath, transcodingJobType)
		fmt.Printf("returning %s [it exists, try 2]\n", segmentPath)
		return d.GetSegmentResult(state, playlistPath, segmentPath, segmentExtension, segmentId, job, ctx)
	}

	currentTranscodingIndex := d.GetCurrentTranscodingIndex(playlistPath, segmentExtension)
	fmt.Println("########################### >>", currentTranscodingIndex)
	segmentGapRequiringTranscodingChange := 24 / state.SegmentLength()
	fmt.Println(state.SegmentLength)
	fmt.Println("segmentlength.......................................................")
	/*
		segmentGapRequiringTranscodingChange := 24 / 3
	*/

	var startTranscoding = false
	if segmentId == -1 {
		fmt.Println("Starting transcoding because fmp4 init file is being requested")
		startTranscoding = true
		segmentId = 0
	} else if currentTranscodingIndex == nil {
		fmt.Println("Starting transcoding because currentTranscodingIndex=null")
		startTranscoding = true
	} else if segmentId < *currentTranscodingIndex {
		fmt.Printf("Starting transcoding because requestedIndex=%d and currentTranscodingIndex=%d\n", segmentId, *currentTranscodingIndex)
		startTranscoding = true

	} else if segmentId-*currentTranscodingIndex > segmentGapRequiringTranscodingChange {
		fmt.Printf("Starting transcoding because segmentGap is %d and max allowed gap is %d. requestedIndex=%d\n", segmentId-*currentTranscodingIndex, segmentGapRequiringTranscodingChange, segmentId)
		startTranscoding = true
	}

	if startTranscoding {
		// Kill existing transcoding jobs
		err = d.transcodeManager.KillTranscodingJobs(streamingRequest.DeviceID, *streamingRequest.PlaySessionID, func(job string) bool {
			return false
		})
		if err != nil {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), err
		}

		if currentTranscodingIndex != nil {
			err = d.DeleteLastFile(playlistPath, segmentExtension, 0)
			if err != nil {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), err
			}
		}

		streamingRequest.StartTimeTicks = &streamingRequest.CurrentRuntimeTicks

		state.WaitForPath = segmentPath
		_, err := d.transcodeManager.StartFfMpeg(
			*state,
			playlistPath,
			d.GetCommandLineArguments(playlistPath, state, false, segmentId),
			ctx.Value("userId").(string),
			transcodingJobType,
			ctx,
			".",
		)
		if err != nil {
			state.Dispose()
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), err
		}
	} else {
		job = d.transcodeManager.OnTranscodeBeginRequest(playlistPath, transcodingJobType)
		if job.TranscodingThrottler != nil {
			err = job.TranscodingThrottler.UnpauseTranscoding()
			if err != nil {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), err
			}
		}
	}

	fmt.Printf("returning %s [general case]\n", segmentPath)
	if job == nil {
		job = d.transcodeManager.OnTranscodeBeginRequest(playlistPath, transcodingJobType)
	}
	return d.GetSegmentResult(state, playlistPath, segmentPath, segmentExtension, segmentId, job, ctx)
}

func (d *DynamicHlsController) GetSegmentResult(
	state *streaming.StreamState,
	playlistPath, segmentPath, segmentExtension string,
	segmentIndex int,
	transcodingJob *mediaencoding.TranscodingJob,
	ctx context.Context,
) (http.Handler, error) {
	segmentExists := fileExists(segmentPath)
	if segmentExists {
		if transcodingJob != nil && transcodingJob.HasExited {
			d.logger.Debugf("serving up %s as transcode is over", segmentPath)
			return d.getSegmentResult(state, segmentPath, transcodingJob)
		}

		currentTranscodingIndex := d.GetCurrentTranscodingIndex(playlistPath, segmentExtension)
		fmt.Println(currentTranscodingIndex, "|||||||||||||||||||||||||||||||||||||||||||||||||||||", segmentIndex)
		if currentTranscodingIndex != nil {
			fmt.Println(*currentTranscodingIndex, segmentIndex)
		}
		if currentTranscodingIndex != nil && segmentIndex < *currentTranscodingIndex {
			d.logger.Debugf("serving up %s as transcode index %d is past requested point %d", segmentPath, *currentTranscodingIndex, segmentIndex)
			return d.getSegmentResult(state, segmentPath, transcodingJob)
		}
	}

	nextSegmentPath, _ := d.getSegmentPath(state, playlistPath, segmentIndex+1)
	if transcodingJob != nil {
		for !transcodingJob.HasExited {
			if segmentExists {
				if transcodingJob.HasExited || fileExists(nextSegmentPath) {
					d.logger.Debugf("Serving up %s as it deemed ready", segmentPath)
					return d.getSegmentResult(state, segmentPath, transcodingJob)
				}
			} else {
				segmentExists = fileExists(segmentPath)
				if segmentExists {
					continue // avoid unnecessary waiting if segment just became available
				}
			}

			select {
			case <-ctx.Done():
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}

		if !fileExists(segmentPath) {
			d.logger.Warnf("cannot serve %s as transcoding quit before we got there", segmentPath)
		} else {
			d.logger.Debugf("serving %s as it's on disk and transcoding stopped", segmentPath)
		}

		select {
		case <-ctx.Done():
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), ctx.Err()
		default:
			// your normal code
		}
	} else {
		d.logger.Warnf("cannot serve %s as it doesn't exist and no transcode is running", segmentPath)
	}

	return d.getSegmentResult(state, segmentPath, transcodingJob)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (d *DynamicHlsController) getSegmentResult(state *streaming.StreamState, segmentPath string, transcodingJob *mediaencoding.TranscodingJob) (http.Handler, error) {
	segmentEndingPositionTicks := state.Request.CurrentRuntimeTicks + state.Request.ActualSegmentLengthTicks
	fmt.Println("TICKSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSS")
	fmt.Println(state.Request.ActualSegmentLengthTicks)
	fmt.Println(state.Request.CurrentRuntimeTicks)
	fmt.Println(segmentEndingPositionTicks)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			//logger.Debugf("Finished serving %s", segmentPath)
			fmt.Printf("Finished serving %s\n", segmentPath)
			if transcodingJob != nil {
				if transcodingJob.DownloadPositionTicks == nil {
					transcodingJob.DownloadPositionTicks = &segmentEndingPositionTicks
				} else {
					fmt.Println(float64(*transcodingJob.DownloadPositionTicks), float64(segmentEndingPositionTicks))
					*transcodingJob.DownloadPositionTicks = int64(math.Max(float64(*transcodingJob.DownloadPositionTicks), float64(segmentEndingPositionTicks)))
				}
				d.transcodeManager.OnTranscodeEndRequest(transcodingJob)
			}
		}()

		fmt.Println("serve file ---------------》", segmentPath)
		http.ServeFile(w, r, segmentPath)
	}), nil
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (d *DynamicHlsController) GetCommandLineArguments(outputPath string, state *streaming.StreamState, isEventPlaylist bool, startNumber int) string {

	//	state.OutputVideoCodec = "h264"
	fmt.Println("ooooooooooooooooooooooooooooooo")
	fmt.Printf("%+v\n", *state)
	fmt.Println("qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq")
	fmt.Printf("%+v\n", state.EncodingJobInfo)
	state.IsOutputVideo = true
	//	videoCodec := d.encodingHelper.GetVideoEncoder(&state.EncodingJobInfo, d.encodingOptions)
	threads := mediaencoding.GetNumberOfThreads(&state.EncodingJobInfo, *d.encodingOptions, nil)

	if state.BaseRequest.BreakOnNonKeyFrames {
		//_logger.LogInformation("Current HLS implementation doesn't support non-keyframe breaks but one is requested, ignoring that request");
		fmt.Println("Current HLS implementation doesn't support non-keyframe breaks but one is requested, ignoring that request")
		state.BaseRequest.BreakOnNonKeyFrames = false
	}

	var mapArgs string
	/*
		state.EncodingJobInfo.VideoStream = &entities.MediaStream{}
		var channels int = 2
		state.EncodingJobInfo.AudioStream = &entities.MediaStream{
			Channels: &channels,
		}
	*/
	if state.IsOutputVideo {
		mapArgs = mediaencoding.GetMapArgs(&state.EncodingJobInfo)
	}

	directory := filepath.Dir(outputPath)

	//outputFileNameWithoutExtension := filepath.Base(outputPath)
	fileName := filepath.Base(outputPath)
	outputFileNameWithoutExtension := filepath.Base(fileName[:len(fileName)-len(filepath.Ext(outputPath))])
	fmt.Println(outputFileNameWithoutExtension)
	outputPrefix := filepath.Join(directory, outputFileNameWithoutExtension)
	outputExtension := mediaencoding.GetSegmentFileExtension(state.Request.SegmentContainer)
	outputTsArg := fmt.Sprintf("%s%%d%s", outputPrefix, outputExtension)

	var segmentFormat string
	segmentContainer := strings.TrimPrefix(outputExtension, ".")
	inputModifier := d.encodingHelper.GetInputModifier(&state.EncodingJobInfo, d.encodingOptions, &segmentContainer)

	if strings.EqualFold(segmentContainer, "ts") {
		segmentFormat = "mpegts"
	} else if strings.EqualFold(segmentContainer, "mp4") {
		var outputFmp4HeaderArg string
		if runtime.GOOS == "windows" {
			outputFmp4HeaderArg = " -hls_fmp4_init_filename " + outputPrefix + "-1" + outputExtension
		} else {
			outputFmp4HeaderArg = " -hls_fmp4_init_filename " + outputFileNameWithoutExtension + "-1" + outputExtension
		}

		segmentFormat = "fmp4" + outputFmp4HeaderArg
	} else {
		//_logger.LogError("Invalid HLS segment container: {SegmentContainer}, default to mpegts", segmentContainer);
		fmt.Printf("Invalid HLS segment container: %s, default to mpegts\n", segmentContainer)
		segmentFormat = "mpegts"
	}

	maxMuxingQueueSize := "128"
	if d.encodingOptions.MaxMuxingQueueSize > 128 {
		maxMuxingQueueSize = strconv.Itoa(d.encodingOptions.MaxMuxingQueueSize)
	}

	baseURLParam := ""
	if isEventPlaylist {
		baseURLParam = fmt.Sprintf("-hls_base_url \"hls/%s/\"", filepath.Base(outputPath))
	}

	hlsPlaylistType := "vod"
	if isEventPlaylist {
		hlsPlaylistType = "event"
	}

	hlsArguments := fmt.Sprintf("-hls_playlist_type %s -hls_list_size 0", hlsPlaylistType)

	videoArgs := d.GetVideoArguments(state, startNumber, isEventPlaylist, state.Request.SegmentContainer)
	audioArgs := d.GetAudioArguments(state)

	fmt.Println(mapArgs)
	args := []string{
		inputModifier,
		d.encodingHelper.GetInputArgument(&state.EncodingJobInfo, d.encodingOptions, state.Request.SegmentContainer),
		"-map_metadata -1",
		"-map_chapters -1",
		"-threads", strconv.Itoa(threads),
		mapArgs,
		videoArgs,
		audioArgs,
		"-copyts",
		"-avoid_negative_ts", "disabled",
		"-max_muxing_queue_size", maxMuxingQueueSize,
		"-f", "hls",
		"-max_delay", "5000000",
		"-hls_time", strconv.Itoa(state.SegmentLength()),
		"-hls_segment_type", segmentFormat,
		"-start_number", strconv.Itoa(startNumber),
		baseURLParam,
		"-hls_segment_filename", encoder.NormalizePath(outputTsArg),
		hlsArguments,
		"-y", encoder.NormalizePath(outputPath),
	}
	//	args = strings.Fields(strings.Join(args, " "))

	return strings.Join(args, " ")
}

func (d *DynamicHlsController) DeleteLastFile(playlistPath string, segmentExtension string, count int) error {
	// Implement DeleteLastFile logic here
	return nil
}

func (d *DynamicHlsController) changeExtension(path, ext string) string {
	return filepath.Join(filepath.Dir(path), filepath.Base(path[:len(path)-len(filepath.Ext(path))]+ext))
}

func (d *DynamicHlsController) GetCurrentTranscodingIndex(playlistPath, segmentExtension string) *int {
	job := d.transcodeManager.GetTranscodingJob2(playlistPath, transcodingJobType)
	if job == nil || job.HasExited {
		fmt.Println(job)
		if job != nil {
			fmt.Println("GetCurrentTranscodingIndex eeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
		}
		return nil
	}

	file := GetLastTranscodingFile(playlistPath, segmentExtension, d.fileSystem)
	if file == nil {
		fmt.Println("GetLastTranscodingFile nillllllllllllllllllllll")
		return nil
	}

	playlistFilename := filepath.Base(playlistPath)
	fmt.Println(playlistFilename)
	playlistFilename = playlistFilename[:len(playlistFilename)-len(filepath.Ext(playlistPath))]
	fmt.Println(playlistFilename)

	indexString := filepath.Base(file.Name)
	indexString = indexString[:len(indexString)-len(filepath.Ext(file.Name))]

	indexString = strings.TrimPrefix(indexString, playlistFilename)
	index, err := strconv.Atoi(indexString)

	if err != nil {
		fmt.Println("strconv", err)
		return nil
	}

	return &index
}

func GetLastTranscodingFile(playlistPath string, segmentExtension string, fileSystem ioo.IFileSystem) *ioo.FileSystemMetadata {
	folder := filepath.Dir(playlistPath)
	if folder == "" {
		panic(fmt.Errorf("path can't be a root directory: %s", playlistPath))
	}

	filePrefix := filepath.Base(playlistPath)
	filePrefix = strings.TrimSuffix(filePrefix, filepath.Ext(filePrefix))

	files, err := fileSystem.GetFiles(folder, false)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	var latestFile *ioo.FileSystemMetadata
	var latestModTime time.Time

	for i, file := range files {
		if filepath.Ext(file.Name) == segmentExtension && strings.HasPrefix(filepath.Base(file.Name), filePrefix) {
			modTime, err := fileSystem.GetLastWriteTimeUtc(filepath.Join(folder, file.Name))
			if err != nil {
				fmt.Println(err)
				continue
			}
			if latestFile == nil || modTime.After(latestModTime) {
				latestFile = &files[i]
				latestModTime = modTime
			}
		}
	}

	return latestFile
}

func (d *DynamicHlsController) getSegmentPath(state *streaming.StreamState, playlist string, index int) (string, error) {
	folder, filename := getPathParts(playlist)
	if folder == "" {
		return "", fmt.Errorf("provided path (%s) is not valid", playlist)
	}

	extension := mediaencoding.GetSegmentFileExtension(state.Request.SegmentContainer)
	segmentFilename := fmt.Sprintf("%s%d%s", filename, index, extension)
	return filepath.Join(folder, segmentFilename), nil
}

func getPathParts(path string) (string, string) {
	folder := filepath.Dir(path)
	filename := filepath.Base(path)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))
	return folder, filename
}

func (d *DynamicHlsController) GetVariantHlsVideoPlaylist(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Println(vars)
	itemId, err := uuid.Parse(vars["itemId"])
	if err != nil {
		d.logger.Warnf("itemId: %v\n", err)
	} else {
		d.logger.Debugf("itemId: %v\n", itemId)
	}

	/*
		// Extract parameters from the request
		playPath := r.URL.Query().Get("PlayPath")
		if strings.HasPrefix(playPath, prefix) {
			seafileServiceName := os.Getenv("SEAFILE_SERVICE")
			playPath = "http://" + seafileServiceName + playPath[len(prefix):]
		} else if strings.HasPrefix(playPath, prefixExternal) {
			dataDir := os.Getenv("MEDIA_SERVER_DATA_DIR")
			playPath = dataDir + playPath
		} else {
			playPath = filepath.Clean(playPath)
			if !filepath.IsAbs(playPath) {
				fmt.Println("invalid PlayPath")
				http.Error(w, "invalid PlayPath", http.StatusBadRequest)
				return
			}

			if _, ok := r.Header[utils.BFL_HEADER]; !ok {
				log.Printf("%s does not exist\n", utils.BFL_HEADER)
				http.Error(w, "bfl header not exist", http.StatusBadRequest)
				return
			}

			bflNames := r.Header[utils.BFL_HEADER]

			playPath, err = utils.GetEffectPlayPath(playPath, bflNames[0])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		fmt.Println(playPath)
	*/

	if _, ok := r.Header[common.REQUEST_HEADER_OWNER]; !ok {
		log.Printf("%s does not exist\n", common.REQUEST_HEADER_OWNER)
		http.Error(w, "bfl header not exist", http.StatusBadRequest)
		return
	}

	bflName := r.Header[common.REQUEST_HEADER_OWNER][0]
	playPath, err := pathCommon(r.URL.Query().Get("PlayPath"), bflName)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
		return
	}

	static, _ := strconv.ParseBool(r.URL.Query().Get("Static"))
	params := r.URL.Query().Get("Params")
	tag := r.URL.Query().Get("Tag")
	//deviceProfileId := r.URL.Query().Get("deviceProfileId")
	playSessionId := r.URL.Query().Get("PlaySessionId")
	var segmentContainer *string
	if tmp := r.URL.Query().Get("SegmentContainer"); tmp != "" {
		segmentContainer = &tmp
	}
	var segmentLength *int
	if _, ok := r.URL.Query()["SegmentLength"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("SegmentLength"))
		if err != nil {
			segmentLength = &tmp
		}
	}
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
	audioBitRate, _ := strconv.Atoi(r.URL.Query().Get("AudioBitrate"))
	audioChannels, _ := strconv.Atoi(r.URL.Query().Get("AudioChannels"))
	maxAudioChannels, _ := strconv.Atoi(r.URL.Query().Get("MaxAudioChannels"))
	profile := r.URL.Query().Get("Profile")
	level := r.URL.Query().Get("Level")
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
	copyTimestamps, _ := strconv.ParseBool(r.URL.Query().Get("CopyTimestamps"))
	startTimeTicks, _ := strconv.ParseInt(r.URL.Query().Get("StartTimeTicks"), 10, 64)
	var width *int
	if _, ok := r.URL.Query()["Width"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("Width"))
		if err != nil {
			width = &tmp
		}
	}
	var height *int
	if _, ok := r.URL.Query()["Height"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("Height"))
		if err != nil {
			height = &tmp
		}
	}
	var maxWidth *int
	if _, ok := r.URL.Query()["MaxWidth"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("MaxWidth"))
		if err != nil {
			maxWidth = &tmp
		}
	}
	var maxHeight *int
	if _, ok := r.URL.Query()["MaxHeight"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("MaxHeight"))
		if err != nil {
			maxHeight = &tmp
		}
	}
	videoBitRate, _ := strconv.Atoi(r.URL.Query().Get("VideoBitrate"))
	subtitleStreamIndex, _ := strconv.Atoi(r.URL.Query().Get("SubtitleStreamIndex"))
	//subtitleMethod, _ := strconv.Atoi(r.URL.Query().Get("SubtitleMethod"))
	subtitleMethod, _ := dlna.ParseSubtitleDeliveryMethod(r.URL.Query().Get("SubtitleMethod"))
	fmt.Println(r.URL.Query().Get("SubtitleMethod"), "-----------------------9527--------------------------------------", subtitleMethod)
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
	subtitleCodec := r.URL.Query().Get("SubtitleCodec")
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

	var streamOptions StreamOptions
	if err := GetFromQuery(r, &streamOptions); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	streamingRequest := &streaming.VideoRequestDto{
		StreamingRequestDto: &streaming.StreamingRequestDto{
			BaseEncodingJobOptions: &mediaencoding.BaseEncodingJobOptions{
				PlayPath:                    playPath,
				Id:                          itemId,
				Static:                      static,
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
				AudioChannels:               &audioChannels,
				Profile:                     profile,
				Level:                       level,
				Framerate:                   framerate,
				MaxFramerate:                maxFramerate,
				CopyTimestamps:              copyTimestamps,
				StartTimeTicks:              &startTimeTicks,
				Width:                       width,
				Height:                      height,
				MaxWidth:                    maxWidth,
				MaxHeight:                   maxHeight,
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
			},
			Params:           &params,
			Tag:              &tag,
			PlaySessionID:    &playSessionId,
			SegmentContainer: segmentContainer,
			SegmentLength:    segmentLength,
			MinSegments:      &minSegments,
		},
		HasFixedResolution:        false,
		EnableSubtitlesInManifest: false,
		EnableTrickplay:           false,
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	playlist, err := d.GetVariantPlaylistInternal(r, ctx, streamingRequest)
	if err != nil {
		log.Printf("GetVariantPlaylistInternal ret: %v", err)
		http.Error(w, "GetPlaylist invalid PlayPath", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/x-mpegURL")
	_, _ = w.Write([]byte(playlist))
}

func (d *DynamicHlsController) GetVariantPlaylistInternal(r *http.Request, ctx context.Context, request interface{}) (string, error) {

	state, err := helpers.GetStreamingState(
		request,
		r,
		//		_mediaSourceManager,
		//		_userManager,
		//		_libraryManager,
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
	fmt.Println(state.Request.SegmentContainer)

	//	state.MediaPath = "../sezc.mkv"
	//    state.SegmentLength = 10
	//	var runTimeTicks int64 = (time.Duration(8515) * time.Second).Nanoseconds() / 100
	//	state.RunTimeTicks = &runTimeTicks
	//    state.SegmentContainer = "ts"
	//    state.OutputVideoCodec = "h264"

	container := func() string {
		if state.Request.SegmentContainer == nil {
			return ""
		} else {
			return *state.Request.SegmentContainer
		}
	}()
	mainPlaylistRequest := playlist.CreateMainPlaylistRequest{
		FilePath:               state.MediaPath,
		DesiredSegmentLengthMs: state.SegmentLength() * 1000,
		TotalRuntimeTicks: func() int64 {
			if state.RunTimeTicks == nil {
				return 0
			} else {
				return *state.RunTimeTicks
			}
		}(),
		SegmentContainer: &container,
		EndpointPrefix:   "hls1/main/",
		QueryString:      "?" + r.URL.RawQuery,
		IsRemuxingVideo:  mediaencoding.IsCopyCodec(state.OutputVideoCodec),
	}

	playlist := d.dynamicHlsPlaylistGenerator.CreateMainPlaylist(&mainPlaylistRequest)

	//    fmt.Println(playlist)
	//    w.Header().Set("Content-Type", MimeTypes.GetMimeType("playlist.m3u8"))
	//    _, err = w.Write([]byte(playlist))
	//    if err != nil {
	//        http.Error(w, err.Error(), http.StatusInternalServerError)
	//    }

	return playlist, nil
}

func containsCaseInsensitive(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func (d *DynamicHlsController) GetVideoArguments(state *streaming.StreamState, startNumber int, isEventPlaylist bool, segmentContainer *string) string {
	if state.VideoStream == nil || !state.IsOutputVideo {
		return ""
	}

	codec := d.encodingHelper.GetVideoEncoder(&state.EncodingJobInfo, d.encodingOptions)
	args := fmt.Sprintf("-codec:v:0 %s", codec)

	isActualOutputVideoCodecAv1 := strings.EqualFold(state.ActualOutputVideoCodec(), "av1")
	isActualOutputVideoCodecHevc := strings.EqualFold(state.ActualOutputVideoCodec(), "h265") || strings.EqualFold(state.ActualOutputVideoCodec(), "hevc")

	if isActualOutputVideoCodecHevc || isActualOutputVideoCodecAv1 {
		requestedRange := state.EncodingJobInfo.GetRequestedRangeTypes(state.ActualOutputVideoCodec())
		clientSupportsDoVi := containsCaseInsensitive(requestedRange, "DOVI")
		videoIsDoVi := mediaencoding.IsDovi(state.VideoStream)

		if mediaencoding.IsCopyCodec(codec) && videoIsDoVi && clientSupportsDoVi && !d.encodingHelper.IsDoviRemoved(&state.EncodingJobInfo) {
			if isActualOutputVideoCodecHevc {
				args += " -tag:v:0 dvh1 -strict -2"
			} else if isActualOutputVideoCodecAv1 {
				args += " -tag:v:0 dav1 -strict -2"
			}
		} else if isActualOutputVideoCodecHevc {
			args += " -tag:v:0 hvc1"
		}
	}

	if mediaencoding.IsCopyCodec(codec) {
		if state.VideoStream != nil && !strings.EqualFold(state.VideoStream.NalLengthSize, "0") {
			bitStreamArgs := d.encodingHelper.GetBitStreamArgs(&state.EncodingJobInfo, "Video")
			if bitStreamArgs != "" {
				args += " " + bitStreamArgs
			}
		}
		args += " -start_at_zero"
	} else {
		preset := DefaultVodEncoderPreset
		if isEventPlaylist {
			preset = DefaultEventEncoderPreset
		}
		args += d.encodingHelper.GetVideoQualityParam(&state.EncodingJobInfo, codec, d.encodingOptions, preset)
		args += d.encodingHelper.GetHlsVideoKeyFrameArguments(&state.EncodingJobInfo, codec, state.SegmentLength(), isEventPlaylist, &startNumber)

		if strings.EqualFold(codec, "libx265") && version.Compare(*d.mediaEncoder.EncoderVersion(), minFFmpegX265BframeInFmp4) < 0 {
			args += " -bf 0"
		}

		videoProcessParam := d.encodingHelper.GetVideoProcessingFilterParam(&state.EncodingJobInfo, d.encodingOptions, codec)
		negativeMapArgs := d.encodingHelper.GetNegativeMapArgsByFilters(&state.EncodingJobInfo, videoProcessParam)
		args = negativeMapArgs + args + videoProcessParam

		if state.SubtitleStream != nil {
			if !(state.SubtitleStream.IsExternal && !state.SubtitleStream.IsTextSubtitleStream()) {
				args += " -start_at_zero"
			}
		}
	}

	if isEventPlaylist && strings.EqualFold(*segmentContainer, "ts") {
		args += " -flags -global_header"
	}

	if state.OutputVideoSync != "" {
		args += mediaencoding.GetVideoSyncOption(state.OutputVideoSync, *d.mediaEncoder.EncoderVersion())
	}

	args += d.encodingHelper.GetOutputFFlags(&state.EncodingJobInfo)

	return args
}

func (d *DynamicHlsController) _GetVideoArguments(state *streaming.StreamState, startNumber int, isEventPlaylist bool, segmentContainer *string) string {
	if state.VideoStream == nil {
		return ""
	}

	if !state.IsOutputVideo {
		return ""
	}

	codec := d.encodingHelper.GetVideoEncoder(&state.EncodingJobInfo, d.encodingOptions)

	args := "-codec:v:0 " + codec

	// Add codec-specific arguments based on the output video codec
	// ...

	// Add video quality parameters
	defaultPreset := DefaultVodEncoderPreset
	if isEventPlaylist {
		defaultPreset = DefaultEventEncoderPreset
	}
	args += d.encodingHelper.GetVideoQualityParam(&state.EncodingJobInfo, codec, d.encodingOptions, defaultPreset)

	// Add HLS video key frame arguments
	args += d.encodingHelper.GetHlsVideoKeyFrameArguments(&state.EncodingJobInfo, codec, state.SegmentLength(), isEventPlaylist, &startNumber)

	// Add video processing filter parameters
	videoProcessParam := d.encodingHelper.GetVideoProcessingFilterParam(&state.EncodingJobInfo, d.encodingOptions, codec)
	fmt.Printf("video process param: %v\n", videoProcessParam)
	negativeMapArgs := d.encodingHelper.GetNegativeMapArgsByFilters(&state.EncodingJobInfo, videoProcessParam)
	args = negativeMapArgs + args + videoProcessParam

	// Add output video sync argument
	if state.OutputVideoSync != "" {
		args += " -vsync " + state.OutputVideoSync
	}

	// Add output flags
	args += d.encodingHelper.GetOutputFFlags(&state.EncodingJobInfo)

	return args
}

func getEncoderPreset(isEventPlaylist bool) entities.EncoderPreset {
	if isEventPlaylist {
		return DefaultEventEncoderPreset
	}
	return DefaultVodEncoderPreset
}

func (d *DynamicHlsController) GetAudioArguments(state *streaming.StreamState) string {
	if state.AudioStream == nil {
		return ""
	}

	audioCodec := d.encodingHelper.GetAudioEncoder(state.EncodingJobInfo)
	bitStreamArgs := d.encodingHelper.GetAudioBitStreamArguments(&state.EncodingJobInfo, state.Request.SegmentContainer, state.MediaSource.Container)

	// opus, dts, truehd and flac (in FFmpeg 5 and older) are experimental in mp4 muxer
	strictArgs := ""
	actualOutputAudioCodec := state.ActualOutputAudioCodec()
	if strings.EqualFold(actualOutputAudioCodec, "opus") ||
		strings.EqualFold(actualOutputAudioCodec, "dts") ||
		strings.EqualFold(actualOutputAudioCodec, "truehd") ||
		(strings.EqualFold(actualOutputAudioCodec, "flac") || version.Compare(*d.mediaEncoder.EncoderVersion(), minFFmpegFlacInMp4) < 0) {
		strictArgs = " -strict -2"
	}

	if !state.IsOutputVideo {
		audioTranscodeParams := ""

		// -vn to drop any video streams
		audioTranscodeParams += "-vn"

		if mediaencoding.IsCopyCodec(audioCodec) {
			return audioTranscodeParams + " -acodec copy" + bitStreamArgs + strictArgs
		}

		audioTranscodeParams += " -acodec " + audioCodec + bitStreamArgs + strictArgs

		audioBitrate := state.OutputAudioBitrate
		audioChannels := state.OutputAudioChannels

		if audioBitrate != nil && !utils.Contains(mediaencoding.LosslessAudioCodecs, strings.ToLower(actualOutputAudioCodec)) {
			tmp := 2
			if audioChannels != nil {
				tmp = *audioChannels
			}
			vbrParam := d.encodingHelper.GetAudioVbrModeParam(audioCodec, *audioBitrate/int(tmp))
			if d.encodingOptions.EnableAudioVbr && vbrParam != "" {
				audioTranscodeParams += vbrParam
			} else {
				audioTranscodeParams += " -ab " + strconv.Itoa(*audioBitrate)
			}
		}

		if audioChannels != nil {
			audioTranscodeParams += " -ac " + strconv.Itoa(*audioChannels)
		}

		if state.OutputAudioSampleRate != nil {
			audioTranscodeParams += " -ar " + strconv.Itoa(*state.OutputAudioSampleRate())
		}

		return audioTranscodeParams
	}

	if mediaencoding.IsCopyCodec(audioCodec) {
		videoCodec := d.encodingHelper.GetVideoEncoder(&state.EncodingJobInfo, d.encodingOptions)
		copyArgs := "-codec:a:0 copy" + bitStreamArgs + strictArgs

		if mediaencoding.IsCopyCodec(videoCodec) && state.EnableBreakOnNonKeyFrames(videoCodec) {
			return copyArgs + " -copypriorss:a:0 0"
		}

		return copyArgs
	}

	args := "-codec:a:0 " + audioCodec + bitStreamArgs + strictArgs

	channels := state.OutputAudioChannels
	fmt.Println("xxxxxxxxxxxxxxxzzzzzzzzz", *channels)

	useDownMixAlgorithm := *state.AudioStream.Channels == 6 && d.encodingOptions.DownMixStereoAlgorithm != entities.None

	if channels != nil && (*channels != 2 || (state.AudioStream != nil && state.AudioStream.Channels != nil && !useDownMixAlgorithm)) {
		args += " -ac " + strconv.Itoa(*channels)
	}

	bitrate := state.OutputAudioBitrate
	if bitrate != nil && !slices.Contains(mediaencoding.LosslessAudioCodecs, strings.ToLower(actualOutputAudioCodec)) {
		tmp := 2
		fmt.Println("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", channels, *channels)
		if channels != nil {
			tmp = *channels
		}
		vbrParam := d.encodingHelper.GetAudioVbrModeParam(audioCodec, *bitrate/int(tmp))
		if d.encodingOptions.EnableAudioVbr && vbrParam != "" {
			args += vbrParam
		} else {
			args += " -ab " + strconv.Itoa(*bitrate)
		}
	}

	if state.OutputAudioSampleRate != nil {
		args += " -ar " + strconv.Itoa(*state.OutputAudioSampleRate())
	}

	args += d.encodingHelper.GetAudioFilterParam(&state.EncodingJobInfo, d.encodingOptions)

	return args
}
