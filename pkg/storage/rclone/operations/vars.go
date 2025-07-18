package operations

var (
	ListPath = "operations/list"
)

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
}

type OperationsListItemMetadata struct {
	Btime       string `json:"btime"`
	ContentType string `json:"content-type"`
	Tier        string `json:"tier"`
}

// list
type OperationsListReq struct {
	Fs     string `json:"fs"`
	Remote string `json:"remote"`
	Opt    struct {
		Metadata bool `json:"metadata"`
	} `json:"opt"`
}
