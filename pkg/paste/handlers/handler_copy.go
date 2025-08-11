package handlers

import (
	"errors"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/models"
	"files/pkg/utils"
	"k8s.io/klog/v2"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

func (c *Handler) CloudCopy() error {
	klog.Infof("CloudCopy - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	return c.cloudTransfer()
}

func (c *Handler) SyncCopy() error {
	klog.Infof("~~~Copy Debug log: Sync  copy begins!")
	header := make(http.Header)
	header.Add("X-Bfl-User", c.owner)

	totalSize, err := c.GetFromSyncFileCount(header, "size") // file and dir can both use this
	if err != nil {
		klog.Errorf("DownloadFromSync - GetFromSyncFileCount - %v", err)
		return err
	}
	klog.Infof("~~~Copy Debug log: DownloadFromSync - GetFromSyncFileCount - totalSize: %d", totalSize)
	if totalSize == 0 {
		return errors.New("DownloadFromSync - GetFromSyncFileCount - empty total size")
	}
	c.UpdateTotalSize(totalSize)

	err = c.DoSyncCopy(header, nil, nil)
	if err != nil {
		klog.Errorf("DownloadFromSync - DoSyncCopy - %v", err)
		return err
	}
	return nil
}

func MapProgressByTime(left, right int, size, speed int64, usedTime int) int {
	transferredBytes := int64(usedTime) * speed

	var progressPercentage int64
	if size > 0 {
		progress := transferredBytes * 10000 / size
		progressPercentage = progress / 100 // Keep all calculations in int64
	} else {
		progressPercentage = 0
	}

	if progressPercentage < 0 {
		progressPercentage = 0
	} else if progressPercentage > 100 {
		progressPercentage = 100
	}

	// Convert progressPercentage to int for the final mapping
	mappedProgress := left + (right-left)*int(progressPercentage)/100

	if mappedProgress < left {
		mappedProgress = left
	} else if mappedProgress > right {
		mappedProgress = right
	}

	return mappedProgress
}

func (c *Handler) SimulateProgress(left, right int, speed int64) {
	startTime := time.Now()
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Simulate progress update
			usedTime := int(time.Now().Sub(startTime).Seconds())
			status, _, transferred, size := c.GetProgress()
			progress := MapProgressByTime(left, right, size, speed, usedTime)

			if status == "running" {
				c.UpdateProgress(progress, transferred)
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (c *Handler) DoSyncCopy(header http.Header, src, dst *models.FileParam) error {
	select {
	case <-c.ctx.Done():
		return nil
	default:
	}

	if src == nil {
		src = c.src
	}
	if dst == nil {
		dst = c.dst
	}

	go c.SimulateProgress(0, 99, 50000000)

	var err error
	srcParentDir := filepath.Dir(strings.TrimSuffix(src.Path, "/"))
	srcDirents := []string{filepath.Base(strings.TrimSuffix(src.Path, "/"))}
	dstParentDir := filepath.Dir(strings.TrimSuffix(dst.Path, "/"))
	if c.action == "copy" {
		_, err = seahub.HandleBatchCopy(header, src.Extend, srcParentDir, srcDirents, dst.Extend, dstParentDir)
		if err != nil {
			return err
		}
	} else {
		_, err = seahub.HandleBatchMove(header, src.Extend, srcParentDir, srcDirents, dst.Extend, dstParentDir)
		if err != nil {
			return err
		}
	}
	_, _, _, size := c.GetProgress()
	c.UpdateProgress(100, size)
	return err
}
