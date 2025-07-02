package clouds

import (
	"context"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"path"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type Download struct {
	ctx            context.Context
	owner          string
	service        base.CloudServiceInterface
	fileParam      *models.FileParam
	fileName       string
	fileSize       int64
	fileTargetPath string
}

func NewDownloader(ctx context.Context, owner string, service base.CloudServiceInterface, fileParam *models.FileParam, fileName string, fileSize int64, fileTargetPath string) *Download {

	return &Download{
		ctx:            ctx,
		owner:          owner,
		service:        service,
		fileParam:      fileParam,
		fileName:       fileName,
		fileSize:       fileSize,
		fileTargetPath: fileTargetPath,
	}
}

func (d *Download) download() error {
	if err := d.checkCtx(); err != nil {
		return err
	}

	_, err := d.checkFreeDiskSpace()
	if err != nil {
		return err
	}

	var p = &models.DownloadAsyncParam{
		Drive:         d.fileParam.FileType,
		Name:          d.fileParam.Extend,
		CloudFilePath: d.fileParam.Path,
		LocalFolder:   d.fileTargetPath,
		LocalFileName: d.fileName,
	}

	klog.Infof("Cloud download, owner: %s, param: %s", d.owner, utils.ToJson(p))

	type asyncResult struct {
		data []byte
		err  error
	}

	ch := make(chan asyncResult, 1)
	defer close(ch)
	go func() {
		res, err := d.service.DownloadAsync(p)
		ch <- asyncResult{data: res, err: err}
	}()

	var resp asyncResult
	select {
	case <-d.ctx.Done():
		return d.ctx.Err()
	case resp = <-ch:
	}

	if err := d.checkCtx(); err != nil {
		return err
	}

	if resp.err != nil {
		return resp.err
	}

	var task models.TaskResponse
	if err := json.Unmarshal(resp.data, &task); err != nil {
		return err
	}

	if !task.IsSuccess() {
		return errors.New(task.FailMessage())
	}

	var taskQueryParam = &models.QueryTaskParam{
		TaskIds: []string{task.Data.ID},
	}

	var ticker = time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-ticker.C:
		}

		res, err := d.service.QueryTask(taskQueryParam)
		if err != nil {
			return err
		}

		var taskResp *models.TaskQueryResponse
		if err := json.Unmarshal(res, &taskResp); err != nil {
			return err
		}

		if len(taskResp.Data) == 0 {
			return fmt.Errorf("task not found")
		}

		klog.Infof("Cloud download task status, user: %s, file: %s, id: %s, status:%s", d.owner, d.fileName, task.Data.ID, taskResp.Status(task.Data.ID))

		if taskResp.Completed(task.Data.ID) {
			return nil
		}

		if taskResp.InProgress(task.Data.ID) {
			continue
		}

		return errors.New(taskResp.Status(task.Data.ID))
	}
}

func (d *Download) generateBufferFolder() (string, error) {
	var exists = false
	var bufferFolder = path.Join("/", "data", "buffer", d.owner)
	if fileutils.FilePathExists(bufferFolder) {
		exists = true
	}

	if !exists {
		if err := fileutils.MkdirAllWithChown(nil, bufferFolder, 0755); err != nil {
			return "", err
		}
	}

	if !strings.HasSuffix(bufferFolder, "/") {
		bufferFolder = bufferFolder + "/"
	}

	return bufferFolder, nil
}

func (d *Download) checkFreeDiskSpace() (bool, error) {
	spaceOk, needs, avails, reserved, err := common.CheckDiskSpace("/data", d.fileSize)
	if err != nil {
		return false, err
	}
	needsStr := common.FormatBytes(needs)
	availsStr := common.FormatBytes(avails)
	reservedStr := common.FormatBytes(reserved)
	if !spaceOk {
		errorMessage := fmt.Sprintf("Insufficient disk space available. This file still requires: %s, but only %s is available (with an additional %s reserved for the system).",
			needsStr, availsStr, reservedStr)
		return false, errors.New(errorMessage)
	}
	return true, nil
}

func (d *Download) checkCtx() error {
	select {
	case <-d.ctx.Done():
		return d.ctx.Err()
	default:
		return nil
	}
}
