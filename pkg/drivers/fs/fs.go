package fs

import (
	"files/pkg/common"
	"files/pkg/drivers/base"
	basefs "files/pkg/drivers/fs/base"
	"files/pkg/drivers/fs/cache"
	"files/pkg/drivers/fs/data"
	"files/pkg/drivers/fs/drive"
	externalinternal "files/pkg/drivers/fs/external_internal"
	"net/http"
)

func NewFsStorage(fsType string, w http.ResponseWriter, r *http.Request, d *common.Data) base.Execute {
	var self = &basefs.FSStorage{
		Base: base.NewBaseStorage(w, r, d),
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
	case "external":
		return &externalinternal.InternalStorage{
			Base: self,
		}
	default:
		panic("driver not found")
	}
}
