namespace go api.resources

struct GetResourcesResp {
}

struct PostResourcesResp {
}

struct PatchResourcesReq {
    1: required string destination (api.query="destination");
}

struct PatchResourcesResp {
}

struct PutResourcesReq {
    1: required binary Body (api.raw_body="raw_body");
}

struct PutResourcesResp {
}

struct DeleteResourcesReq {
    1: required list<string> dirents (api.body="dirents");
}

struct DeleteResourcesResp {
}

service ResourcesService {
    GetResourcesResp GetResourcesMethod() (api.get="/api/resources/*path");
    PostResourcesResp PostResourcesMethod() (api.post="/api/resources/*path");
    PatchResourcesResp PatchResourcesMethod(1: PatchResourcesReq request) (api.patch="/api/resources/*path");
    PutResourcesResp PutResourcesMethod(1: PutResourcesReq request) (api.put="/api/resources/*path");
    DeleteResourcesResp DeleteResourcesMethod(1: DeleteResourcesReq request) (api.delete="/api/resources/*path");
}