package drivers

import (
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds"
	"files/pkg/drivers/fs"
	syncs "files/pkg/drivers/sync"
)

type DriverHandler struct{}

func (d *DriverHandler) NewFileHandler(fileType string, handlerParam *base.HandlerParam) base.Execute {
	switch fileType {
	case "drive", "data", "cache", "internal", "hdd", "smb", "usb":
		return fs.NewFsStorage(fileType, handlerParam)
	case "external": // todo
		return fs.NewFsStorage(fileType, handlerParam)
	case "sync":
		return syncs.NewSyncStorage(handlerParam)
	case "google", "awss3", "", "tencent", "dropbox":
		return clouds.NewCloudStorage(fileType, handlerParam)

	default:
		panic("driver not found")
	}
}
