package http

import (
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/pool"
	"fmt"
	"k8s.io/klog/v2"
	"net/http"
)

func resourceTaskGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	taskID := r.URL.Query().Get("task_id")
	var t *pool.Task
	if storedTask, ok := pool.TaskManager.Load(taskID); ok {
		if t, ok = storedTask.(*pool.Task); ok {
			klog.Infof("Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
			klog.Infof("Task %s Progress: %d%%\n", t.ID, t.GetProgress())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	return common.RenderJSON(w, r, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"task": pool.FormattedTask{Task: *t},
	})
}

func resourceTaskDeleteHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	taskID := r.URL.Query().Get("task_id")
	klog.Infof("~~~Debug log: cancel Task %s\n", taskID)
	var t *pool.Task

	if storedTask, ok := pool.TaskManager.Load(taskID); ok {
		// for test
		if t, ok = storedTask.(*pool.Task); ok {
			var cannotDeleteJson = map[string]interface{}{
				"code": -1,
				"msg":  fmt.Sprintf("cannot cancel task for copying from %s type to %s type", t.SrcType, t.DstType),
			}
			if (t.SrcType == drives.SrcTypeSync || t.DstType == drives.SrcTypeSync) ||
				(drives.IsCloudDrives(t.SrcType) && t.SrcType == t.DstType) {
				w.Header().Set("Content-Type", "application/json")
				return common.RenderJSON(w, r, cannotDeleteJson)
			}

			klog.Infof("~~~Debug log: before cancel Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
			klog.Infof("~~~Debug log: before cancel Task %s Progress: %d%%\n", t.ID, t.GetProgress())
			pool.CancelTask(taskID, false)
			klog.Infof("~~~Debug log: after cancel Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
			klog.Infof("~~~Debug log: after cancel Task %s Progress: %d%%\n", t.ID, t.GetProgress())
		}

		//pool.TaskManager.Delete(taskID)
	}

	// for test
	if storedTask, ok := pool.TaskManager.Load(taskID); ok {
		if t, ok = storedTask.(*pool.Task); ok {
			klog.Infof("~~~Debug log: after delete Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
			klog.Infof("~~~Debug log: after delete Task %s Progress: %d%%\n", t.ID, t.GetProgress())
		} else {
			klog.Infof("After delete, Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	return common.RenderJSON(w, r, map[string]interface{}{
		"code": 0,
		"msg":  "success",
	})
}
