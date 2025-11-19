package mediaencoding

import (
	"context"

	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/mediabrowser/model/mediainfo"
	"files/pkg/media/utils/version"
)

type IMediaEncoder interface {
	dlna.ITranscoderSupport
	EncoderPath() string
	ProbePath() string
	EncoderVersion() *version.Version
	IsPkeyPauseSupported() bool
	IsVaapiDeviceAmd() bool
	IsVaapiDeviceInteliHD() bool
	IsVaapiDeviceInteli965() bool
	IsVaapiDeviceSupportVulkanDrmInterop() bool
	SupportsEncoder(encoder string) bool
	SupportsDecoder(decoder string) bool
	SupportsHwaccel(hwaccel string) bool
	SupportsFilter(filter string) bool
	SupportsFilterWithOption(option FilterOptionType) bool
	SupportsBitStreamFilterWithOption(option BitStreamFilterOptionType) bool
	//    ExtractAudioImage(path string, imageStreamIndex *int, cancellationToken context.Context) (string, error)
	//    ExtractVideoImage(inputFile, container string, mediaSource dto.MediaSourceInfo, videoStream entities.MediaStream, threedFormat *entities.Video3DFormat, offset *time.Duration, cancellationToken context.Context) (string, error)
	//    ExtractVideoImage(inputFile, container string, mediaSource dto.MediaSourceInfo, imageStream entities.MediaStream, imageStreamIndex *int, targetFormat *ImageFormat, cancellationToken context.Context) (string, error)
	//    ExtractVideoImagesOnIntervalAccelerated(inputFile, container string, mediaSource dto.MediaSourceInfo, imageStream entities.MediaStream, maxWidth int, interval time.Duration, allowHwAccel, enableHwEncoding bool, threads *int, qualityScale *int, priority *ProcessPriorityClass, encodingHelper EncodingHelper, cancellationToken CancellationToken) (string, error)
	GetMediaInfo(request *MediaInfoRequest, ctx context.Context, headers string) (*mediainfo.MediaInfo, error)
	//    GetInputArgument(inputFile string, mediaSource dto.MediaSourceInfo) string
	//    GetInputArgument(inputFiles []string, mediaSource dto.MediaSourceInfo) string
	GetExternalSubtitleInputArgument(inputFile string) string
	GetTimeParameter(ticks int64) string
	ConvertImage(inputPath, outputPath string) error
	EscapeSubtitleFilterPath(path string) string
	SetFFmpegPath() bool
	UpdateEncoderPath(path, pathType string)
	GetPrimaryPlaylistVobFiles(path string, titleNumber *uint) []string
	GetPrimaryPlaylistM2tsFiles(path string) []string
	GetInputPathArgument(state EncodingJobInfo) string
	GetInputPathArgument2(path string, mediaSource dto.MediaSourceInfo) string
	GenerateConcatConfig(source dto.MediaSourceInfo, concatFilePath string)
}
