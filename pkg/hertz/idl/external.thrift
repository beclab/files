namespace go api.external

struct MountedInfo {
    1: optional string type;
    2: optional string path;
    3: optional string fstype;
    4: optional i64 total;
    5: optional i64 free;
    6: optional i64 used;
    7: optional double usedPercent;
    8: optional i64 inodesTotal;
    9: optional i64 inodesUsed;
    10: optional i64 inodesFree;
    11: optional double inodesUsedPercent;
    12: optional bool read_only;
    13: optional bool invalid;
    14: optional string id_serial;
    15: optional string id_serial_short;
    16: optional string partition_uuid;
}

struct MountedResp {
    1: required i32 code;
    2: required string message;
    3: required list<MountedInfo> mountedData;
}

struct MountReq {
    1: required string smbPath (api.body="smbPath");
    2: optional string user (api.body="user");
    3: optional string password (api.body="password");
    4: required string externalType (api.query="external_type");
}

struct MountPath {
    1: bool mounted;
    2: string path;
}

struct MountResp {
    1: i32 code;
    2: string message;
    3: list<MountPath> data;
}

struct UnmountReq {
    1: required string externalType (api.query="external_type");
}

struct UnmountResp {
    1: i32 code;
    2: string message;
}

struct SmbInfo {
    1: string url;
    2: string username;
    3: string password;
    4: i64 timestamp;
}

struct GetSmbHistoryResp {
    1: list<SmbInfo> data;
}

struct SmbHistoryInfo {
    1: required string url (api.data="url");
    2: string username (api.data="username");
    3: string password (api.data="password");
}

struct PutSmbHistoryReq {
    1: required list<SmbHistoryInfo> data;
}

struct DeleteSmbHistoryReq {
    1: required list<SmbHistoryInfo> data;
}

struct AccountInfo {
    1: string name;
    2: string type;
    3: bool available;
    4: i64 create_at;
    5: i64 expires_at;
}

struct AccountsResp {
    1: required i32 code,
    2: optional string msg,
    3: list<AccountInfo> data
}

struct MountedStates {
    1: string type,
    2: string path,
    3: string fstype,
    4: i64 total,
    5: i64 free,
    6: i64 used,
    7: double usedPercent,
    8: i64 inodesTotal,
    9: i64 inodesUsed,
    10: i64 inodesFree,
    11: double inodesUsedPercent,
    12: bool read_only,
    13: bool invalid,
    14: string id_serial,
    15: string id_serial_short,
    16: string partition_uuid
}

struct MountedStatesResp {
}

service ExternalService {
    MountedResp MountedMethod() (api.get="/api/mounted/:node/");
    MountResp MountMethod(1: MountReq request) (api.post="/api/mount/:node/");
    UnmountResp UnmountMethod(1: UnmountReq request) (api.post="/api/unmount/*path");
    GetSmbHistoryResp GetSmbHistoryMethod() (api.get="/api/smb_history/:node/");
    string PutSmbHistoryMethod(1: PutSmbHistoryReq request) (api.put="/api/smb_history/:node/");
    string DeleteSmbHistoryMethod(1: DeleteSmbHistoryReq request) (api.delete="/api/smb_history/:node/");
    AccountsResp AccountsMethod() (api.get="/api/accounts");
    MountedStatesResp ReportMountedStates(1: list<MountedInfo> req) (api.post="/api/mounted_states/");
}
