package fs

import (
	"files/pkg/drivers/base"
	basefs "files/pkg/drivers/fs/base"
	"files/pkg/drivers/fs/cache"
	"files/pkg/drivers/fs/data"
	"files/pkg/drivers/fs/drive"
	"files/pkg/drivers/fs/external/hdd"
	"files/pkg/drivers/fs/external/inner"
	"files/pkg/drivers/fs/external/smb"
	"files/pkg/drivers/fs/external/usb"
)

func NewFsStorage(fsType string, handlerParam *base.HandlerParam) base.Execute {
	var self = &basefs.FSStorage{
		Handler: handlerParam,
	}

	switch fsType {
	case "drive":
		return &drive.DriveStorage{
			Base: self,
		}
	case "cache":
		return &cache.CacheStorage{
			Base: self,
		}
	case "data":
		return &data.DataStorage{
			Base: self,
		}
	case "internal", "external":
		return &inner.InternalStorage{
			Base: self,
		}
	case "hdd":
		return &hdd.HddStorage{
			Base: self,
		}
	case "smb":
		return &smb.SmbStorage{
			Base: self,
		}
	case "usb":
		return &usb.UsbStorage{
			Base: self,
		}
	default:
		panic("driver not found")
	}
}
