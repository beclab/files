package http

import (
	"files/pkg/common"
	"files/pkg/models"
	"files/pkg/tasks"
	"net/http"

	"k8s.io/klog/v2"
)

var WrapperTaskArgs = func(prefix string) http.Handler {
	return taskHandle(prefix)
}

func taskHandle(prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var taskParam, _ = models.NewTaskParam(r)

		if r.Method == http.MethodDelete {
			taskCancel(w, taskParam.Owner, taskParam.TaskId, taskParam.Delete, taskParam.All)
		} else {
			taskQuery(w, taskParam.Owner, taskParam.TaskId, taskParam.Status)
		}
	})

	return handler
}

func taskCancel(w http.ResponseWriter, owner string, taskId string, deleted string, all string) {
	_ = deleted
	tasks.TaskManager.CancelTask(owner, taskId, all)

	w.Header().Set("Content-Type", "application/json")
	var data = map[string]interface{}{
		"code": 0,
		"msg":  "success",
	}
	w.Write([]byte(common.ToJson(data)))
}

func taskQuery(w http.ResponseWriter, owner string, taskId string, status string) {
	tasks := tasks.TaskManager.GetTask(owner, taskId, status)
	var data = make(map[string]interface{})
	data["code"] = 0
	data["msg"] = "success"

	if taskId == "" {
		data["tasks"] = tasks
	} else {
		if tasks != nil && len(tasks) > 0 {
			data["task"] = tasks[0]
		} else {
			data["task"] = tasks
		}
	}

	klog.Infof("Task - id: %s, status: %s, data: %s", taskId, status, common.ToJson(data))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(common.ToJson(data)))
}
