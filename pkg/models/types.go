package models

type HttpContextArgs struct {
	FileParam  *FileParam  `json:"fileParam"`
	QueryParam *QueryParam `json:"queryParam"`
}
