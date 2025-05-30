package http

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/pool"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"strings"
	"time"
)

func resourceTaskGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	statusParam := r.URL.Query().Get("status")
	var statuses []string

	if statusParam != "" {
		for _, s := range strings.Split(statusParam, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				statuses = append(statuses, s)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		var tasks []struct {
			TaskID string `json:"task_id"`
			Action string `json:"action"`
			Status string `json:"status"`
		}

		pool.TaskManager.Range(func(key, value interface{}) bool {
			if t, ok := value.(*pool.Task); ok {
				if len(statuses) == 0 {
					tasks = append(tasks, struct {
						TaskID string `json:"task_id"`
						Action string `json:"action"`
						Status string `json:"status"`
					}{
						TaskID: t.ID,
						Action: t.Action,
						Status: t.Status,
					})
				} else {
					for _, s := range statuses {
						if s == t.Status {
							tasks = append(tasks, struct {
								TaskID string `json:"task_id"`
								Action string `json:"action"`
								Status string `json:"status"`
							}{
								TaskID: t.ID,
								Action: t.Action,
								Status: t.Status,
							})
							break
						}
					}
				}
			}
			return true
		})

		return common.RenderJSON(w, r, map[string]interface{}{
			"code":  0,
			"msg":   "success",
			"tasks": tasks,
		})
	} else {
		logViewStr := r.URL.Query().Get("log_view")
		logView := false
		if logViewStr == "1" {
			logView = true
		}
		if storedTask, ok := pool.TaskManager.Load(taskID); ok {
			if t, ok := storedTask.(*pool.Task); ok {
				klog.Infof("Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
				klog.Infof("Task %s Progress: %d%%\n", t.ID, t.GetProgress())

				return common.RenderJSON(w, r, map[string]interface{}{
					"code": 0,
					"msg":  "success",
					"task": pool.SerializeTask(t, logView),
				})
			}
		}

		return common.RenderJSON(w, r, map[string]interface{}{
			"code": -1,
			"msg":  "task not found",
		})
	}
}

func resourceTaskDeleteHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	taskID := r.URL.Query().Get("task_id")
	delStr := r.URL.Query().Get("delete")
	del := false
	if delStr == "1" {
		del = true
	}
	var t *pool.Task

	if storedTask, ok := pool.TaskManager.Load(taskID); ok {
		// for test
		if t, ok = storedTask.(*pool.Task); ok {
			if !del {
				var cannotDeleteJson = map[string]interface{}{
					"code": -1,
					"msg":  fmt.Sprintf("cannot cancel task for copying from %s type to %s type", t.SrcType, t.DstType),
				}

				if !t.Cancellable {
					w.Header().Set("Content-Type", "application/json")
					return common.RenderJSON(w, r, cannotDeleteJson)
				}
			}

			if t.RelationTaskID != "" {
				client := &http.Client{
					Timeout: 10 * time.Second,
				}

				taskUrl := fmt.Sprintf("http://127.0.0.1:80/api/cache/%s/task?task_id=%s&&delete=%s", t.RelationNode, t.RelationTaskID, delStr)

				req, err := http.NewRequestWithContext(t.Ctx, "DELETE", taskUrl, nil)
				if err != nil {
					drives.TaskLog(t, "error", fmt.Errorf("failed to create request: %v", err))
					return common.ErrToStatus(err), err
				}
				req.Header = r.Header

				resp, err := client.Do(req)
				if err != nil {
					drives.TaskLog(t, "error", fmt.Errorf("failed to query task status: %v", err))
					return common.ErrToStatus(err), err
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					drives.TaskLog(t, "error", fmt.Errorf("failed to query task status: %s", resp.Status))
					return common.ErrToStatus(err), err
				}

				body, err := io.ReadAll(drives.SuitableResponseReader(resp))
				if err != nil {
					drives.TaskLog(t, "error", fmt.Errorf("failed to read response body: %v", err))
					return common.ErrToStatus(err), err
				}

				var apiResponse map[string]interface{}
				if err = json.Unmarshal(body, &apiResponse); err != nil {
					drives.TaskLog(t, "error", fmt.Errorf("failed to unmarshal response: %v", err))
					return common.ErrToStatus(err), err
				}

				time.Sleep(1 * time.Second)
			}

			pool.CancelTask(taskID, del)
		}
	}

	// for test
	if storedTask, ok := pool.TaskManager.Load(taskID); ok {
		if t, ok = storedTask.(*pool.Task); ok {
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
