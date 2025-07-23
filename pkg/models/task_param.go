package models

import (
	"errors"
	"files/pkg/constant"
	"net/http"
)

type TaskParam struct {
	Owner   string `json:"owner"`
	TaskId  string `json:"taskId"`
	Status  string `json:"status"`
	LogView string `json:"logView"`
}

func NewTaskParam(r *http.Request) (*TaskParam, error) {
	var owner = r.Header.Get(constant.REQUEST_HEADER_OWNER)
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var taskParam = &TaskParam{
		Owner:   owner,
		TaskId:  r.URL.Query().Get("task_id"),
		Status:  r.URL.Query().Get("status"),
		LogView: r.URL.Query().Get("log_view"),
	}

	return taskParam, nil
}
