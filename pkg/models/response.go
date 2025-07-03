package models

type PreviewHandlerResponse struct {
	FileName     string `json:"file_name"`
	FileModified string `json:"file_modified"`
	Data         []byte `json:"file_data"`
	Error        error  `json:"error"`
}
