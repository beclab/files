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
	"time"
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

			if t.RelationTaskID != "" {
				client := &http.Client{
					Timeout: 10 * time.Second,
				}

				taskUrl := fmt.Sprintf("http://127.0.0.1:80/api/cache/%s/task?task_id=%s", t.RelationNode, t.RelationTaskID)

				// 发送HTTP请求
				req, err := http.NewRequestWithContext(t.Ctx, "DELETE", taskUrl, nil)
				if err != nil {
					t.ErrChan <- fmt.Errorf("failed to create request: %v", err)
					return common.ErrToStatus(err), err
				}
				req.Header = r.Header

				resp, err := client.Do(req)
				if err != nil {
					t.ErrChan <- fmt.Errorf("failed to query task status: %v", err)
					return common.ErrToStatus(err), err
				}
				defer resp.Body.Close() // 确保响应体在函数返回时关闭

				if resp.StatusCode != http.StatusOK {
					t.ErrChan <- fmt.Errorf("failed to query task status: %s", resp.Status)
					return common.ErrToStatus(err), err
				}

				// 读取响应体
				body, err := io.ReadAll(drives.SuitableResponseReader(resp))
				if err != nil {
					t.ErrChan <- fmt.Errorf("failed to read response body: %v", err)
					return common.ErrToStatus(err), err
				}

				var apiResponse map[string]interface{}
				if err = json.Unmarshal(body, &apiResponse); err != nil {
					klog.Infof("~~~Debug Log: failed to unmarshal response body: %v", body)
					t.ErrChan <- fmt.Errorf("failed to unmarshal response: %v", err)
					return common.ErrToStatus(err), err
				}
			}

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
