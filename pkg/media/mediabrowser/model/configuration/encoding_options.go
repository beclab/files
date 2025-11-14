package configuration

import (
	"files/pkg/media/mediabrowser/model/entities"
)

type EncodingOptions struct {
	EnableFallbackFont                                        bool                              `json:"EnableFallbackFont"`
	EnableAudioVbr                                            bool                              `json:"EnableAudioVbr"`
	DownMixAudioBoost                                         float64                           `json:"DownMixAudioBoost"`
	DownMixStereoAlgorithm                                    entities.DownMixStereoAlgorithms  `json:"DownMixStereoAlgorithm"`
	MaxMuxingQueueSize                                        int                               `json:"MaxMuxingQueueSize"`
	EnableThrottling                                          bool                              `json:"EnableThrottling"`
	ThrottleDelaySeconds                                      int                               `json:"ThrottleDelaySeconds"`
	EnableSegmentDeletion                                     bool                              `json:"EnableSegmentDeletion"`
	SegmentKeepSeconds                                        int                               `json:"SegmentKeepSeconds"`
	EncodingThreadCount                                       int                               `json:"EncodingThreadCount"`
	VaapiDevice                                               string                            `json:"VaapiDevice"`
	QsvDevice                                                 string                            `json:"QsvDevice"`
	EnableTonemapping                                         bool                              `json:"EnableTonemapping"`
	EnableVppTonemapping                                      bool                              `json:"EnableVppTonemapping"`
	EnableVideoToolboxTonemapping                             bool                              `json:"EnableVideoToolboxTonemapping"`
	TonemappingAlgorithm                                      entities.TonemappingAlgorithm     `json:"TonemappingAlgorithm"`
	TonemappingMode                                           entities.TonemappingMode          `json:"TonemappingMode"`
	TonemappingRange                                          entities.TonemappingRange         `json:"TonemappingRange"`
	TonemappingDesat                                          float64                           `json:"TonemappingDesat"`
	TonemappingPeak                                           float64                           `json:"TonemappingPeak"`
	TonemappingParam                                          float64                           `json:"TonemappingParam"`
	VppTonemappingBrightness                                  float64                           `json:"VppTonemappingBrightness"`
	VppTonemappingContrast                                    float64                           `json:"VppTonemappingContrast"`
	H264Crf                                                   int                               `json:"H264Crf"`
	H265Crf                                                   int                               `json:"H265Crf"`
	DeinterlaceDoubleRate                                     bool                              `json:"DeinterlaceDoubleRate"`
	DeinterlaceMethod                                         string                            `json:"DeinterlaceMethod"`
	EnableDecodingColorDepth10Hevc                            bool                              `json:"EnableDecodingColorDepth10Hevc"`
	EnableDecodingColorDepth10Vp9                             bool                              `json:"EnableDecodingColorDepth10Vp9"`
	EnableDecodingColorDepth10HevcRext                        bool                              `json:"EnableDecodingColorDepth10HevcRext"`
	EnableDecodingColorDepth12HevcRext                        bool                              `json:"EnableDecodingColorDepth12HevcRext"`
	EnableEnhancedNvdecDecoder                                bool                              `json:"EnableEnhancedNvdecDecoder"`
	PreferSystemNativeHwDecoder                               bool                              `json:"PreferSystemNativeHwDecoder"`
	EnableIntelLowPowerH264HwEncoder                          bool                              `json:"EnableIntelLowPowerH264HwEncoder"`
	EnableIntelLowPowerHevcHwEncoder                          bool                              `json:"EnableIntelLowPowerHevcHwEncoder"`
	EnableHardwareEncoding                                    bool                              `json:"EnableHardwareEncoding"`
	AllowHevcEncoding                                         bool                              `json:"AllowHevcEncoding"`
	AllowAv1Encoding                                          bool                              `json:"AllowAv1Encoding"`
	EnableSubtitleExtraction                                  bool                              `json:"EnableSubtitleExtraction"`
	AllowOnDemandMetadataBasedKeyframeExtractionForExtensions []string                          `json:"AllowOnDemandMetadataBasedKeyframeExtractionForExtensions" xml:"AllowOnDemandMetadataBasedKeyframeExtractionForExtensions>string"`
	HardwareDecodingCodecs                                    []string                          `json:"HardwareDecodingCodecs" xml:"HardwareDecodingCodecs>string"`
	TranscodingTempPath                                       string                            `json:"TranscodingTempPath"`
	FallbackFontPath                                          string                            `json:"FallbackFontPath"`
	HardwareAccelerationType                                  entities.HardwareAccelerationType `json:"HardwareAccelerationType"`
	EncoderAppPath                                            string                            `json:"EncoderAppPath"`
	EncoderAppPathDisplay                                     string                            `json:"EncoderAppPathDisplay"`
	EncoderPreset                                             entities.EncoderPreset            `json:"EncoderPreset"`
}

func NewEncodingOptions() *EncodingOptions {
	return &EncodingOptions{
		EnableFallbackFont:                 false,
		EnableAudioVbr:                     false,
		DownMixAudioBoost:                  2,
		DownMixStereoAlgorithm:             entities.None,
		MaxMuxingQueueSize:                 2048,
		EnableThrottling:                   true,
		ThrottleDelaySeconds:               180,
		EnableSegmentDeletion:              true,
		SegmentKeepSeconds:                 720,
		EncodingThreadCount:                -1,
		VaapiDevice:                        "/dev/dri/renderD128",
		QsvDevice:                          "",
		EnableTonemapping:                  false,
		EnableVppTonemapping:               false,
		EnableVideoToolboxTonemapping:      false,
		TonemappingAlgorithm:               entities.TonemappingAlgorithmBT2390,
		TonemappingMode:                    entities.TonemappingModeAuto,
		TonemappingRange:                   entities.TonemappingRangeAuto,
		TonemappingDesat:                   0,
		TonemappingPeak:                    100,
		TonemappingParam:                   0,
		VppTonemappingBrightness:           16,
		VppTonemappingContrast:             1,
		H264Crf:                            23,
		H265Crf:                            28,
		DeinterlaceDoubleRate:              false,
		DeinterlaceMethod:                  "yadif",
		EnableDecodingColorDepth10Hevc:     true,
		EnableDecodingColorDepth10Vp9:      true,
		EnableDecodingColorDepth10HevcRext: false,
		EnableDecodingColorDepth12HevcRext: false,
		EnableEnhancedNvdecDecoder:         true,
		PreferSystemNativeHwDecoder:        true,
		EnableIntelLowPowerH264HwEncoder:   false,
		EnableIntelLowPowerHevcHwEncoder:   false,
		EnableHardwareEncoding:             true,
		AllowHevcEncoding:                  false,
		AllowAv1Encoding:                   false,
		EnableSubtitleExtraction:           true,
		AllowOnDemandMetadataBasedKeyframeExtractionForExtensions: []string{"mkv"},
		HardwareDecodingCodecs: []string{"h264", "vc1"},
	}
}
