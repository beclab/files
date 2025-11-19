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

struct UpdateNamedConfigReq {}
struct UpdateNamedConfigResp {}

service MediaService {
  PlayControllerReq GetCustomPlayController(1: PlayControllerReq request) (api.get="/videos/:node/");
  GetMasterHlsVideoPlaylistResp GetMasterHlsVideoPlaylist(1: GetMasterHlsVideoPlaylistReq request) (api.get="/videos/master.m3u8");
  GetVariantHlsVideoPlaylistResp GetVariantHlsVideoPlaylist(1: GetVariantHlsVideoPlaylistReq request) (api.get="/videos/:node/main.m3u8");
  GetVideoSegmentResp GetHlsVideoSegment(1: GetVideoSegmentReq request) (api.get="/videos/:node/hls1/:playlistId/:filename");
  GetNamedConfigResp GetNamedConfig(1: GetNamedConfigReq request) (api.get="/System/Configuration/:key");
  UpdateNamedConfigResp UpdateNamedConfig(1: UpdateNamedConfigReq request) (api.post="/System/Configuration/:key");
}