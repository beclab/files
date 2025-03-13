package drives

import (
	"bytes"
	"files/pkg/common"
	"files/pkg/errors"
	"files/pkg/files"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"time"
)

type ResourceService interface {
	GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	//PutHandle(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	//PostHandle(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	//PatchHandle(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	//DeleteHandle(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
}

var (
	BaseService        = &BaseResourceService{}
	DriveService       = &DriveResourceService{}
	CacheService       = &CacheResourceService{}
	GoogleDriveService = &GoogleDriveResourceService{}
	SyncService        = &SyncResourceService{}
	CloudDriveService  = &CloudDriveResourceService{}
)

const (
	SrcTypeDrive   = "drive"
	SrcTypeCache   = "cache"
	SrcTypeSync    = "sync"
	SrcTypeGoogle  = "google"
	SrcTypeCloud   = "cloud"
	SrcTypeAWSS3   = "awss3"
	SrcTypeTencent = "tencent"
	SrcTypeDropbox = "dropbox"
)

func GetResourceService(srcType string) (ResourceService, error) {
	switch srcType {
	case SrcTypeDrive:
		return DriveService, nil
	case SrcTypeCache:
		return CacheService, nil
	case SrcTypeSync:
		return SyncService, nil
	case SrcTypeGoogle:
		return GoogleDriveService, nil
	case SrcTypeCloud, SrcTypeAWSS3, SrcTypeTencent, SrcTypeDropbox:
		return CloudDriveService, nil
	default:
		return BaseService, nil
	}
}

type BaseResourceService struct{}

func (rs *BaseResourceService) GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	xBflUser := r.Header.Get("X-Bfl-User")
	klog.Infoln("X-Bfl-User: ", xBflUser)

	streamStr := r.URL.Query().Get("stream")
	stream := 0
	var err error = nil
	if streamStr != "" {
		stream, err = strconv.Atoi(streamStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	var mountedData []files.DiskInfo = nil
	if files.TerminusdHost != "" {
		// for 1.12: path-incluster URL exists, won't err in normal condition
		// for 1.11: path-incluster URL may not exist, if err, use usb-incluster and hdd-incluster for system functional
		url := "http://" + files.TerminusdHost + "/system/mounted-path-incluster"

		headers := r.Header.Clone()
		headers.Set("Content-Type", "application/json")
		headers.Set("X-Signature", "temp_signature")

		mountedData, err = files.FetchDiskInfo(url, headers)
		if err != nil {
			klog.Infof("Failed to fetch data from %s: %v", url, err)
			usbUrl := "http://" + files.TerminusdHost + "/system/mounted-usb-incluster"

			usbHeaders := r.Header.Clone()
			usbHeaders.Set("Content-Type", "application/json")
			usbHeaders.Set("X-Signature", "temp_signature")

			usbData, err := files.FetchDiskInfo(usbUrl, usbHeaders)
			if err != nil {
				klog.Infof("Failed to fetch data from %s: %v", usbUrl, err)
			}

			klog.Infoln("USB Data:", usbData)

			hddUrl := "http://" + files.TerminusdHost + "/system/mounted-hdd-incluster"

			hddHeaders := r.Header.Clone()
			hddHeaders.Set("Content-Type", "application/json")
			hddHeaders.Set("X-Signature", "temp_signature")

			hddData, err := files.FetchDiskInfo(hddUrl, hddHeaders)
			if err != nil {
				klog.Infof("Failed to fetch data from %s: %v", hddUrl, err)
			}

			klog.Infoln("HDD Data:", hddData)

			for _, item := range usbData {
				item.Type = "usb"
				mountedData = append(mountedData, item)
			}

			for _, item := range hddData {
				item.Type = "hdd"
				mountedData = append(mountedData, item)
			}
		}
		klog.Infoln("Mounted Data:", mountedData)
	}

	var file *files.FileInfo
	if mountedData != nil {
		file, err = files.NewFileInfoWithDiskInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.Server.TypeDetectionByHeader,
			Content:    true,
		}, mountedData)
	} else {
		file, err = files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.Server.TypeDetectionByHeader,
			Content:    true,
		})
	}
	if err != nil {
		if common.ErrToStatus(err) == http.StatusNotFound && r.URL.Path == "/External/" {
			listing := &files.Listing{
				Items:         []*files.FileInfo{},
				NumDirs:       0,
				NumFiles:      0,
				NumTotalFiles: 0,
				Size:          0,
				FileSize:      0,
			}
			file = &files.FileInfo{
				Path:         "/External/",
				Name:         "External",
				Size:         0,
				FileSize:     0,
				Extension:    "",
				ModTime:      time.Now(),
				Mode:         os.FileMode(2147484141),
				IsDir:        true,
				IsSymlink:    false,
				Type:         "",
				ExternalType: "others",
				Subtitles:    []string{},
				Checksums:    make(map[string]string),
				Listing:      listing,
				Fs:           nil,
			}

			return common.RenderJSON(w, r, file)
		}
		return common.ErrToStatus(err), err
	}

	if file.IsDir {
		if files.CheckPath(file.Path, files.ExternalPrefix, "/") {
			file.ExternalType = files.GetExternalType(file.Path, mountedData)
		}
		file.Listing.Sorting = files.DefaultSorting
		file.Listing.ApplySort()
		if stream == 1 {
			streamListingItems(w, r, file.Listing, d, mountedData)
			return 0, nil
		} else {
			return common.RenderJSON(w, r, file)
		}
	}

	if checksum := r.URL.Query().Get("checksum"); checksum != "" {
		err := file.Checksum(checksum)
		if err == errors.ErrInvalidOption {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}

		// do not waste bandwidth if we just want the checksum
		file.Content = ""
	}

	if file.Type == "video" {
		osSystemServer := "system-server.user-system-" + xBflUser

		httpposturl := fmt.Sprintf("http://%s/legacy/v1alpha1/api.intent/v1/server/intent/send", osSystemServer)

		klog.Infoln("HTTP JSON POST URL:", httpposturl)

		var jsonData = []byte(`{
			"action": "view",
			"category": "video",
			"data": {
				"name": "` + file.Name + `",
				"path": "` + file.Path + `",
				"extention": "` + file.Extension + `"
			}
		}`)
		request, error := http.NewRequest("POST", httpposturl, bytes.NewBuffer(jsonData))
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")

		client := &http.Client{}
		response, error := client.Do(request)
		if error != nil {
			panic(error)
		}
		defer response.Body.Close()

		klog.Infoln("response Status:", response.Status)
		klog.Infoln("response Headers:", response.Header)
		body, _ := ioutil.ReadAll(response.Body)
		klog.Infoln("response Body:", string(body))
	}
	return common.RenderJSON(w, r, file)
}
