package tasks

type TaskInfo struct {
	Id            string `json:"id"`
	Action        string `json:"action"`
	IsDir         bool   `json:"is_dir"`
	FileName      string `json:"filename"`
	Dst           string `json:"dest"`
	DstPath       string `json:"dst_filename"`
	DstFileType   string `json:"dst_type"`
	Src           string `json:"source"`
	SrcFileType   string `json:"src_type"`
	Progress      int    `json:"progress"`
	Transferred   int64  `json:"transferred"`
	TotalFileSize int64  `json:"total_file_size"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"failed_reason"`
}
