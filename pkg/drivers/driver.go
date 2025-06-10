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
	case "drive", "data", "internal", "hdd", "smb", "usb":
		return &posix.PosixStorage{
			Handler: handlerParam,
		}
	case "cache":
		return &posix.CacheStorage{
			Posix: &posix.PosixStorage{
				Handler: handlerParam,
			},
		}
	case "sync":
		return syncs.NewSyncStorage(handlerParam)
	case "google", "awss3", "tencent", "dropbox":
		return clouds.NewCloudStorage(fileType, handlerParam)

	default:
		panic("driver not found")
	}
}
