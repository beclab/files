package models

type TaskParam struct {
	Owner   string `json:"owner"`
	TaskId  string `json:"taskId"`
	Status  string `json:"status"` // x,y,z
	LogView string `json:"logView"`
	Delete  string `json:"delete"`
	All     string `json:"all"`
}
