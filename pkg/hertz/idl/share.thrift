namespace go api.share

// gorm models
struct SharePath {
    1: required string id (go.tag = 'gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"')
    2: required string owner (go.tag = 'gorm:"column:owner;type:text;not null"')
    3: required string file_type (go.tag = 'gorm:"column:file_type;type:varchar(10);not null"')
    4: required string extend (go.tag = 'gorm:"column:extend;type:text;not null"')
    5: required string path (go.tag = 'gorm:"column:path;type:text;not null"')
    6: required string share_type (go.tag = 'gorm:"column:share_type;type:varchar(10);not null"')
    7: required string name (go.tag = 'gorm:"column:name;type:text"')
    8: string password_md5 (go.tag = 'gorm:"column:password_md5;type:varchar(150)"')
    /* millisecond */
    9: required i64 expire_in (go.tag = 'gorm:"column:expire_in;not null"')
    10: required string expire_time (go.tag = 'gorm:"column:expire_time;type:timestamptz;not null"')
    11: required i32 permission (go.tag = 'gorm:"column:permission;not null;default:0"')
    12: i32 smb_share_public (go.tag = 'gorm:"column:smb_share_public;not null;default:0"')
    13: required string create_time (go.tag = 'gorm:"column:create_time;type:timestamptz;not null;autoCreateTime:milli"')
    14: required string update_time (go.tag = 'gorm:"column:update_time;type:timestamptz;not null;autoUpdateTime:milli"')
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

struct ShareSmbUser {
    1: required i64 id (go.tag = 'gorm:"column:id;primaryKey;autoIncrement"')
    2: required string owner (go.tag = 'gorm:"column:owner;type:varchar(100);not null"')
    3: required string user_id (go.tag = 'gorm:"column:user_id;type:uuid;not null"')
    4: required string user_name (go.tag = 'gorm:"column:user_name;type:varchar(24);not null;unique"')
    5: required string password (go.tag = 'gorm:"column:password;type:varchar(250)"')
    6: required string create_time (go.tag = 'gorm:"column:create_time;type:timestamptz;not null;autoCreateTime"')
    7: required string update_time (go.tag = 'gorm:"column:update_time;type:timestamptz;not null;autoUpdateTime"')
}

struct ShareSmbMember {
    1: required i64 id (go.tag = 'gorm:"column:id;primaryKey;autoIncrement"')
    2: required string owner (go.tag = 'gorm:"column:owner;type:varchar(100);not null"')
    3: required string path_id (go.tag = 'gorm:"column:path_id;type:uuid;not null"')
    4: required string user_id (go.tag = 'gorm:"column:user_id;type:uuid;not null"')
    /* permission = 1 or 3 */
    5: required i32 permission (go.tag = 'gorm:"column:permission;not null"')
    6: required string create_time (go.tag = 'gorm:"column:create_time;type:timestamptz;not null;autoCreateTime"')
    7: required string update_time (go.tag = 'gorm:"column:update_time;type:timestamptz;not null;autoUpdateTime"')
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
    14: string smb_link (go.tag='json:"smb_link,omitempty"')
    15: string smb_user (go.tag='json:"smb_user,omitempty"')
    16: string smb_password (go.tag='json:"smb_password,omitempty"')
}

struct CreateSharePathReq {
    // path is in the URL
    1: required string share_type (api.body="share_type", api.vd="($ == 'internal'||$ == 'external'||$ == 'smb')");
    2: string name (api.body="name");
    3: string password (api.body="password");
    4: i64 expire_in (api.body="expire_in");
    5: string expire_time (api.body="expire_time");
    6: i32 permission (api.body="permission", api.vd="$>=0 && $<=4");
    7: list<CreateSmbSharePathMembers> users (api.body="users");
}

struct CreateSmbSharePathMembers {
    1: string id;
    2: i32 permission (api.body="permission", api.vd="$>=0 && $<=4");
}

struct CreateSharePathResp {
    1: ViewSharePath share_path;
}

struct ListSharePathReq {
    1: string PathId (api.query="path_id");
    2: string ShareRelativeUser (api.query="share_relative_user");
    3: string FileType (api.query="file_type");
    4: string Extend (api.query="extend");
    5: string Path (api.query="path");
    6: string ShareType (api.query="share_type");
    7: string Name (api.query="name");
    8: string Permission (api.query="permission");
    9: i64 ExpireIn (api.query="expire_in");
    10: optional bool SharedWithMe (api.query="shared_with_me");
    11: optional bool SharedByMe (api.query="shared_by_me");
}

struct ListSharePathResp {
    1: i32 total;
    2: list<ViewSharePath> share_paths;
}

struct GetExternalSharePathReq {
    1: required string PathId (api.query="path_id");
    2: required string Token (api.query="token");
}

struct GetExternalSharePathResp {}

struct GetInternalSmbSharePathResp {}

struct UpdateSharePathReq {
    1: required string PathId (api.body="path_id");
    2: required string Name (api.body="name");
}

struct UpdateSharePathResp {
    1: ViewSharePath share_path;
}

struct DeleteSharePathReq {
    1: string PathIds (api.query="path_ids");
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
    1: string PathId (api.query="path_id");
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

struct GetTokenReq {
    1: required string ShareId (api.body="id");
    2: required string Password (api.body="pass");
}

struct GetTokenResp {
}

struct AddShareMemberInfo {
    1: required string ShareMember (api.body="share_member");
    2: required i32 Permission (api.body="permission", api.vd="$>=0 && $<=4");
}

struct AddShareMemberReq {
    1: required string PathId (api.body="path_id");
    2: required list<AddShareMemberInfo> ShareMembers (api.body="share_members");
}

struct AddShareMemberResp {
    1: list<ShareMember> created;
    2: list<ShareMember> updated;
    3: list<ShareMember> existed;
}

struct ListShareMemberReq {
    1: string PathId (api.query="path_id");
    2: string ShareMember (api.query="share_member");
    3: string Permission (api.query="permission");  // for multi-filtering
}

struct ListShareMemberResp {
    1: i32 total;
    2: list<ShareMember> share_members;
}

struct UpdateShareMemberInfo {
    1: required i64 MemberId (api.body="member_id");
    2: required i32 Permission (api.body="permission", api.vd="$>=0 && $<=4");
}

struct UpdateShareMemberPermissionReq {
    1: required list<UpdateShareMemberInfo> ShareMembers (api.body="share_members");
}

struct UpdateShareMemberPermissionResp {
    1: list<ShareMember> updated;
    2: list<ShareMember> existed;
    3: list<UpdateShareMemberInfo> not_existed;
    4: list<UpdateShareMemberInfo> share_deleted;
}

struct RemoveShareMemberReq {
    1: required string MemberIds (api.query="member_ids");
}

struct RemoveShareMemberResp {
    1: bool success;
}

struct SmbAccount {
  1: string user;
  2: string password;
}

struct SmbCreate {
  1: string owner;
  2: string id;
  3: string path;
  4: string user;
}

struct ResetPasswordReq {
  1: required string pathId (api.body="path_id");
  2: string user (api.body="user");
  3: required string password (api.body="password");
}

struct ResetPasswordResp {}

struct ListSmbUserReq {}
struct ListSmbUserResp {}

struct CreateSmbUserReq {
    1: required string user (api.body="user");
    2: required string password (api.body="password");
}
struct CreateSmbUserResp {}

struct DeleteSmbUserReq {
    1: required list<string> users (api.body="users");
}
struct DeleteSmbUserResp {}

struct ModifySmbMemberReq {
    1: required string PathId (api.body="path_id");
    2: list<CreateSmbSharePathMembers> Users (api.body="users");
}
struct ModifySmbMemberResp {}

struct SmbShareView {
    1: string id (go.tag = 'gorm:"column:id"');
    2: string owner (go.tag = 'gorm:"column:owner"');
    3: string fileType (go.tag = 'gorm:"column:file_type"');
    4: string extend (go.tag = 'gorm:"column:extend"');
    5: string path (go.tag = 'gorm:"column:path"');
    6: string shareType (go.tag = 'gorm:"column:share_type"');
    7: string name (go.tag = 'gorm:"column:name"');
    8: i64 expireIn (go.tag = 'gorm:"column:expire_in"');
    9: string expireTime (go.tag = 'gorm:"column:expire_time"');
    10: i32 sharePermission (go.tag = 'gorm:"column:share_permission"');
    11: i32 smbSharePublic (go.tag = 'gorm:"column:smb_share_public"');
    12: string userId (go.tag = 'gorm:"column:user_id"');
    13: string userName (go.tag = 'gorm:"column:user_name"');
    14: string password (go.tag = 'gorm:"column:password"');
    15: i32 permission (go.tag = 'gorm:"column:permission"');
}

// services
service ShareService {
    CreateSharePathResp CreateSharePath(1: CreateSharePathReq request) (api.post="/api/share/share_path/*path");
    ListSharePathResp ListSharePath(1: ListSharePathReq request) (api.get="/api/share/share_path/");
    UpdateSharePathResp UpdateSharePath(1: UpdateSharePathReq request) (api.put="/api/share/share_path/");
    DeleteSharePathResp DeleteSharePath(1: DeleteSharePathReq request) (api.delete="/api/share/share_path/");
    ResetPasswordResp ResetPassword(1: ResetPasswordReq request) (api.put="/api/share/share_password/"); /* todo */
    GetExternalSharePathResp GetExternalSharePath(1: GetExternalSharePathReq request) (api.get="/api/share/get_share/");
    GetInternalSmbSharePathResp GetInternalSmbSharePath() (api.get="/api/share/get_share_internal_smb/*path");

    GenerateShareTokenResp GenerateShareToken(1: GenerateShareTokenReq request) (api.post="/api/share/share_token/");
    ListShareTokenResp ListShareToken(1: ListShareTokenReq request) (api.get="/api/share/share_token/");
    RevokeShareTokenResp RevokeShareToken(1: RevokeShareTokenReq request) (api.delete="/api/share/share_token/");
    GetTokenResp GetToken(1: GetTokenReq request) (api.post="/api/share/get_token/");

    AddShareMemberResp AddShareMember(1: AddShareMemberReq request) (api.post="/api/share/share_member/");
    ListShareMemberResp ListShareMember(1: ListShareMemberReq request) (api.get="/api/share/share_member/");
    UpdateShareMemberPermissionResp UpdateShareMemberPermission(1: UpdateShareMemberPermissionReq request) (api.put="/api/share/share_member/");
    RemoveShareMemberResp RemoveShareMember(1: RemoveShareMemberReq request) (api.delete="/api/share/share_member/");

    /* samba */
    ListSmbUserResp ListSmbUser(1: ListSmbUserReq request) (api.get="/api/share/smb_share_user/");
    CreateSmbUserResp CreateSmbUser(1: CreateSmbUserReq request) (api.post="/api/share/smb_share_user/");
    DeleteSmbUserResp DeleteSmbUser(1: DeleteSmbUserReq request) (api.delete="/api/share/smb_share_user/");
    ModifySmbMemberResp ModifySmbMember(1: ModifySmbMemberReq request) (api.post="/api/share/smb_share_member/");
}