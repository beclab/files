package clouds

import (
	"context"
	"errors"
	"files/pkg/common"
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
	service        *service
	fileParam      *models.FileParam
	fileName       string
	fileSize       int64
	fileTargetPath string
}

func NewDownloader(ctx context.Context, service *service, fileParam *models.FileParam, fileName string, fileSize int64, fileTargetPath string) *Download {

	return &Download{
		ctx:            ctx,
		service:        service,
		fileParam:      fileParam,
		fileName:       fileName,
		fileSize:       fileSize,
		fileTargetPath: fileTargetPath,
	}
}

func (d *Download) download() error {
	var owner = d.fileParam.Owner
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

	klog.Infof("Cloud download, owner: %s, param: %s", owner, utils.ToJson(p))

	type asyncResult struct {
		jobId int
		err   error
	}

	ch := make(chan asyncResult, 1)
	defer close(ch)
	go func() {
		res, err := d.service.DownloadAsync(owner, p)
		ch <- asyncResult{jobId: res, err: err}
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

	var ticker = time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-ticker.C:
		}

		res, err := d.service.QueryJob(resp.jobId)
		if err != nil {
			klog.Errorf("Cloud download, query job error: %v", err)
			return err
		}

		klog.Infof("Cloud download task status, user: %s, file: %s, id: %d, status:%s", owner, d.fileName, resp.jobId, utils.ToJson(res))

		if res.Finished {
			return nil
		}

		if res.Error == "job not found" {
			return nil
		}

		if !res.Success && !res.Finished {
			continue
		}

		return errors.New(res.Error)
	}
}

func (d *Download) generateBufferFolder() (string, error) {
	var owner = d.fileParam.Owner
	var exists = false
	var bufferFolder = path.Join("/", "data", "buffer", owner)
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
