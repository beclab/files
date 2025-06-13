package drivers

import (
	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/drivers/clouds"
	"files/pkg/drivers/fs"
	syncs "files/pkg/drivers/sync"
	"net/http"
)

func NewDriver(srcType string, w http.ResponseWriter, r *http.Request, d *common.Data) base.Execute {

	switch srcType {
	case "drive", "data", "cache", "external":
		return fs.NewFsStorage(srcType, w, r, d)
	case "sync":
		return syncs.NewSyncStorage(w, r, d)
	case "awss3", "tencent", "dropbox", "google":
		return clouds.NewCloudStorage(srcType, w, r, d)
	default:
		panic("driver not found")
	}

}
