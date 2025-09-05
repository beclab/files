namespace go share

// gorm models
struct SharePath {
    1: required string id (go.tag = 'gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"')
    2: required string owner (go.tag = 'gorm:"column:owner;type:text;not null"')
    3: required string file_type (go.tag = 'gorm:"column:file_type;type:varchar(10);not null"')
    4: required string extend (go.tag = 'gorm:"column:extend;type:varchar(32);uniqueIndex:idx_share_path_extend"')
    5: required string path (go.tag = 'gorm:"column:path;type:text;not null"')
    6: required string share_type (go.tag = 'gorm:"column:share_type;type:varchar(10);not null"')
    7: required string name (go.tag = 'gorm:"column:name;type:text"')
    8: required string password_md5 (go.tag = 'gorm:"column:password_md5;type:varchar(32)"')
    9: required i64 expire_in (go.tag = 'gorm:"column:expire_in;not null"')
    10: required string expire_time (go.tag = 'gorm:"column:expire_time;type:timestamptz;not null"')
    11: required i32 permission (go.tag = 'gorm:"column:permission;not null"')
    12: required string create_time (go.tag = 'gorm:"column:create_time;type:timestamptz;not null;autoCreateTime:milli"')
    13: required string update_time (go.tag = 'gorm:"column:update_time;type:timestamptz;not null;autoUpdateTime:milli"')
}

struct ShareToken {
    1: required i64 id (go.tag = 'gorm:"column:id;primaryKey;autoIncrement"')
    2: required string path_id (go.tag = 'gorm:"column:path_id;type:uuid;not null"')
    3: required string token (go.tag = 'gorm:"column:token;type:uuid;not null;default:gen_random_uuid();uniqueIndex:idx_share_token_token"')
    4: required string expire_at (go.tag = 'gorm:"column:expire_at;type:timestamptz;not null"')
}

struct ShareMember {
    1: required i64 id (go.tag = 'gorm:"column:id;primaryKey;autoIncrement"')
    2: required string path_id (go.tag = 'gorm:"column:path_id;type:uuid;not null"')
    3: required string share_member (go.tag = 'gorm:"column:share_member;type:text;not null"')
    4: required i32 permission (go.tag = 'gorm:"column:permission;not null"')
    5: required string create_time (go.tag = 'gorm:"column:create_time;type:timestamptz;not null;autoCreateTime"')
    6: required string update_time (go.tag = 'gorm:"column:update_time;type:timestamptz;not null;autoUpdateTime"')
}

// api models
struct ViewSharePath {
    1: required string id
    2: required string owner
    3: required string file_type
    4: required string extend
    5: required string path
    6: required string share_type
    7: required string name
    8: required i64 expire_in
    9: required string expire_time
    10: required i32 permission
    11: required string create_time
    12: required string update_time
    13: bool shared_by_me
}

struct CreateSharePathReq {
    // path is in the URL
    1: required string share_type (api.body="share_type", api.vd="($ == 'internal'||$ == 'external'||$ == 'smb')");
    2: required string name (api.body="name");
    3: required string password (api.body="password");
    4: i64 expire_in (api.body="expire_in");
    5: string expire_time (api.body="expire_time");
    6: required i32 permission (api.body="permission", api.vd="$>=0 && $<=4");
}

struct CreateSharePathResp {
    1: ViewSharePath share_path;
}

struct ListSharePathReq {
    1: string PathId (api.query="path_id");
    2: string Owner (api.query="owner");
    3: string FileType (api.query="file_type");
    4: string Extend (api.query="extend");
    5: string Path (api.query="path");
    6: string ShareType (api.query="share_type");
    7: string Name (api.query="name");
    8: i64 ExpireIn (api.query="expire_in");
    9: bool SharedToMe (api.query="shared_to_me");
    10: bool SharedByMe (api.query="shared_by_me");
}

struct ListSharePathResp {
    1: i32 total;
    2: list<ViewSharePath> share_paths;
}

struct DeleteSharePathReq {
    1: string PathId (api.query="path_id");
}

struct DeleteSharePathResp {
    1: bool success;
}

struct GenerateShareTokenReq {
    1: required string PathId (api.body="path_id");
    2: required string ExpireAt (api.body="expire_at");
}

struct GenerateShareTokenResp {
    1: ShareToken share_token;
}

struct ListShareTokenReq {
    1: required string PathId (api.query="path_id");
}

struct ListShareTokenResp {
    1: i32 total;
    2: list<ShareToken> share_tokens;
}

struct RevokeShareTokenReq {
    1: required string Token (api.query="token");
}

struct RevokeShareTokenResp {
    1: bool success;
}

struct AddShareMemberReq {
    1: required string PathId (api.body="path_id");
    2: required string ShareMember (api.body="share_member");
    3: required i32 Permission (api.body="permission", api.vd="$>=0 && $<=4");
}

struct AddShareMemberResp {
    1: ShareMember share_member;
}

struct ListShareMemberReq {
    1: required string PathId (api.query="path_id");
    2: string ShareMember (api.query="share_member");
    3: string Permission (api.query="permission");  // for multi-filtering
}

struct ListShareMemberResp {
    1: i32 total;
    2: list<ShareMember> share_members;
}

struct UpdateShareMemberPermissionReq {
    1: required i64 MemberId (api.body="member_id");
    2: required i32 Permission (api.body="permission", api.vd="$>=0 && $<=4");
}

struct UpdateShareMemberPermissionResp {
    1: bool success;
}

struct RemoveShareMemberReq {
    1: required i64 MemberId (api.query="member_id");
}

struct RemoveShareMemberResp {
    1: bool success;
}

// services
service ShareService {
    CreateSharePathResp CreateSharePath(1: CreateSharePathReq request) (api.post="/api/share/share_path/*path");
    ListSharePathResp ListSharePath(1: ListSharePathReq request) (api.get="/api/share/share_path/:node/");
    DeleteSharePathResp DeleteSharePath(1: DeleteSharePathReq request) (api.delete="/api/share/share_path/:node/");

    GenerateShareTokenResp GenerateShareToken(1: GenerateShareTokenReq request) (api.post="/api/share/share_token/:node/");
    ListShareTokenResp ListShareToken(1: ListShareTokenReq request) (api.get="/api/share/share_token/:node/");
    RevokeShareTokenResp RevokeShareToken(1: RevokeShareTokenReq request) (api.delete="/api/share/share_token/:node/");

    AddShareMemberResp AddShareMember(1: AddShareMemberReq request) (api.post="/api/share/share_member/:node/");
    ListShareMemberResp ListShareMember(1: ListShareMemberReq request) (api.get="/api/share/share_member/:node/");
    UpdateShareMemberPermissionResp UpdateShareMemberPermission(1: UpdateShareMemberPermissionReq request) (api.put="/api/share/share_member/:node/");
    RemoveShareMemberResp RemoveShareMember(1: RemoveShareMemberReq request) (api.delete="/api/share/share_member/:node/");
}