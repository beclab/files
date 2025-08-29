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
    12: optional bool readOnly;
    13: optional bool invalid;
    14: optional string idSerial;
    15: optional string idSerialShort;
    16: optional string partitionUUID;
}

struct MountedResp {
    1: required i32 code;
    2: required string message;
    3: optional list<MountedInfo> mountedData;
}

struct MountReq {
    1: required string smbPath (api.body="smbPath");
    2: required string user (api.body="user");
    3: required string password (api.body="password");
}

struct MountResp {
    1: i32 code;
    2: string message;
}

struct UnmountResp {
    1: i32 code;
    2: string message;
}

struct SmbInfo {
    1: optional string url;
    2: optional string username;
    3: optional string password;
    4: optional i64 timestamp;
}

struct GetSmbHistoryResp {
    // only an array now, should change api
    1: optional list<SmbInfo> data;
}

struct PutSmbHistoryReq {
    1: required string url (api.data="url");
    2: optional string username (api.data="username");
    3: optional string password (api.data="password");
}

struct PutSmbHistoryResp {
    // only a string now, should change api
}

struct DeleteSmbHistoryReq {
    1: required string url (api.data="url");
    2: optional string username (api.data="username");
    3: optional string password (api.data="password");
}

struct DeleteSmbHistoryResp {
    // only a string now, should change api
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

service ExternalService {
    MountedResp MountedMethod() (api.get="/api/mounted/*node");
    MountResp MountMethod(1: MountReq request) (api.post="/api/mount/*node");
    UnmountResp UnmountMethod() (api.post="/api/unmount/*path");
    GetSmbHistoryResp GetSmbHistoryMethod() (api.get="/api/smb_history/*node");
    PutSmbHistoryResp PutSmbHistoryMethod(1: PutSmbHistoryReq request) (api.put="/api/smb_history/*node");
    DeleteSmbHistoryResp DeleteSmbHistoryMethod(1: DeleteSmbHistoryReq request) (api.delete="/api/smb_history/*node");
    AccountsResp AccountsMethod() (api.get="/api/accounts");
}