namespace go api.paste

struct PasteReq {
    1: required string Action (api.body="action");
    2: required string Source (api.body="source");
    3: required string Destination (api.body="destination");
}

struct PasteResp {
    1: string task_id
}

struct GetTaskReq {
    1: string TaskId (api.query="task_id");
}

struct TaskInfo {
    1: string id,
    2: string action,
    3: bool is_dir,
    4: string filename,
    5: string dest,
    6: string dst_filename,
    7: string dst_type,
    8: string source,
    9: string src_type,
    10: i32 current_phase,
    11: i32 total_phases,
    12: i32 progress,
    13: i64 transferred,
    14: i64 total_file_size,
    15: bool tidy_dirs,
    16: string status,
    17: string failed_reason
}

struct GetTaskResp {
    1: required i32 code,
    2: optional string msg,
    3: optional list<TaskInfo> tasks
    4: optional TaskInfo task
}

struct DeleteTaskReq {
    1: string TaskId (api.query="task_id");
}

struct DeleteTaskResp {
    1: required i32 code,
    2: optional string msg
}

service PasteService {
    PasteResp PasteMethod(1: PasteReq request) (api.patch="/api/paste/*node");
    GetTaskResp GetTaskMethod(1: GetTaskReq request) (api.get="/api/task/*node");
    DeleteTaskResp DeleteTaskMethod(1: DeleteTaskReq request) (api.delete="/api/task/*node");
}