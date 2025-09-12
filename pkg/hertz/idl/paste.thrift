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
    2: optional string Status (api.query="status");
    3: optional i32 LogView (api.query="log_view");
}

struct TaskInfo {
    1: string id,
    2: string action,
    3: bool is_dir,
    4: string filename,
    5: string dest,
    6: string dst_filename,
    7: string source,
    8: i32 current_phase,
    9: i32 total_phases,
    10: i32 progress,
    11: i64 transferred,
    12: i64 total_file_size,
    13: bool tidy_dirs,
    14: string status,
    15: string failed_reason,
    16: bool pause_able,
    17: string drive_id
}

struct GetTaskResp {
    1: required i32 code,
    2: optional string msg,
    3: optional list<TaskInfo> tasks
    4: optional TaskInfo task
}

struct DeleteTaskReq {
    1: string TaskId (api.query="task_id");
    2: i32 Delete (api.query="delete");
    3: optional i32 All (api.query="all");
}

struct DeleteTaskResp {
    1: required i32 code,
    2: optional string msg
}

struct PauseResumeTaskReq {
    1: string TaskId (api.query="task_id");
    2: string Op (api.query="op");
}

struct PauseResumeTaskResp {
    1: required i32 code,
    2: optional string msg
}

service PasteService {
    PasteResp PasteMethod(1: PasteReq request) (api.patch="/api/paste/:node/");
    GetTaskResp GetTaskMethod(1: GetTaskReq request) (api.get="/api/task/:node/");
    DeleteTaskResp DeleteTaskMethod(1: DeleteTaskReq request) (api.delete="/api/task/:node/");
    PauseResumeTaskResp PauseResumeTaskMethod(1: PauseResumeTaskReq request) (api.post="/api/task/:node/");
}
