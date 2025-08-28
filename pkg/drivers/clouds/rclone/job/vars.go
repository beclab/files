package job

var (
	JobStatusPath = "job/status"
	JobStopPath   = "job/stop"
	JobListPath   = "job/list"
	CoreStats     = "core/stats"
)

type JobListResp struct {
	ExecuteId string `json:"executeId"`
	JobIds    []int  `json:"jobids"`
}

type JobStatusReq struct {
	JobId int `json:"jobid"`
}

type JobStatusResp struct {
	Id        int         `json:"id"`
	Group     string      `json:"group"`
	StartTime string      `json:"startTime"`
	EndTime   string      `json:"endTime"`
	Duration  float64     `json:"duration"`
	Success   bool        `json:"success"`
	Finished  bool        `json:"finished"`
	Error     string      `json:"error"`
	Path      string      `json:"path,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	Output    interface{} `json:"output,omitempty"`
}

type CoreStatsReq struct {
	Group string `json:"group"` // job/xxx
}

type CoreStatsResp struct {
	Bytes               int64                   `json:"bytes"`
	Checks              int64                   `json:"checks"`
	DeletedDirs         int64                   `json:"deletedDirs"`
	Deletes             int64                   `json:"deletes"`
	ElapsedTime         float64                 `json:"elapsedTime"`
	Errors              int64                   `json:"errors"`
	Eta                 int64                   `json:"eta"`
	FatalError          bool                    `json:"fatalError"`
	Listed              int64                   `json:"listed"`
	Renames             int64                   `json:"renames"`
	RetryError          bool                    `json:"retryError"`
	ServerSideCopies    int64                   `json:"serverSideCopies"`
	ServerSideCopyBytes int64                   `json:"serverSideCopyBytes"`
	ServerSideMoveBytes int64                   `json:"serverSideMoveBytes"`
	ServerSideMoves     int64                   `json:"serverSideMoves"`
	Speed               float64                 `json:"speed"`
	TotalBytes          int64                   `json:"totalBytes"`
	TotalChecks         int64                   `json:"totalChecks"`
	TotalTransfers      int64                   `json:"totalTransfers"`
	TransferTime        float64                 `json:"transferTime"`
	Transfers           int64                   `json:"transfers"`
	Transferring        []*CoreStatTransferring `json:"transferring"`
}

type CoreStatTransferring struct {
	Bytes      int64   `json:"bytes"`
	DstFs      string  `json:"dstFs"`
	Eta        int64   `json:"eta"`
	Group      string  `json:"group"`
	Name       string  `json:"name"`
	Percentage int64   `json:"percentage"`
	Size       int64   `json:"size"`
	Speed      float64 `json:"speed"`
	SpeedAvg   float64 `json:"speedAvg"`
	SrcFs      string  `json:"srcFs"`
}
