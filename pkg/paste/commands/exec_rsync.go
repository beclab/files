package commands

import (
	"context"
	"files/pkg/paste/exec"
	"files/pkg/utils"

	"k8s.io/klog/v2"
)

func (c *command) Rsync() error {
	klog.Infof("Rsync - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	rsync, err := utils.GetCommand("rsync")
	if err != nil {
		return err
	}

	src, err := c.src.GetResourceUri()
	if err != nil {
		return err
	}

	dst, err := c.dst.GetResourceUri()
	if err != nil {
		return err
	}

	srcPath := src + c.src.Path
	dstPath := dst + c.dst.Path

	klog.Infof("Rsync - owner: %s, srcPath: %s, dstPath: %s", c.owner, srcPath, dstPath)

	var args = []string{
		"-av",
		"--bwlimit=8000", // from env
		"--info=progress2",
		srcPath,
		dstPath,
	}

	_, err = exec.ExecRsync(context.Background(), rsync, args)
	if err != nil {
		return err
	}

	return nil
}
