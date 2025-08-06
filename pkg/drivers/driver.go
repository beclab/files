package drivers

import (
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds"
	"files/pkg/drivers/posix/cache"
	"files/pkg/drivers/posix/external"
	"files/pkg/drivers/posix/posix"
	"files/pkg/drivers/sync"

	"k8s.io/klog/v2"
)

var Adaptor *driverHandler

type driverHandler struct{}

func NewDriverHandler() {
	Adaptor = &driverHandler{}
}

func (d *driverHandler) NewFileHandler(fileType string, handlerParam *base.HandlerParam) base.Execute {
	switch fileType {

	case "drive":
		return posix.NewPosixStorage(handlerParam)

	case "cache":
		return cache.NewCacheStorage(handlerParam)

	case "external", "internal", "hdd", "smb", "usb":
		return external.NewExternalStorage(handlerParam)

	case "sync":
		return sync.NewSyncStorage(handlerParam)

	case "google", "awss3", "tencent", "dropbox":
		return clouds.NewCloudStorage(handlerParam)

	default:
		klog.Errorf("driver not found, fileType: %s", fileType)
		return nil

	}
}
