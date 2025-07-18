package http

import (
	"files/pkg/models"
	"files/pkg/tasks"
	"files/pkg/utils"
	"net/http"

	"k8s.io/klog/v2"
)

var wrapperTaskArgs = func(prefix string) http.Handler {
	return taskHandle(prefix)
}

func taskHandle(prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var taskParam, _ = models.NewTaskParam(r)

		if r.Method == http.MethodDelete {
			taskCancel(w, taskParam.TaskId, taskParam.Delete)
		} else {
			taskQuery(w, taskParam.TaskId)
		}
	})

	return handler
}

func taskCancel(w http.ResponseWriter, taskId string, deleted string) {
	_ = deleted
	tasks.TaskManager.CancelTask(taskId)

	w.Header().Set("Content-Type", "application/json")
	var data = map[string]interface{}{
		"code": 0,
		"msg":  "success",
	}
	w.Write([]byte(utils.ToJson(data)))
}

func taskQuery(w http.ResponseWriter, taskId string) {
	if taskId == "" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
		// return common.RenderJSON(w, r, map[string]interface{}{
		// 	"code":  0,
		// 	"msg":   "success",
		// 	"tasks": tasks,
		// })
	} else {
		task := tasks.TaskManager.GetTask(taskId)
		var data = make(map[string]interface{})
		data["code"] = 0
		data["msg"] = "success"
		data["task"] = task

		klog.Infof("Task - data: %s", utils.ToJson(data))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(utils.ToJson(data)))
	}
}
