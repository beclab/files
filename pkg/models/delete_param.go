package models

type FileDeleteArgs struct {
	FileParam *FileParam `json:"fileParam"`
	Dirents   []string   `json:"dirents"`
}
