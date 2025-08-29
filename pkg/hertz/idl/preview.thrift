namespace go api.preview

struct PreviewReq {
    1: string Auth (api.query="auth");
    2: string Inline (api.query="inline");
    3: string Key (api.query="key");
    4: optional string Size (api.query="size");
    5: optional string Thumb (api.query="thumb");
}

struct PreviewResp {
}

service PreviewService {
    PreviewResp PreviewMethod(1: PreviewReq request) (api.get="/api/preview/*path");
}
