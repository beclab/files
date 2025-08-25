package models

type PasteReq struct {
	Owner       string `json:"owner"`
	Extend      string `json:"extend"`
	Action      string `json:"action"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type PasteParam struct {
	Owner         string `json:"owner"`
	Action        string `json:"action"`
	UploadToCloud bool   `json:"uploadToCloud"`
	Src           *FileParam
	Dst           *FileParam
	Temp          *FileParam
	Delete        *FileParam
}
