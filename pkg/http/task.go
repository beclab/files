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

//func resourceTaskGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
//	taskID := r.URL.Query().Get("task_id")
//	var t *pool.Task
//	if storedTask, ok := pool.TaskManager.Load(taskID); ok {
//		if t, ok = storedTask.(*pool.Task); ok {
//			klog.Infof("Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
//			klog.Infof("Task %s Progress: %d%%\n", t.ID, t.GetProgress())
//		}
//	}
//
//	w.Header().Set("Content-Type", "application/json")
//	return common.RenderJSON(w, r, map[string]interface{}{
//		"code": 0,
//		"msg":  "success",
//		"task": pool.FormattedTask{Task: *t},
//	})
//}

func resourceTaskGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	statusParam := r.URL.Query().Get("status") // 获取筛选的状态参数
	var statuses []string

	// 解析多状态参数（支持逗号分隔）
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
		// 列出所有任务的 task_id 和 status
		var tasks []struct {
			TaskID string `json:"task_id"`
			Action string `json:"action"`
			Status string `json:"status"`
		}

		pool.TaskManager.Range(func(key, value interface{}) bool {
			if t, ok := value.(*pool.Task); ok {
				// 状态匹配逻辑修改点
				if len(statuses) == 0 { // 无状态筛选
					tasks = append(tasks, struct {
						TaskID string `json:"task_id"`
						Action string `json:"action"`
						Status string `json:"status"`
					}{
						TaskID: t.ID,
						Action: t.Action,
						Status: t.Status,
					})
				} else { // 多状态筛选
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
							break // 匹配到任意一个状态即添加
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
		// 处理单个任务（保持不变）
		logViewStr := r.URL.Query().Get("log_view")
		logView := false
		if logViewStr == "1" {
			logView = true
		}
		if storedTask, ok := pool.TaskManager.Load(taskID); ok {
			if t, ok := storedTask.(*pool.Task); ok {
				klog.Infof("Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
				klog.Infof("Task %s Progress: %d%%\n", t.ID, t.GetProgress())

				ft := pool.FormattedTask{Task: *t}
				ft.WithLogControl(logView)
				ftByte, _ := ft.MarshalJSON()
				var ftJson interface{}
				err := json.Unmarshal(ftByte, &ftJson)
				if err != nil {
					klog.Errorf("Failed to unmarshal json: %v", err)
					ftJson = ftByte
				}
				return common.RenderJSON(w, r, map[string]interface{}{
					"code": 0,
					"msg":  "success",
					"task": ftJson,
				})
			}
		}

		// 如果没有找到任务，返回错误（保持不变）
		return common.RenderJSON(w, r, map[string]interface{}{
			"code": -1,
			"msg":  "task not found",
		})
	}
}

func resourceTaskDeleteHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	taskID := r.URL.Query().Get("task_id")
	klog.Infof("~~~Debug log: cancel Task %s\n", taskID)
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
				//if (t.SrcType == drives.SrcTypeSync || t.DstType == drives.SrcTypeSync) ||
				//	(drives.IsCloudDrives(t.SrcType) && t.SrcType == t.DstType) {
				if !t.Cancellable {
					w.Header().Set("Content-Type", "application/json")
					return common.RenderJSON(w, r, cannotDeleteJson)
				}
			}

			klog.Infof("~~~Debug log: before cancel Task %s Infos: %s\n", t.ID, pool.FormattedTask{Task: *t})
			klog.Infof("~~~Debug log: before cancel Task %s Progress: %d%%\n", t.ID, t.GetProgress())

			if t.RelationTaskID != "" {
				client := &http.Client{
					Timeout: 10 * time.Second,
				}

				taskUrl := fmt.Sprintf("http://127.0.0.1:80/api/cache/%s/task?task_id=%s&&delete=%s", t.RelationNode, t.RelationTaskID, delStr)

				// 发送HTTP请求
				req, err := http.NewRequestWithContext(t.Ctx, "DELETE", taskUrl, nil)
				if err != nil {
					//t.ErrChan <- fmt.Errorf("failed to create request: %v", err)
					drives.TaskLog(t, "error", fmt.Errorf("failed to create request: %v", err))
					return common.ErrToStatus(err), err
				}
				req.Header = r.Header

				resp, err := client.Do(req)
				if err != nil {
					//t.ErrChan <- fmt.Errorf("failed to query task status: %v", err)
					drives.TaskLog(t, "error", fmt.Errorf("failed to query task status: %v", err))
					return common.ErrToStatus(err), err
				}
				defer resp.Body.Close() // 确保响应体在函数返回时关闭

				if resp.StatusCode != http.StatusOK {
					//t.ErrChan <- fmt.Errorf("failed to query task status: %s", resp.Status)
					drives.TaskLog(t, "error", fmt.Errorf("failed to query task status: %s", resp.Status))
					return common.ErrToStatus(err), err
				}

				// 读取响应体
				body, err := io.ReadAll(drives.SuitableResponseReader(resp))
				if err != nil {
					//t.ErrChan <- fmt.Errorf("failed to read response body: %v", err)
					drives.TaskLog(t, "error", fmt.Errorf("failed to read response body: %v", err))
					return common.ErrToStatus(err), err
				}

				var apiResponse map[string]interface{}
				if err = json.Unmarshal(body, &apiResponse); err != nil {
					//klog.Infof("~~~Debug Log: failed to unmarshal response body: %v", body)
					//t.ErrChan <- fmt.Errorf("failed to unmarshal response: %v", err)
					drives.TaskLog(t, "error", fmt.Errorf("failed to unmarshal response: %v", err))
					return common.ErrToStatus(err), err
				}

				time.Sleep(1 * time.Second)
			}

			pool.CancelTask(taskID, del)
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
