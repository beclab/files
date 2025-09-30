namespace go api.compress

struct CompressReq {
    1: required list<string> Paths (api.body="paths");
    2: string Format (api.body="format", api.default="zip");
}

struct CompressResp {
    1: string TaskId;
}

struct UncompressReq {
    1: required string Destination (api.body="destination");
    2: bool Override (api.param="override", api.default="false");
}

struct UncompressResp {
    1: string TaskId;
}

service CompressService {
    CompressResp CompressMethod(1: CompressReq request) (api.post="/api/compress/*path");
    UncompressResp UncompressMethod(1: UncompressReq request) (api.post="/api/uncompress/*path");
}