package drivers

import (
	"context"
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds"
	"files/pkg/drivers/posix"
	"files/pkg/drivers/sync"

	"k8s.io/klog/v2"
)

var Adaptor *driverHandler

type driverHandler struct{}

func NewDriverHandler() {
	Adaptor = &driverHandler{}
}

func (d *driverHandler) NewFileHandler(ctx context.Context, fileType string, handlerParam *base.HandlerParam) base.Execute {
	switch fileType {

	case "drive", "external", "internal", "hdd", "smb", "usb", "cache":
		return posix.NewPosixStorage(ctx, handlerParam)

	case "sync":
		return sync.NewSyncStorage(ctx, handlerParam)

	case "google", "awss3", "tencent", "dropbox":
		return clouds.NewCloudStorage(ctx, handlerParam)

	default:
		klog.Errorf("driver not found, fileType: %s", fileType)
		return nil

	}
}
