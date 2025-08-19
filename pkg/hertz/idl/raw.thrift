namespace go api.raw

struct RawResp {
}

service RawService {
    RawResp RawMethod() (api.get="/api/raw/*path");
}