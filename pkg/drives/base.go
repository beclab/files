package drives

import (
	"context"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

var (
	MountedData   []files.DiskInfo = nil
	mu            sync.Mutex
	MountedTicker = time.NewTicker(5 * time.Minute)
)

type handleFunc func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
type PathProcessor func(*gorm.DB, string, string, time.Time) (int, error)
type RecordsStatusProcessor func(db *gorm.DB, processedPaths map[string]bool, srcTypes []string, status int) error

type ResourceService interface {
	// resource handlers
	PutHandler(fileParam *models.FileParam) handleFunc
}

var (
	BaseService = &BaseResourceService{}
	SyncService = &SyncResourceService{}
)

const (
	SrcTypeDrive    = "drive"
	SrcTypeData     = "data"
	SrcTypeExternal = "external"
	SrcTypeCache    = "cache"
	SrcTypeSync     = "sync"
	SrcTypeGoogle   = "google"
	SrcTypeCloud    = "cloud"
	SrcTypeAWSS3    = "awss3"
	SrcTypeTencent  = "tencent"
	SrcTypeDropbox  = "dropbox"
	SrcTypeInternal = "internal"
	SrcTypeUsb      = "usb"
	SrcTypeSmb      = "smb"
	SrcTypeHdd      = "hdd"
)

func GetResourceService(srcType string) (ResourceService, error) {
	switch srcType {
	case SrcTypeSync:
		return SyncService, nil
	default:
		return BaseService, nil
	}
}

// TODO：protected
func GetMountedData(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	var err error = nil
	if files.TerminusdHost != "" {
		// for 1.12: path-incluster URL exists, won't err in normal condition
		// for 1.11: path-incluster URL may not exist, if err, use usb-incluster and hdd-incluster for system functional
		url := "http://" + files.TerminusdHost + "/system/mounted-path-incluster"

		headers := make(http.Header)
		headers.Set("Content-Type", "application/json")
		headers.Set("X-Signature", "temp_signature")

		MountedData, err = files.FetchDiskInfo(url, headers)
		if err != nil {
			klog.Infof("Failed to fetch data from %s: %v", url, err)
			usbUrl := "http://" + files.TerminusdHost + "/system/mounted-usb-incluster"

			usbHeaders := headers.Clone()
			usbHeaders.Set("Content-Type", "application/json")
			usbHeaders.Set("X-Signature", "temp_signature")

			usbData, err := files.FetchDiskInfo(usbUrl, usbHeaders)
			if err != nil {
				klog.Infof("Failed to fetch data from %s: %v", usbUrl, err)
			}

			klog.Infoln("USB Data:", usbData)

			hddUrl := "http://" + files.TerminusdHost + "/system/mounted-hdd-incluster"

			hddHeaders := headers.Clone()
			hddHeaders.Set("Content-Type", "application/json")
			hddHeaders.Set("X-Signature", "temp_signature")

			hddData, err := files.FetchDiskInfo(hddUrl, hddHeaders)
			if err != nil {
				klog.Infof("Failed to fetch data from %s: %v", hddUrl, err)
			}

			klog.Infoln("HDD Data:", hddData)

			for _, item := range usbData {
				item.Type = "usb"
				MountedData = append(MountedData, item)
			}

			for _, item := range hddData {
				item.Type = "hdd"
				MountedData = append(MountedData, item)
			}
		}
		klog.Infoln("Mounted Data:", MountedData)
	}
	MountedTicker.Reset(5 * time.Minute)
	return
}

type BaseResourceService struct{}

// TODO：protected
func (rs *BaseResourceService) PutHandler(fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		// Only allow PUT for files.
		if strings.HasSuffix(fileParam.Path, "/") {
			return http.StatusMethodNotAllowed, nil
		}

		uri, err := fileParam.GetResourceUri()
		if err != nil {
			return http.StatusBadRequest, err
		}
		urlPath := strings.TrimPrefix(uri+fileParam.Path, "/data")

		exists, err := afero.Exists(files.DefaultFs, urlPath)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !exists {
			return http.StatusNotFound, nil
		}

		info, err := files.WriteFile(files.DefaultFs, urlPath, r.Body)
		etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
		w.Header().Set("ETag", etag)

		return common.ErrToStatus(err), err
	}
}
