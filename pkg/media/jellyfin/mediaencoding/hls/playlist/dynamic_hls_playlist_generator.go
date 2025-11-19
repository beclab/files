package playlist

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"files/pkg/media/jellyfin/mediaencoding/hls/extractors"
	"files/pkg/media/jellyfin/mediaencoding/keyframes"
	"files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"

	"k8s.io/klog/v2"
)

type IDynamicHlsPlaylistGenerator interface {
	CreateMainPlaylist(request *CreateMainPlaylistRequest) string
}

type DynamicHlsPlaylistGenerator struct {
	serverConfigurationManager configuration.IServerConfigurationManager
	extractors                 []extractors.IKeyframeExtractor
}

// func NewDynamicHlsPlaylistGenerator(serverConfigurationManager configuration.IServerConfigurationManager, extractors []extractors.IKeyframeExtractor) *dynamicHlsPlaylistGenerator {
func NewDynamicHlsPlaylistGenerator(serverConfigurationManager configuration.IServerConfigurationManager) *DynamicHlsPlaylistGenerator {
	return &DynamicHlsPlaylistGenerator{
		serverConfigurationManager: serverConfigurationManager,
		extractors:                 []extractors.IKeyframeExtractor{&extractors.FfProbeKeyframeExtractor{}, &extractors.MatroskaKeyframeExtractor{}},
	}
}

func (g *DynamicHlsPlaylistGenerator) CreateMainPlaylist(request *CreateMainPlaylistRequest) string {
	var segments []float64
	var keyframeData keyframes.KeyframeData
	if request.IsRemuxingVideo && g.TryExtractKeyframes(request.FilePath, &keyframeData) {
		segments = g.ComputeSegments(keyframeData, request.DesiredSegmentLengthMs)
	} else {
		segments = g.ComputeEqualLengthSegments(request.DesiredSegmentLengthMs, request.TotalRuntimeTicks)
	}

	segmentExtension := mediaencoding.GetSegmentFileExtension(request.SegmentContainer)
	isHlsInFmp4 := strings.EqualFold(segmentExtension, ".mp4")
	hlsVersion := "3"
	if isHlsInFmp4 {
		hlsVersion = "7"
	}

	builder := strings.Builder{}
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	builder.WriteString("#EXT-X-VERSION:")
	builder.WriteString(hlsVersion)
	builder.WriteString("\n")
	builder.WriteString("#EXT-X-TARGETDURATION:")
	builder.WriteString(fmt.Sprint(math.Ceil(getMaxSegmentLength(segments, request.DesiredSegmentLengthMs))))
	builder.WriteString("\n")
	builder.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")

	if isHlsInFmp4 {
		// Init file that only includes fMP4 headers
		builder.WriteString("#EXT-X-MAP:URI=\"")
		builder.WriteString(request.EndpointPrefix)
		builder.WriteString("-1")
		builder.WriteString(segmentExtension)
		builder.WriteString(request.QueryString)
		builder.WriteString("&runtimeTicks=0")
		builder.WriteString("&actualSegmentLengthTicks=0")
		builder.WriteString("\"\n")
	}

	currentRuntimeInSeconds := int64(0)
	klog.Infoln(len(segments))
	for index, length := range segments {
		lengthTicks := (time.Duration(length) * time.Second).Nanoseconds() / 100 //tick
		builder.WriteString("#EXTINF:")
		builder.WriteString(fmt.Sprintf("%.6f", length))
		builder.WriteString(", nodesc")
		builder.WriteString("\n")
		builder.WriteString(request.EndpointPrefix)
		builder.WriteString(fmt.Sprint(index))
		builder.WriteString(segmentExtension)
		builder.WriteString(request.QueryString)
		builder.WriteString("&runtimeTicks=")
		builder.WriteString(fmt.Sprint(currentRuntimeInSeconds))
		builder.WriteString("&actualSegmentLengthTicks=")
		builder.WriteString(fmt.Sprint(lengthTicks))
		builder.WriteString("\n")

		currentRuntimeInSeconds += lengthTicks
	}

	builder.WriteString("#EXT-X-ENDLIST\n")

	return builder.String()
}

func (d *DynamicHlsPlaylistGenerator) TryExtractKeyframes(filePath string, keyframeData *keyframes.KeyframeData) bool {

	if !d.IsExtractionAllowedForFile(filePath,
		d.serverConfigurationManager.GetEncodingOptions().AllowOnDemandMetadataBasedKeyframeExtractionForExtensions) {
		return false
	}

	for _, extractor := range d.extractors {
		if keyframe, ok := extractor.TryExtractKeyframes(filePath); ok {
			*keyframeData = keyframe
			return true
		}
	}
	return false
}

func (g *DynamicHlsPlaylistGenerator) IsExtractionAllowedForFile(filePath string, allowedExtensions []string) bool {
	extension := filepath.Ext(filePath)
	klog.Infof("Extension: %s\n", filePath)
	if extension == "" {
		return false
	}

	// Remove the leading dot
	extension = extension[1:]
	klog.Infof("Extension: %s\n", extension)
	for _, allowedExtension := range allowedExtensions {
		if strings.EqualFold(extension, strings.TrimPrefix(allowedExtension, ".")) {
			return true
		}
	}
	return false
}

func (g *DynamicHlsPlaylistGenerator) ComputeSegments(keyframeData keyframes.KeyframeData, desiredSegmentLengthMs int) []float64 {
	if len(keyframeData.KeyframeTicks) > 0 && keyframeData.TotalDuration < keyframeData.KeyframeTicks[len(keyframeData.KeyframeTicks)-1] {
		panic("Invalid duration in keyframe data")
	}

	var result []float64
	lastKeyframe := int64(0)
	desiredSegmentLengthTicks := int64(desiredSegmentLengthMs) * int64(time.Millisecond)
	desiredCutTime := desiredSegmentLengthTicks

	for _, keyframe := range keyframeData.KeyframeTicks {
		if keyframe >= desiredCutTime {
			currentSegmentLength := float64(keyframe-lastKeyframe) / float64(time.Second)
			result = append(result, currentSegmentLength)
			lastKeyframe = keyframe
			desiredCutTime += desiredSegmentLengthTicks
		}
	}

	result = append(result, float64(keyframeData.TotalDuration-lastKeyframe)/float64(time.Second))
	return result
}

func (d *DynamicHlsPlaylistGenerator) ComputeEqualLengthSegments(desiredSegmentLengthMs int, totalRuntimeTicks int64) []float64 {
	klog.Infoln("ComputeEqualLengthSegments", desiredSegmentLengthMs, totalRuntimeTicks)
	if desiredSegmentLengthMs == 0 || totalRuntimeTicks == 0 {
		panic(fmt.Sprintf("Invalid segment length (%d) or runtime ticks (%d)", desiredSegmentLengthMs, totalRuntimeTicks))
	}

	desiredSegmentLength := time.Duration(desiredSegmentLengthMs) * time.Millisecond
	segmentLengthTicks := desiredSegmentLength.Nanoseconds() / 100
	wholeSegments := totalRuntimeTicks / segmentLengthTicks
	remainingTicks := totalRuntimeTicks % segmentLengthTicks

	segmentsLen := int(wholeSegments)
	if remainingTicks != 0 {
		segmentsLen++
	}
	segments := make([]float64, segmentsLen)

	for i := 0; i < int(wholeSegments); i++ {

		segments[i] = desiredSegmentLength.Seconds()
	}

	if remainingTicks != 0 {
		//        segments[len(segments)-1] = float64(time.Duration(remainingTicks)*time.Nanosecond) / time.Second
		segments[len(segments)-1] = float64(remainingTicks) / float64(desiredSegmentLength.Nanoseconds())
	}

	return segments
}

func getMaxSegmentLength(segments []float64, desiredSegmentLengthMs int) float64 {
	if len(segments) <= 0 {
		return float64(desiredSegmentLengthMs / 1000)
	}

	var maxLength float64
	for _, length := range segments {
		if length > maxLength {
			maxLength = length
		}
	}
	return maxLength
}
