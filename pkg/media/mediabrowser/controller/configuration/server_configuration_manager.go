package configuration

import (
	//	"encoding/json"
	//	"fmt"
	//	"os"

	cc "files/pkg/media/mediabrowser/common/configuration"
	"files/pkg/media/mediabrowser/model/configuration"
)

/*
type IServerApplicationPaths interface {
	GetCachePath() string
	GetDataPath() string
	GetConfigurationPath() string
	// Add other necessary methods from IServerApplicationPaths
}
*/

type IServerConfigurationManager interface {
	cc.IConfigurationManager
	//	ApplicationPaths() IServerApplicationPaths
	Configuration() *configuration.ServerConfiguration
}

/*
type ServerConfigurationManager struct {
	applicationPaths IServerApplicationPaths
	configuration    *ServerConfiguration
}

func NewServerConfigurationManager(applicationPaths IServerApplicationPaths, configuration *ServerConfiguration) *ServerConfigurationManager {
	return &ServerConfigurationManager{
		applicationPaths: applicationPaths,
		configuration:    configuration,
	}
}

func (s *ServerConfigurationManager) GetConfiguration(string) any {
	fmt.Println("ServerConfigurationManager.GetConfiguration###############")
	return `{
    "EncodingThreadCount": -1,
    "EnableFallbackFont": false,
    "EnableAudioVbr": false,
    "DownMixAudioBoost": 2,
    "DownMixStereoAlgorithm": "None",
    "MaxMuxingQueueSize": 2048,
    "EnableThrottling": false,
    "ThrottleDelaySeconds": 180,
    "EnableSegmentDeletion": false,
    "SegmentKeepSeconds": 720,
    "EncoderAppPath": "/usr/lib/jellyfin-ffmpeg/ffmpeg",
    "EncoderAppPathDisplay": "/usr/lib/jellyfin-ffmpeg/ffmpeg",
    "VaapiDevice": "/dev/dri/renderD128",
    "EnableTonemapping": false,
    "EnableVppTonemapping": false,
    "EnableVideoToolboxTonemapping": false,
    "TonemappingAlgorithm": "bt2390",
    "TonemappingMode": "auto",
    "TonemappingRange": "auto",
    "TonemappingDesat": 0,
    "TonemappingPeak": 100,
    "TonemappingParam": 0,
    "VppTonemappingBrightness": 16,
    "VppTonemappingContrast": 1,
    "H264Crf": 23,
    "H265Crf": 28,
    "DeinterlaceDoubleRate": false,
    "DeinterlaceMethod": "yadif",
    "EnableDecodingColorDepth10Hevc": true,
    "EnableDecodingColorDepth10Vp9": true,
    "EnableEnhancedNvdecDecoder": true,
    "PreferSystemNativeHwDecoder": true,
    "EnableIntelLowPowerH264HwEncoder": false,
    "EnableIntelLowPowerHevcHwEncoder": false,
    "EnableHardwareEncoding": true,
    "AllowHevcEncoding": false,
    "AllowAv1Encoding": false,
    "EnableSubtitleExtraction": true,
    "HardwareAccelerationType": "videotoolbox",
    "HardwareAccelerationType": "rkmpp",
    "HardwareAccelerationType": "vaapi",
    "HardwareDecodingCodecs": [
        "h264",
        "vc1"
    ],
    "AllowOnDemandMetadataBasedKeyframeExtractionForExtensions": [
        "mkv"
    ]
}`
}

func (c *ServerConfigurationManager) GetTranscodePath() string {
	transcodingTempPath := "/tmp/cache/transcodes"
	// Make sure the directory exists
	err := os.MkdirAll(transcodingTempPath, 0755)
	if err != nil {
		fmt.Println(err)
	}

	return transcodingTempPath
}

func (s *ServerConfigurationManager) GetEncodingOptions() *configuration.EncodingOptions {
	fmt.Println("### ServerConfigurationManager GetEncodingOptions ###")
	var options configuration.EncodingOptions
	err := json.Unmarshal([]byte(s.GetConfiguration("encoding").(string)), &options)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return nil
	}
	fmt.Printf("options: %+v\n", options)
	return &options
	if false {

		options, ok := s.GetConfiguration("encoding").(*configuration.EncodingOptions)
		if !ok {
			fmt.Println("not ok GetEncodingOptions")
			return nil
		}
		fmt.Println(options)
		return options
		return &configuration.EncodingOptions{
			AllowOnDemandMetadataBasedKeyframeExtractionForExtensions: []string{
				"mkv",
			},
		}
	}
}
*/
