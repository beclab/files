package models

type CompressParam struct {
	Owner       string   `json:"owner"`
	Action      string   `json:"action"`        // "compress", "uncompress"
	Format      string   `json:"format"`        // only in compress
	FileList    []string `json:"file_list"`     // only in compress
	RelPathList []string `json:"rel_path_list"` // only in compress
	SrcPath     string   `json:"src_path"`      // only in uncompress
	DstPath     string   `json:"dst_path"`      // both in compress and uncompress
	TotalSize   int64    `json:"total_size"`    // only in compress
	Override    bool     `json:"override"`      // only in uncompress
}
