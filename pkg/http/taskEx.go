package http

import (
	"files/pkg/models"
	"files/pkg/tasks"
	"files/pkg/utils"
	"net/http"
)

var wrapperTaskArgs = func(prefix string) http.Handler {
	return taskHandle(prefix)
}

func taskHandle(prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var taskParam, _ = models.NewTaskParam(r)

		if taskParam.TaskId == "" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(""))
			// return common.RenderJSON(w, r, map[string]interface{}{
			// 	"code":  0,
			// 	"msg":   "success",
			// 	"tasks": tasks,
			// })
		} else {
			task := tasks.TaskManager.GetTask(taskParam.TaskId)
			var data = make(map[string]interface{})
			data["code"] = 0
			data["msg"] = "success"
			data["task"] = task

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(utils.ToJson(data)))
		}
	})

	return handler
}
