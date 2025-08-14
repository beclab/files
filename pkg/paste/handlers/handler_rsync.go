package handlers

import (
	"context"
	"files/pkg/common"
	"files/pkg/files"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

func (c *Handler) Rsync() error {
	var src = c.src
	var dst = c.dst
	var srcUri, err = src.GetResourceUri()
	if err != nil {
		klog.Errorf("command - Rsync, get src uri error: %v", err)
		return err
	}

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		klog.Errorf("command - Rsync, get dst uri error: %v", err)
		return err
	}

	srcPath := srcUri + src.Path

	if dst.FileType == common.External {
		if err = c.checkDstPathPermission(); err != nil {
			return err
		}
	}

	pathMeta, err := files.GetFileInfo(srcPath)
	if err != nil {
		klog.Errorf("command - Rsync, get src meta info error: %v", err)
		return err
	}

	c.UpdateTotalSize(pathMeta.Size)

	klog.Infof("command - Rsync, srcPath: %s, srcMeta: %s", srcPath, common.ToJson(pathMeta))

	dstFree, dstUsedPercent, err := files.GetSpaceSize(dstUri)
	if err != nil {
		klog.Errorf("command - Rsync, get dst space size error: %v", err)
		return err
	}

	if dstUsedPercent > common.FreeLimit {
		return fmt.Errorf("target disk usage has reached %.2f%%. Please clean up disk space first.", common.FreeLimit)
	}

	if pathMeta.Size > int64(dstFree) {
		return fmt.Errorf("not enough free space on target disk, required: %s, available: %s", common.FormatBytes(pathMeta.Size), common.FormatBytes(int64(dstFree)))
	}

	klog.Infof("command - Rsync, srcPath: %s, dstUri: %s, dstFree: %d, dstUsed: %.2f%%", srcPath, dstUri, dstFree, dstUsedPercent)

	generatedDstNewName, generatedDstNewPath, err := c.generateNewName(pathMeta)
	if err != nil {
		klog.Errorf("command - Rsync, generate dst name error: %v", err)
		return err
	}

	klog.Infof("command - Rsync, generated, name: %s, path: %s", generatedDstNewName, generatedDstNewPath)

	if generatedDstNewName != "" {
		c.dst.Path = generatedDstNewPath
	}

	if c.action == "move" {
		return c.move()
	}
	return c.rsync()
}

func (c *Handler) rsync() error {
	klog.Infof("Rsync - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, common.ToJson(c.src), common.ToJson(c.dst))

	rsync, err := common.GetCommand("rsync")
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
		// "--bwlimit=15000", // from env
		"--info=PROGRESS2",
		srcPath,
		dstPath,
	}

	_, err = common.ExecRsync(c.ctx, rsync, args, c.UpdateProgress)

	if err != nil {
		return err
	}

	return nil
}

func (c *Handler) move() error {
	klog.Infof("Move - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, common.ToJson(c.src), common.ToJson(c.dst))

	mv, err := common.GetCommand("mv")
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

	if _, err = common.ExecCommand(context.Background(), mv, args); err != nil {
		return err
	}

	return nil
}

func (c *Handler) generateNewName(srcFileInfo *files.PathMeta) (string, string, error) {
	var dstUri, _ = c.dst.GetResourceUri()
	var dstPath = dstUri + c.dst.Path
	var targetPath string
	var targetName string
	if !files.FilePathExists(dstPath) {
		return "", "", nil
	}

	var ext = srcFileInfo.Ext
	var isDir = srcFileInfo.IsDir
	if !isDir {
		targetPath = strings.TrimSuffix(dstPath, srcFileInfo.Name) //strings.ReplaceAll(dstPath, srcFileInfo.Name, "")
		targetName = strings.ReplaceAll(srcFileInfo.Name, srcFileInfo.Ext, "")
	} else {
		var tmp = strings.TrimSuffix(dstPath, "/")
		var pos = strings.LastIndex(tmp, "/")
		targetPath = tmp[:strings.LastIndex(tmp, "/")]
		targetName = tmp[pos:]
		targetName = strings.Trim(targetName, "/")
	}

	dupNames, err := common.CollectDupNames(targetPath, targetName, ext, isDir)
	if err != nil {
		return "", "", err
	}

	if dupNames == nil || len(dupNames) == 0 {
		return "", "", nil
	}

	newPrefixName := files.GenerateDupCommonName(dupNames, targetName)
	var newName string
	if isDir {
		newName = newPrefixName
	} else {
		newName = fmt.Sprintf("%s%s", newPrefixName, ext)
	}

	// new dst.Path
	var newDstPath string = files.UpdatePathName(c.dst.Path, newName, isDir)

	return newName, newDstPath, nil

}

func (c *Handler) checkDstPathPermission() error {
	var dst, _ = c.dst.GetResourceUri()
	var dstPath = dst + c.dst.Path
	var tmp = strings.TrimSuffix(dstPath, "/")
	var pos = strings.LastIndex(tmp, "/")
	dstPath = tmp[:pos] + "/"

	return files.WriteTempFile(dstPath)
}
