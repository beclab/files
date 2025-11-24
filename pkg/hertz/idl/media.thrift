namespace go api.media

struct PlayControllerReq {}
struct PlayControllerResp {}

struct GetMasterHlsVideoPlaylistReq {}
struct GetMasterHlsVideoPlaylistResp {}

struct GetVariantHlsVideoPlaylistReq {}
struct GetVariantHlsVideoPlaylistResp {}

struct GetVideoSegmentReq {}
struct GetVideoSegmentResp {}

struct GetNamedConfigReq {}
struct GetNamedConfigResp {}

struct UpdateNamedConfigReq {
  1: UpdateNamedConfigData data;
}

struct UpdateNamedConfigData {
  1: bool EnableFallbackFont;
  2: bool EnableAudioVbr;
  3: i32 DownMixAudioBoost;
  4: string DownMixStereoAlgorithm;
  5: i32 MaxMuxingQueueSize;
  6: bool EnableThrottling;
  7: i32 ThrottleDelaySeconds;
  8: bool EnableSegmentDeletion;
  9: i32 SegmentKeepSeconds;
  10: i32 EncodingThreadCount;
  11: string VaapiDevice;
  12: string QsvDevice;
  13: bool EnableTonemapping;
  14: bool EnableVppTonemapping;
  15: bool EnableVideoToolboxTonemapping;
  16: string TonemappingAlgorithm;
  17: string TonemappingMode;
  18: string TonemappingRange;
  19: i32 TonemappingDesat;
  20: i32 TonemappingPeak;
  21: i32 TonemappingParam;
  22: i32 VppTonemappingBrightness;
  23: i32 VppTonemappingContrast;
  24: i32 H264Crf;
  25: i32 H265Crf;
  26: bool DeinterlaceDoubleRate;
  27: string DeinterlaceMethod;
  28: bool EnableDecodingColorDepth10Hevc;
  29: bool EnableDecodingColorDepth10Vp9;
  30: bool EnableDecodingColorDepth10HevcRext;
  31: bool EnableDecodingColorDepth12HevcRext;
  32: bool EnableEnhancedNvdecDecoder;
  33: bool PreferSystemNativeHwDecoder;
  34: bool EnableIntelLowPowerH264HwEncoder;
  35: bool EnableIntelLowPowerHevcHwEncoder;
  36: bool EnableHardwareEncoding;
  37: bool AllowHevcEncoding;
  38: bool AllowAv1Encoding;
  39: bool EnableSubtitleExtraction;
  40: list<string> AllowOnDemandMetadataBasedKeyframeExtractionForExtensions;
  41: list<string> HardwareDecodingCodecs;
  42: string TranscodingTempPath;
  43: string FallbackFontPath;
  44: string HardwareAccelerationType;
  45: string EncoderAppPath;
  46: string EncoderAppPathDisplay;
  47: string EncoderPreset;
}

struct UpdateNamedConfigResp {
  1: UpdateNamedConfigData data;
}

service MediaService {
  PlayControllerReq GetCustomPlayController(1: PlayControllerReq request) (api.get="/videos/:node/");
  GetMasterHlsVideoPlaylistResp GetMasterHlsVideoPlaylist(1: GetMasterHlsVideoPlaylistReq request) (api.get="/videos/master.m3u8");
  GetVariantHlsVideoPlaylistResp GetVariantHlsVideoPlaylist(1: GetVariantHlsVideoPlaylistReq request) (api.get="/videos/:node/main.m3u8");
  GetVideoSegmentResp GetHlsVideoSegment(1: GetVideoSegmentReq request) (api.get="/videos/:node/hls1/:playlistId/:filename");
  GetNamedConfigResp GetNamedConfig(1: GetNamedConfigReq request) (api.get="/System/Configuration/:key");
  UpdateNamedConfigResp UpdateNamedConfig(1: UpdateNamedConfigReq request) (api.post="/System/Configuration/:key");
}