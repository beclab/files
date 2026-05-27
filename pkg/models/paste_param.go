package models

type PasteParam struct {
	Owner                   string `json:"owner"`
	Action                  string `json:"action"`
	UploadToCloud           bool   `json:"uploadToCloud"`
	UploadToCloudParentPath string `json:"uploadToCloudParentPath"`
	Src                     *FileParam
	Dst                     *FileParam
	Temp                    *FileParam
	Share                   int    `json:"share"`
	SrcShareType            string `json:"srcShareType"`
	DstShareType            string `json:"dstShareType"`
	SrcOwner                string `json:"srcOwner"`
	DstOwner                string `json:"dstOwner"`
	SrcSharePath            *FileParam
	DstSharePath            *FileParam

	// Srcs is populated for ActionCompress only, carrying the list of
	// sources to archive together.
	Srcs []*FileParam `json:"-"`
	// Archive is populated for ActionCompress / ActionExtract only.
	Archive *ArchiveOption `json:"-"`
}
