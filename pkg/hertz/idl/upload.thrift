namespace go upload

struct UploadLinkReq {
    1: required string FilePath (api.query="file_path");
    2: required string From (api.query="from");
}

struct UploadedBytesReq {
    1: required string ParentDir (api.query="parent_dir");
    2: required string FileName (api.query="file_name");
}

struct UploadedBytesResp {
    1: i64 uploadedBytes
}

struct UploadChunksReq {
    1: required bool retJson (api.query="ret-json");
    2: required i32 resumableChunkNumber (api.form="resumableChunkNumber");
    3: required i32 resumableChunkSize (api.form="resumableChunkSize");
    4: required i32 resumableCurrentChunkSize (api.form="resumableCurrentChunkSize");
    5: required i64 resumableTotalSize (api.form="resumableTotalSize");
    6: string resumableType (api.form="resumableType");
    7: required string resumableIdentifier (api.form="resumableIdentifier");
    8: required string resumableFilename (api.form="resumableFilename");
    9: required string resumableRelativePath (api.form="resumableRelativePath");
    10: required i32 resumableTotalChunks (api.form="resumableTotalChunks");
    11: required string parent_dir (api.form="parent_dir");
    12: required string fullPath (api.form="fullPath");
    13: required string pathname (api.form="pathname");
    14: optional string repoId (api.form="repoId");
    15: required string driveType (api.form="driveType");
    16: required string node (api.form="node");
    17: optional string md5 (api.form="md5");
    18: binary file (api.form="file");
}

union UploadChunksResp {
    1: UploadChunksSuccess success
    2: list<UploadChunksFileItem> items
}

struct UploadChunksSuccess {
    1: bool success
}

struct UploadChunksFileItem {
    1: string id
    2: string name
    3: i64 size
    4: optional string state
    5: optional string taskId
}

service UploadService {
    UploadChunksResp UploadChunksMethod(1: UploadChunksReq request) (api.post="/upload/upload-link/:node/:uid");
    string UploadLinkMethod(1: UploadLinkReq request) (api.get="/upload/upload-link/:node/");
    UploadedBytesResp UploadedBytesMethod(1: UploadedBytesReq request) (api.get="/upload/file-uploaded-bytes/:node/");
}
