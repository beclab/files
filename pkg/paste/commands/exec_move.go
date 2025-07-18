package commands

import (
	"context"
	"files/pkg/paste/exec"
	"files/pkg/utils"
)

func (c *command) Move() error {
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

	var args = []string{srcPath, dstPath}

	if _, err = exec.ExecCmd(context.Background(), mv, args); err != nil {
		return err
	}

	return nil
}
