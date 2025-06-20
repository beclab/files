package clouds

import (
	"files/pkg/drivers/base"
	basecloud "files/pkg/drivers/clouds/base"
	"files/pkg/drivers/clouds/dropbox"
	"files/pkg/drivers/clouds/google"
	"files/pkg/drivers/clouds/s3"
	"files/pkg/drivers/clouds/tencent"
)

func NewCloudStorage(fsType string, handlerParam *base.HandlerParam) base.Execute {
	var self = &basecloud.CloudStorage{
		Handler: handlerParam,
		Service: basecloud.NewService(handlerParam),
	}

	switch fsType {
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
			Service: google.NewService(handlerParam),
		}
	default:
		panic("driver not found")
	}
}
