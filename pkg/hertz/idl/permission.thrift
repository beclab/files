namespace go api.permission

struct GetPermissionResp {
    1: required i32 uid;
}

struct PutPermissionReq {
    1: required i32 Uid (api.query="uid");
    2: optional i32 Recursive (api.query="recursive");
}

struct PutPermissionResp {
}

service PermissionService {
    GetPermissionResp GetPermissionMethod() (api.get="/api/permission/*path");
    PutPermissionResp PutPermissionMethod(1: PutPermissionReq request) (api.put="/api/permission/*path");
}