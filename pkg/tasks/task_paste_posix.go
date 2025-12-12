package tasks

import (
	"context"
	"files/pkg/common"
	"files/pkg/files"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

func (t *Task) Rsync() error {
	var user = t.param.Owner
	var action = t.param.Action
	var src = t.param.Src
	var dst = t.param.Dst
	var share = t.param.Share
	var srcShareType = t.param.SrcShareType
	var dstShareType = t.param.DstShareType
	var srcOwner = t.param.SrcOwner
	var dstOwner = t.param.DstOwner

	if share == 1 {
		src.Owner = srcOwner
		dst.Owner = dstOwner
	}

	klog.Infof("[Task] Id: %s, start, rsync, user: %s, action: %s, src: %s, dst: %s, shareType: %s,%s, shareOwner:%s,%s",
		t.id, user, action, common.ToJson(src), common.ToJson(dst), srcShareType, dstShareType, srcOwner, dstOwner)

	var srcUri, err = src.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get src uri error: %v", err)
	}

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get dst uri error: %v", err)
	}

	srcPath := srcUri + src.Path

	if dst.FileType == common.External {
		if err = t.checkDstPathPermission(); err != nil {
			return fmt.Errorf("check dst permission error: %v", err)
		}
	}

	pathMeta, err := files.GetFileInfo(srcPath)
	if err != nil {
		return fmt.Errorf("get src meta info error: %v", err)
	}

	t.updateTotalSize(pathMeta.Size)

	klog.Infof("[Task] Id: %s, srcPath: %s, dstUri: %s, srcMeta: %s", t.id, srcPath, dstUri, common.ToJson(pathMeta))

	// check free space
	dstFree, dstUsedPercent, err := files.GetSpaceSize(dstUri)
	if err != nil {
		return fmt.Errorf("get dst space size error: %v", err)
	}

	if pathMeta.Size > int64(dstFree) {
		return fmt.Errorf("not enough free space on target disk, required: %s, available: %s", common.FormatBytes(pathMeta.Size), common.FormatBytes(int64(dstFree)))
	}

	klog.Infof("[Task] Id: %s, dstFree: %d, dstUsed: %.2f%%", t.id, dstFree, dstUsedPercent)

	if !t.wasPaused {
		generatedDstNewName, generatedDstNewPath, err := t.generateNewName(pathMeta)
		if err != nil {
			return fmt.Errorf("generate dst name error: %v", err)
		}

		if generatedDstNewName != "" {
			t.param.Dst.Path = generatedDstNewPath
		}
	}

	klog.Infof("[Task] Id: %s, src: %s, dst: %s", t.id, common.ToJson(t.param.Src), common.ToJson(t.param.Dst))

	if t.param.Action == common.ActionMove {
		if err = t.move(); err != nil { // move
			klog.Errorf("[Task] Id: %s, move dst error: %v", t.id, err)

			return err
		}

		klog.Infof("[Task] Id: %s, move done!", t.id)
		t.details = append(t.details, "move done")

		return nil
	}

	err = t.rsync() // rsync
	if err != nil {
		klog.Errorf("[Task] Id: %s, copy dst error: %v", t.id, err)
		t.pausedParam = t.param.Dst
		return err
	}

	klog.Infof("[Task] Id: %s, copy done!", t.id)

	return nil
}

// ~ rsync
func (t *Task) rsync() error {
	rsync, err := common.GetCommand("rsync")
	if err != nil {
		return fmt.Errorf("get command rsync error: %v", err)
	}

	src, err := t.param.Src.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get src uri error: %v", err)
	}

	dst, err := t.param.Dst.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get dst uri error: %v", err)
	}

	srcPath := src + t.param.Src.Path
	dstPath := dst + t.param.Dst.Path

	klog.Infof("[Task] Id: %s, conditon rsync, srcPath: %s, dstPath: %s", t.id, srcPath, dstPath)

	var args = []string{
		"-av",
		// "--bwlimit=15000", // from env
		"--info=PROGRESS2",
		srcPath,
		dstPath,
	}

	_, err = common.ExecRsync(t.ctx, rsync, args, t.updateProgressRsync)
	if err != nil {
		klog.Errorf("exec rsync error: %v", err)
		return err
	}

	return nil
}

// ~ move
func (t *Task) move() error {
	mv, err := common.GetCommand("mv")
	if err != nil {
		return fmt.Errorf("get command mv error: %v", err)
	}

	src, err := t.param.Src.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get src uri error: %v", err)
	}

	dst, err := t.param.Dst.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get dst uri error: %v", err)
	}

	srcPath := src + t.param.Src.Path
	dstPath := dst + t.param.Dst.Path

	klog.Infof("[Task] Id: %s, conditon move, srcPath: %s, dstPath: %s", t.id, srcPath, dstPath)
	t.details = append(t.details, fmt.Sprintf("move %s -> %s", srcPath, dstPath))

	var args = []string{srcPath, dstPath}

	if _, err = common.ExecCommand(context.Background(), mv, args); err != nil {
		return fmt.Errorf("exec mv error: %v", err)
	}

	return nil
}

func (t *Task) generateNewName(srcFileInfo *files.PathMeta) (string, string, error) {
	var dstUri, _ = t.param.Dst.GetResourceUri()
	var dstPath = dstUri + t.param.Dst.Path
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

	dupNames, err := files.CollectDupNames(targetPath, targetName, ext, isDir)
	if err != nil {
		return "", "", err
	}

	if dupNames == nil || len(dupNames) == 0 {
		return "", "", nil
	}

	newPrefixName := files.GenerateDupName(dupNames, targetName, t.isFile)
	var newName string
	if isDir {
		newName = newPrefixName
	} else {
		newName = fmt.Sprintf("%s%s", newPrefixName, ext)
	}

	// new dst.Path
	var newDstPath string = files.UpdatePathName(t.param.Dst.Path, newName, isDir)

	return newName, newDstPath, nil

}

func (t *Task) checkDstPathPermission() error {
	var dst, _ = t.param.Dst.GetResourceUri()
	var dstPath = dst + t.param.Dst.Path
	var tmp = strings.TrimSuffix(dstPath, "/")
	var pos = strings.LastIndex(tmp, "/")
	dstPath = tmp[:pos] + "/"

	return files.WriteTempFile(dstPath)
}
