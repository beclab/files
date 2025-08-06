package handlers

import (
	"files/pkg/utils"

	"k8s.io/klog/v2"
)

func (c *Handler) DownloadFromFiles() error {
	return nil
}

func (c *Handler) DownloadFromSync() error {
	return nil
}

func (c *Handler) DownloadFromCloud() error {
	klog.Infof("DownloadFromCloud - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	return c.cloudTransfer()

	// todo If the operation fails or the task is canceled, the target file needs to be deleted;
	// todo if it is a paste operation and the copy is successful, the source file needs to be deleted.
}
