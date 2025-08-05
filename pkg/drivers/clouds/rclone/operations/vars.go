package operations

var (
	ListPath       = "operations/list"
	MkdirPath      = "operations/mkdir"
	UploadfilePath = "operations/uploadfile"
	CopyfilePath   = "operations/copyfile"
	StatPath       = "operations/stat"
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

	Opt *OperationsOpt `json:"opt,omitempty"`
}

type OperationsOpt struct {
	Metadata bool `json:"metadata,omitempty"`
}
