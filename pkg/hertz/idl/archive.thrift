namespace go api.archive

struct CompressReq {
    1: required list<string> Sources       (api.body="sources");
    2: required string Destination         (api.body="destination");
    3: string Format                       (api.body="format");
    4: i32    Level                        (api.body="level");
    5: i64    VolumeSizeMB                 (api.body="volumeSizeMB");
    6: bool   PreserveSymlinks             (api.body="preserveSymlinks");
    7: string Conflict                     (api.body="conflict");
    // Password is sent via the X-Archive-Password header; never in
    // the body, so access logs / replays don't capture it.
}

struct CompressResp {
    1: required i32    code,
    2: string msg,
    3: string task_id
}

struct ExtractReq {
    1: required string Source            (api.body="source");
    2: required string Destination       (api.body="destination");
    3: string Format                     (api.body="format");
    4: bool   PreserveSymlinks           (api.body="preserveSymlinks");
    5: string Conflict                   (api.body="conflict");
}

struct ExtractResp {
    1: required i32    code,
    2: string msg,
    3: string task_id
}

struct EntriesReq {
    1: required string Source (api.query="source");
}

struct EntriesResp {
}

struct EntryReq {
    1: required string Source (api.query="source");
    2: required string Path   (api.query="path");
}

struct EntryResp {
}

service ArchiveService {
    CompressResp CompressMethod (1: CompressReq r) (api.post="/api/archive/:node/compress");
    ExtractResp  ExtractMethod  (1: ExtractReq  r) (api.post="/api/archive/:node/extract");
    EntriesResp  EntriesMethod  (1: EntriesReq  r) (api.get="/api/archive/:node/entries");
    EntryResp    EntryMethod    (1: EntryReq    r) (api.get="/api/archive/:node/entry");
}
