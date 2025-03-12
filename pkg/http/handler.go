package http

import "net/http"

type ResourceGetHandlerInterface interface {
	Handle(w http.ResponseWriter, r *http.Request, stream, meta int, d *data) (int, error)
}

func getResourceHandler(srcType string) (ResourceGetHandlerInterface, error) {
	switch srcType {
	case "sync":
		return &resourceGetSyncHandler{}, nil
	case "google":
		return &resourceGetGoogleHandler{}, nil
	case "cloud", "awss3", "tencent", "dropbox":
		return &resourceGetCloudDriveHandler{}, nil
	default:
		return &resourceGetDriveCacheHandler{}, nil
	}
}
