package models

import (
	"errors"
	"files/pkg/common"
	"net/http"
)

type TaskParam struct {
	Owner   string `json:"owner"`
	TaskId  string `json:"taskId"`
	Status  string `json:"status"` // x,y,z
	LogView string `json:"logView"`
	Delete  string `json:"delete"`
	All     string `json:"all"`
	Op      string `json:"op"`
}

func NewTaskParam(r *http.Request) (*TaskParam, error) {
	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var taskParam = &TaskParam{
		Owner:   owner,
		TaskId:  r.URL.Query().Get("task_id"),
		Status:  r.URL.Query().Get("status"),
		LogView: r.URL.Query().Get("log_view"),
		Delete:  r.URL.Query().Get("delete"),
		All:     r.URL.Query().Get("all"),
		Op:      r.URL.Query().Get("op"),
	}

	return taskParam, nil
}
