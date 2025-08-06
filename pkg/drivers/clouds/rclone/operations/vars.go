package operations

var (
	ListPath       = "operations/list"
	MkdirPath      = "operations/mkdir"
	UploadfilePath = "operations/uploadfile"
	CopyfilePath   = "operations/copyfile"
	DeletefilePath = "operations/deletefile"
	StatPath       = "operations/stat"
	SizePath       = "operations/size"

	SyncCopyPath = "sync/copy"
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

	Opt *OperationsOpt `json:"opt,omitempty"`
}

type OperationsOpt struct {
	Metadata bool `json:"metadata,omitempty"`
}

type OperationsCopyFileResp struct {
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
	Async              *bool  `json:"_async,omitempty"`
}
