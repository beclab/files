package clouds

import (
	"files/pkg/common"
	"files/pkg/drivers/base"
	basecloud "files/pkg/drivers/clouds/base"
	"files/pkg/drivers/clouds/dropbox"
	"files/pkg/drivers/clouds/google"
	"files/pkg/drivers/clouds/s3"
	"files/pkg/drivers/clouds/tencent"
	"net/http"
)

func NewCloudStorage(cloudType string, w http.ResponseWriter, r *http.Request, d *common.Data) base.Execute {
	var self = &basecloud.CloudStorage{
		Base:    base.NewBaseStorage(w, r, d),
		Service: basecloud.NewService(w, r, d),
	}

	switch cloudType {
	case "awss3":
		return &s3.S3Storage{
			Base: self,
		}
	case "tencent":
		return &tencent.TencentStorage{
			Base: self,
		}
	case "dropbox":
		return &dropbox.DropBoxStorage{
			Base: self,
		}
	case "google":
		return &google.GoogleStorage{
			Base:    self,
			Service: google.NewService(w, r, d),
		}
	default:
		panic("driver not found")
	}
}
