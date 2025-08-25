namespace go api.raw

struct RawReq {
    1: optional string Auth (api.query="auth");
    2: optional string Inline (api.query="inline");
    3: optional string Meta (api.query="meta");
}

struct RawResp {
}

service RawService {
    RawResp RawMethod(1: RawReq request) (api.get="/api/raw/*path");
}