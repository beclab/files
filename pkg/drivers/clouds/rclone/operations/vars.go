package operations

var (
	ListPath       = "operations/list"
	MkdirPath      = "operations/mkdir"
	UploadfilePath = "operations/uploadfile"
	CopyfilePath   = "operations/copyfile"
	Movefilepath   = "operations/movefile"
	DeletePath     = "operations/delete"
	DeletefilePath = "operations/deletefile"
	RmDirsPath     = "operations/rmdirs"
	PurgePath      = "operations/purge"
	StatPath       = "operations/stat"
	SizePath       = "operations/size"
	AboutPath      = "operations/about"

	SyncCopyPath = "sync/copy"
	SyncMovePath = "sync/move"

	FsCacheClearPath = "fscache/clear"
)

type OperationsStat struct {
	Item *OperationsListItem `json:"item"`
}

type OperationsList struct {
	List []*OperationsListItem `json:"list"`
}

type OperationsListItem struct {
	Path     string                      `json:"Path"`
	Name     string                      `json:"Name"`
	Size     int64                       `json:"Size"`
	MimeType string                      `json:"MimeType"`
	ModTime  string                      `json:"ModTime"`
	IsDir    bool                        `json:"IsDir"`
	Tier     string                      `json:"Tier"`
	Metadata *OperationsListItemMetadata `json:"Metadata"`
	ID       string                      `json:"ID"`
}

type OperationsListItemMetadata struct {
	Btime       string `json:"btime"`
	ContentType string `json:"content-type"`
	Tier        string `json:"tier"`
}

// operations request
type OperationsReq struct {
	Fs     string `json:"fs"`
	Remote string `json:"remote"`
	Source string `json:"source,omitempty"`

	SrcFs     string `json:"srcFs,omitempty"`
	SrcRemote string `json:"srcRemote,omitempty"`
	DstFs     string `json:"dstFs,omitempty"`
	DstRemote string `json:"dstRemote,omitempty"`
	Async     *bool  `json:"_async,omitempty"`
	LeaveRoot *bool  `json:"leaveRoot,omitempty"`

	Opt    *OperationsOpt    `json:"opt,omitempty"`
	Filter *OperationsFilter `json:"_filter,omitempty"`
}

type OperationsOpt struct {
	Recurse    bool `json:"recurse"`
	NoModTime  bool `json:"noModTime"`
	NoMimeType bool `json:"noMimeType"`
	DirsOnly   bool `json:"dirsOnly"`
	FilesOnly  bool `json:"filesOnly"`
	ShowHash   bool `json:"showHash"`
	Metadata   bool `json:"metadata"`
}

type OperationsFilter struct {
	MaxSize     string   `json:"MaxSize,omitempty"`
	MinSize     string   `json:"MinSize,omitempty"`
	MaxAge      string   `json:"MaxAge,omitempty"`
	MinAge      string   `json:"MinAge,omitempty"`
	IncludeRule []string `json:"IncludeRule,omitempty"`
	ExcludeRule []string `json:"ExcludeRule,omitempty"`
	FilterRule  []string `json:"FilterRule,omitempty"`
	IgnoreCase  bool     `json:"IgnoreCase"`
}

type OperationsCopyFileResp struct {
	JobId *int `json:"jobid,omitempty"`
}

type OperationsAsyncJobResp struct {
	JobId *int `json:"jobid,omitempty"`
}

/*
*
  - {
    "bytes": 42128571,
    "count": 30,
    "sizeless": 0
    }
*/
type OperationsSizeResp struct {
	Bytes    int64 `json:"bytes"`
	Count    int64 `json:"count"`
	Sizeless int64 `json:"sizeless"`
}

// sync
type SyncCopyReq struct {
	SrcFs              string `json:"srcFs"`
	DstFs              string `json:"dstFs"`
	CreateEmptySrcDirs bool   `json:"createEmptySrcDirs"`
	DeleteEmptySrcDirs bool   `json:"deleteEmptySrcDirs"`
	Async              *bool  `json:"_async,omitempty"`
}

type OperationsAboutResp struct {
	Free  int64 `json:"free"`
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
}
