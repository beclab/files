package handle_func

import (
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/hertz/biz/model/api/paste"
	"files/pkg/models"
	"files/pkg/tasks"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
	"strconv"
)

func NewPasteParam(c *app.RequestContext, req paste.PasteReq) (*models.PasteParam, error) {
	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, errors.New("user not found")
	}

	src, err := models.CreateFileParam(owner, req.Source)
	if err != nil {
		return nil, err
	}

	dst, err := models.CreateFileParam(owner, req.Destination)
	if err != nil {
		return nil, err
	}

	var pasteParam = &models.PasteParam{
		Owner:  owner,
		Action: req.Action,
		Src:    src,
		Dst:    dst,
	}

	return pasteParam, nil
}

func PasteHandle(_ context.Context, c *app.RequestContext, req interface{}, _ string) []byte {
	var pasteParam, err = NewPasteParam(c, req.(paste.PasteReq))
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return nil
	}

	handler := drivers.Adaptor.NewFileHandler(pasteParam.Src.FileType, &base.HandlerParam{})

	task, err := handler.Paste(pasteParam)
	if err != nil {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return nil
	}

	c.SetStatusCode(consts.StatusOK)
	var data = map[string]string{"task_id": task.Id()}
	return common.ToBytes(data)
}

func NewTaskParam(c *app.RequestContext, req interface{}) (*models.TaskParam, error) {
	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var taskParam = &models.TaskParam{
		Owner: owner,
	}
	switch req.(type) {
	case paste.GetTaskReq:
		reqGet := req.(paste.GetTaskReq)
		taskParam.TaskId = reqGet.TaskId
		if reqGet.Status != nil {
			taskParam.Status = *reqGet.Status
		}
		if reqGet.LogView != nil {
			taskParam.LogView = strconv.Itoa(int(*reqGet.LogView))
		}
	case paste.DeleteTaskReq:
		reqDelete := req.(paste.DeleteTaskReq)
		taskParam.TaskId = reqDelete.TaskId
		taskParam.Delete = strconv.Itoa(int(reqDelete.Delete))
		if reqDelete.All != nil {
			taskParam.All = strconv.Itoa(int(*reqDelete.All))
		}
	}

	return taskParam, nil
}

func TaskHandle(_ context.Context, c *app.RequestContext, req interface{}, _ string) []byte {
	var taskParam, _ = NewTaskParam(c, req)

	if string(c.Method()) == consts.MethodDelete {
		return taskCancel(taskParam.Owner, taskParam.TaskId, taskParam.Delete, taskParam.All)
	} else {
		return taskQuery(taskParam.Owner, taskParam.TaskId, taskParam.Status)
	}
}

func taskCancel(owner string, taskId string, deleted string, all string) []byte {
	_ = deleted
	tasks.TaskManager.CancelTask(owner, taskId, all)

	var data = map[string]interface{}{
		"code": 0,
		"msg":  "success",
	}
	return common.ToBytes(data)
}

func taskQuery(owner string, taskId string, status string) []byte {
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
	return common.ToBytes(data)
}
