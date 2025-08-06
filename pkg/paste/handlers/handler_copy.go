package handlers

import (
	"files/pkg/utils"

	"k8s.io/klog/v2"
)

func (c *Handler) SyncCopy() error {
	return nil
}
func (c *Handler) CloudCopy() error {
	klog.Infof("CloudCopy - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	return c.cloudTransfer()
}
