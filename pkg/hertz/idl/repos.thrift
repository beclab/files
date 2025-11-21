namespace go api.repos

struct GetReposReq {
    1: optional string type (api.query="type");
}

struct GetReposResp {
}

struct PostReposReq {
    1: string repoName (api.query="repoName");
}

struct PostReposResp {
    1: string relay_id,
    2: string relay_addr,
    3: string relay_port,
    4: string email,
    5: string token,
    6: string repo_id,
    7: string repo_name,
    8: string repo_desc,
    9: i64 repo_size,
    10: string repo_size_formatted,
    11: string mtime_iso,
    12: string mtime_relative,
    13: string repo_version,
    14: string head_commit_id,
    15: string permission,
    16: i32 encrypted,
    17: string magic,
    18: string random_key,
    19: string salt,
    20: i32 enc_version
}

struct DeleteReposReq {
    1: required string repoId (api.query="repoId");
}

struct DeleteReposResp {
    1: required bool success
}

struct PatchReposReq {
    1: required string repoId (api.query="repoId");
    2: required string destination (api.query="destination");
}

struct GetAccountInfoResp {
    1: i32 ai_credit,
    2: double ai_cost,
    3: string ai_usage_rate,
    4: string space_usage,
    5: string avatar_url,
    6: string email,
    7: string name,
    8: i64 total,
    9: i64 usage,
    10: string login_id,
    11: string department,
    12: string contact_email,
    13: string institution,
    14: bool is_staff,
    15: bool enable_subscription,
    16: i32 file_updates_email_interval,
    17: i32 collaborate_email_interval
}

struct GetReposDownloadInfoResp {
    1: string relay_id,
    2: string relay_addr,
    3: string relay_port,
    4: string email,
    5: string token,
    6: string repo_id,
    7: string repo_name,
    8: string repo_desc,
    9: i64 repo_size,
    10: string repo_size_formatted,
    11: string mtime,
    12: string mtime_relative,
    13: i32 encrypted,
    14: i32 enc_version,
    15: string salt,
    16: string magic,
    17: string random_key,
    18: string repo_version,
    19: string head_commit_id,
    20: string permission,
    21: string pwd_hash,
    22: string pwd_hash_algo,
    23: string pwd_hash_params
}

service ReposService {
    GetReposResp GetReposMethod(1: GetReposReq request) (api.get="/api/repos/");
    PostReposResp PostReposMethod(1: PostReposReq request) (api.post="/api/repos/");
    DeleteReposResp DeleteReposMethod(1: DeleteReposReq request) (api.delete="/api/repos/");
    string PatchReposMethod(1: PatchReposReq request) (api.patch="/api/repos/");

    GetAccountInfoResp GetAccountInfoMethod() (api.get="/api/sync/account/info/");
    GetReposDownloadInfoResp GetReposDonwloadInfoMethod() (api.get="/api/repos/:repo_id/download-info/");
}

