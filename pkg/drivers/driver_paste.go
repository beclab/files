package drivers

import (
	"files/pkg/constant"
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds"
	"files/pkg/drivers/posix"
	"files/pkg/drivers/posix/cache"
	"files/pkg/drivers/posix/external"
	"files/pkg/drivers/sync"
	"files/pkg/models"

	"k8s.io/klog/v2"
)

// Incoming entry point function
func (d *driverHandler) Paste(pasteParam *models.PasteParam, handlerParam *base.HandlerParam) error {
	var user = pasteParam.Owner
	var action = pasteParam.Action

	_ = action

	srcFileParam, err := models.CreateFileParam(user, pasteParam.Source)
	if err != nil {
		klog.Errorf("Paste create src file param failed, err: %v", err)
		return err
	}

	dstFileParam, err := models.CreateFileParam(user, pasteParam.Destination)
	if err != nil {
		klog.Errorf("Paste create dst file param failed, err: %v", err)
		return err
	}

	var p base.PasteExecute

	var src = srcFileParam.FileType
	if src == constant.Drive {
		p = &posix.PosixStorage{Handler: handlerParam}

	} else if src == constant.External {
		p = &external.ExternalStorage{Handler: handlerParam}

	} else if src == constant.Cache {
		p = &cache.CacheStorage{Handler: handlerParam}

	} else if src == constant.Sync {
		p = &sync.SyncStorage{Handler: handlerParam}

	} else if src == constant.Cloud {
		p = &clouds.CloudStorage{Handler: handlerParam}
	}

	var dst = dstFileParam.FileType
	if dst == constant.Drive {
		p.CopyToDrive(srcFileParam, dstFileParam)

	} else if dst == constant.External {
		p.CopyToExternal(srcFileParam, dstFileParam)

	} else if dst == constant.Cache {
		p.CopyToCache(srcFileParam, dstFileParam)

	} else if dst == constant.Sync {
		p.CopyToSync(srcFileParam, dstFileParam)

	} else if dst == constant.Cloud {
		p.CopyToCloud(srcFileParam, dstFileParam)

	}

	return nil

}
