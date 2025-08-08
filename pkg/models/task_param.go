package models

import (
	"errors"
	"files/pkg/utils"
	"net/http"
)

type TaskParam struct {
	Owner   string `json:"owner"`
	TaskId  string `json:"taskId"`
	Status  string `json:"status"`
	LogView string `json:"logView"`
	Delete  string `json:"delete"`
}

func NewTaskParam(r *http.Request) (*TaskParam, error) {
	var owner = r.Header.Get(utils.REQUEST_HEADER_OWNER)
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var taskParam = &TaskParam{
		Owner:   owner,
		TaskId:  r.URL.Query().Get("task_id"),
		Status:  r.URL.Query().Get("status"),
		LogView: r.URL.Query().Get("log_view"),
		Delete:  r.URL.Query().Get("delete"),
	}

	return taskParam, nil
}
