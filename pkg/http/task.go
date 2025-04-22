package http

import (
	"files/pkg/common"
	"files/pkg/pool"
	"k8s.io/klog/v2"
	"net/http"
)

func resourceTaskGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	taskID := r.URL.Query().Get("task_id")
	var t *pool.Task
	if storedTask, ok := pool.TaskManager.Load(taskID); ok {
		if t, ok = storedTask.(*pool.Task); ok {
			klog.Infof("Task %s Infos: %v\n", t.ID, t)
			klog.Infof("Task %s Progress: %d%%\n", t.ID, t.GetProgress())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	return common.RenderJSON(w, r, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"task": t,
	})
}
