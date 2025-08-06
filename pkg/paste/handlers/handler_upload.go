package handlers

import (
	"files/pkg/utils"

	"k8s.io/klog/v2"
)

func (c *Handler) UploadToSync() error {
	return nil
}

// todo check file same name before
func (c *Handler) UploadToCloud() error {
	klog.Infof("UploadToCloud - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	return c.cloudTransfer()

}
