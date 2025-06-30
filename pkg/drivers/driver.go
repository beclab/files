package drivers

import (
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds"
	"files/pkg/drivers/posix"
	syncs "files/pkg/drivers/sync"
)

type DriverHandler struct{}

func (d *DriverHandler) NewFileHandler(fileType string, handlerParam *base.HandlerParam) base.Execute {
	switch fileType {
	case "external":
		return &posix.ExternalStorage{
			Posix: &posix.PosixStorage{
				Handler: handlerParam,
			},
		}
	case "drive", "internal", "hdd", "smb", "usb", "cache":
		return &posix.PosixStorage{
			Handler: handlerParam,
		}
	case "sync":
		return &syncs.SyncStorage{
			Handler: handlerParam,
			Service: syncs.NewService(handlerParam),
		}
	case "google", "awss3", "tencent", "dropbox":
		return &clouds.CloudStorage{
			Handler: handlerParam,
			Service: clouds.NewService(handlerParam.Owner, handlerParam.ResponseWriter, handlerParam.Request),
		}
	default:
		panic("driver not found")
	}
}
