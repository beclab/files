namespace go api.archive

// Archive write endpoints. Streaming preview (entries / entry) is NOT
// expressed in thrift because its response is NDJSON / octet-stream;
// it is registered as a bare handler in
// pkg/hertz/biz/router/api/archive.

struct CompressReq {
    1: required list<string> Sources       (api.body="sources");
    2: required string Destination         (api.body="destination");
    3: optional string Format              (api.body="format");
    4: optional i32    Level               (api.body="level");
    5: optional i64    VolumeSizeMB        (api.body="volumeSizeMB");
    6: optional bool   PreserveSymlinks    (api.body="preserveSymlinks");
    7: optional string Conflict            (api.body="conflict");
    // Password is sent via the X-Archive-Password header; never in
    // the body, so access logs / replays don't capture it.
}

struct CompressResp {
    1: required i32    code,
    2: optional string msg,
    3: optional string task_id
}

struct ExtractReq {
    1: required string Source            (api.body="source");
    2: required string Destination       (api.body="destination");
    3: optional string Format            (api.body="format");
    4: optional bool   PreserveSymlinks  (api.body="preserveSymlinks");
    5: optional string Conflict          (api.body="conflict");
}

struct ExtractResp {
    1: required i32    code,
    2: optional string msg,
    3: optional string task_id
}

service ArchiveService {
    CompressResp CompressMethod (1: CompressReq r) (api.post="/api/archive/:node/compress");
    ExtractResp  ExtractMethod  (1: ExtractReq  r) (api.post="/api/archive/:node/extract");
}
