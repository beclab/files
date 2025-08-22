namespace go api.repos

struct GetReposReq {
    1: optional string type (api.query="type");
}

struct ReposRepo {
    1: required string encrypted,
    2: required bool is_virtual,
    3: required string last_modified,
    4: required string modifier_contact_email,
    5: required string modifier_email,
    6: required string modifier_name,
    7: required string owner_contact_email,
    8: required string owner_email,
    9: required string owner_name,
    10: required string permission,
    11: required string repo_id,
    12: required string repo_name,
    13: required string salt,
    14: required string size,
    15: required string status,
    16: required string type
}

struct GetReposResp {
    1: list<ReposRepo> repos
}

struct PostReposReq {
    1: string repoName (api.query="repoName");
}

struct PostReposResp {
}

struct DeleteReposReq {
    1: required string repoId (api.query="repoId");
}

struct DeleteReposResp {
}

struct PatchReposReq {
    1: required string repoId (api.query="repoId");
    2: required string destination (api.query="destination");
}

struct PatchReposResp {
}

service ReposService {
    GetReposResp GetReposMethod(1: GetReposReq request) (api.get="/api/repos");
    PostReposResp PostReposMethod(1: PostReposReq request) (api.post="/api/repos");
    DeleteReposResp DeleteReposMethod(1: DeleteReposReq request) (api.delete="/api/repos");
    PatchReposResp PatchReposMethod(1: PatchReposReq request) (api.patch="/api/repos");
}

