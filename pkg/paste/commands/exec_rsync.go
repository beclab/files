package commands

import (
	"context"
	"files/pkg/paste/exec"
	"files/pkg/utils"

	"k8s.io/klog/v2"
)

func (c *Command) Rsync() error {
	if c.action == "move" {
		return c.move()
	}
	return c.rsync()
}

func (c *Command) rsync() error {
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
		"--bwlimit=5500", // from env
		"--info=PROGRESS2",
		srcPath,
		dstPath,
	}

	_, err = exec.ExecRsync(c.ctx, rsync, args, c.Update)

	if err != nil {
		return err
	}

	return nil
}

func (c *Command) move() error {
	klog.Infof("Move - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	mv, err := utils.GetCommand("mv")
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

	klog.Infof("Move - owner: %s, srcPath: %s, dstPath: %s", c.owner, srcPath, dstPath)

	var args = []string{srcPath, dstPath}

	if _, err = exec.ExecCmd(context.Background(), mv, args); err != nil {
		return err
	}

	return nil
}
