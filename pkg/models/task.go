package models

import "time"

type TaskQueryResponse struct {
	StatusCode string      `json:"status_code"`
	FailReason string      `json:"fail_reason"`
	Data       []*TaskData `json:"data"`
}

type TaskResponse struct {
	StatusCode string    `json:"status_code"`
	FailReason string    `json:"fail_reason"`
	Data       *TaskData `json:"data"`
}

type TaskData struct {
	ID            string          `json:"id"`
	TaskType      string          `json:"task_type"`
	Status        string          `json:"status"`
	Progress      float64         `json:"progress"`
	TaskParameter *TaskParameter  `json:"task_parameter"`
	PauseInfo     *TaskPauseInfo  `json:"pause_info"`
	ResultData    *TaskResultData `json:"result_data"`
	UserName      string          `json:"user_name"`
	DriverName    string          `json:"driver_name"`
	FailedReason  string          `json:"failed_reason"`
	WorkerName    string          `json:"worker_name"`
	CreatedAt     int64           `json:"created_at"`
	UpdatedAt     int64           `json:"updated_at"`
}

type TaskParameter struct {
	Drive         string `json:"drive"`
	LocalFilePath string `json:"local_file_path"`
	Name          string `json:"name"`
	ParentPath    string `json:"parent_path"`
}

type TaskPauseInfo struct {
	FileSize  int64  `json:"file_size"`
	Location  string `json:"location"`
	NextStart int64  `json:"next_start"`
}

type TaskResultData struct {
	FileInfo                 *TaskFileInfo `json:"file_info,omitempty"`
	UploadFirstOperationTime int64         `json:"upload_first_operation_time"`
}

type TaskFileInfo struct {
	Path         string                       `json:"path"`
	Name         string                       `json:"name"`
	Size         int64                        `json:"size"`
	FileSize     int64                        `json:"fileSize"`
	Extension    string                       `json:"extension"`
	Modified     time.Time                    `json:"modified"`
	Mode         string                       `json:"mode"`
	IsDir        bool                         `json:"isDir"`
	IsSymlink    bool                         `json:"isSymlink"`
	Type         string                       `json:"type"`
	Meta         *GoogleDriveResponseDataMeta `json:"meta,omitempty"`
	CanDownload  bool                         `json:"canDownload"`
	CanExport    bool                         `json:"canExport"`
	ExportSuffix string                       `json:"exportSuffix"`
	IdPath       string                       `json:"id_path,omitempty"`
}
