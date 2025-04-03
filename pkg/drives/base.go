package drives

import (
	"bytes"
	"context"
	"files/pkg/common"
	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/preview"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
type PathProcessor func(*gorm.DB, string, string, time.Time) error

type ResourceService interface {
	// resource handlers
	GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	DeleteHandler(fileCache fileutils.FileCache) handleFunc
	PostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	PutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)
	PatchHandler(fileCache fileutils.FileCache) handleFunc

	// raw handler
	RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error)

	// preview handler
	PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc

	// paste funcs
	PasteSame(action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error
	PasteDirFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, fileMode os.FileMode, w http.ResponseWriter,
		r *http.Request, driveIdCache map[string]string) error
	PasteDirTo(fs afero.Fs, src, dst string, fileMode os.FileMode, w http.ResponseWriter, r *http.Request,
		d *common.Data, driveIdCache map[string]string) error
	PasteFileFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, mode os.FileMode, diskSize int64,
		w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error
	PasteFileTo(fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, w http.ResponseWriter, r *http.Request,
		d *common.Data, diskSize int64) error
	GetStat(fs afero.Fs, src string, w http.ResponseWriter, r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error)
	MoveDelete(fileCache fileutils.FileCache, src string, ctx context.Context, d *common.Data, w http.ResponseWriter, r *http.Request) error

	// path list funcs
	GeneratePathList(db *gorm.DB, pathProcessor PathProcessor) error
	parsePathToURI(path string) (string, string)
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
)

var ValidSrcTypes = map[string]bool{
	SrcTypeDrive:    true,
	SrcTypeData:     true,
	SrcTypeExternal: true,
	SrcTypeCache:    true,
	SrcTypeSync:     true,
	SrcTypeGoogle:   true,
	SrcTypeCloud:    true,
	SrcTypeAWSS3:    true,
	SrcTypeTencent:  true,
	SrcTypeDropbox:  true,
}

func GetResourceService(srcType string) (ResourceService, error) {
	switch srcType {
	case SrcTypeDrive, SrcTypeData, SrcTypeExternal:
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

func IsThridPartyDrives(dstType string) bool {
	switch dstType {
	case SrcTypeDrive, SrcTypeData, SrcTypeExternal, SrcTypeCache, SrcTypeSync:
		return false
	case SrcTypeGoogle, SrcTypeCloud, SrcTypeAWSS3, SrcTypeTencent, SrcTypeDropbox:
		return true
	default:
		return false
	}
}

func IsBaseDrives(dstType string) bool {
	switch dstType {
	case SrcTypeDrive, SrcTypeData, SrcTypeExternal, SrcTypeCache:
		return true
	default:
		return false
	}
}

func IsCloudDrives(dstType string) bool {
	switch dstType {
	case SrcTypeCloud, SrcTypeAWSS3, SrcTypeTencent, SrcTypeDropbox:
		return true
	default:
		return false
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

func (rs *BaseResourceService) DeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		if r.URL.Path == "/" {
			return http.StatusForbidden, nil
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     false,
			ReadHeader: d.Server.TypeDetectionByHeader,
		})
		if err != nil {
			return common.ErrToStatus(err), err
		}

		// delete thumbnails
		err = preview.DelThumbs(r.Context(), fileCache, file)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		err = files.DefaultFs.RemoveAll(r.URL.Path)

		if err != nil {
			return common.ErrToStatus(err), err
		}

		return http.StatusOK, nil
	}
}

func (rs *BaseResourceService) PostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	modeParam := r.URL.Query().Get("mode")

	mode, err := strconv.ParseUint(modeParam, 8, 32)
	if err != nil || modeParam == "" {
		mode = 0775
	}

	fileMode := os.FileMode(mode)

	// Directories creation on POST.
	if strings.HasSuffix(r.URL.Path, "/") {
		if err = fileutils.MkdirAllWithChown(files.DefaultFs, r.URL.Path, fileMode); err != nil {
			klog.Errorln(err)
			return common.ErrToStatus(err), err
		}
		return http.StatusOK, nil
	}
	return http.StatusBadRequest, fmt.Errorf("%s is not a valid directory path", r.URL.Path)
}

func (rs *BaseResourceService) PutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	// Only allow PUT for files.
	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	exists, err := afero.Exists(files.DefaultFs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	info, err := fileutils.WriteFile(files.DefaultFs, r.URL.Path, r.Body)
	etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
	w.Header().Set("ETag", etag)

	return common.ErrToStatus(err), err
}

func (rs *BaseResourceService) PatchHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		action := r.URL.Query().Get("action")
		dst, err := common.UnescapeURLIfEscaped(dst)

		if err != nil {
			return common.ErrToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		err = common.CheckParent(src, dst)
		if err != nil {
			return http.StatusBadRequest, err
		}

		rename := r.URL.Query().Get("rename") == "true"
		if !rename {
			if _, err = files.DefaultFs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		if rename {
			dst = common.AddVersionSuffix(dst, files.DefaultFs, strings.HasSuffix(src, "/"))
		}

		klog.Infoln("Before patch action:", src, dst, action, rename)
		err = common.PatchAction(r.Context(), action, src, dst, fileCache)

		return common.ErrToStatus(err), err
	}
}

func (rs *BaseResourceService) RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       r.URL.Path,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})
	if err != nil {
		return common.ErrToStatus(err), err
	}

	if files.IsNamedPipe(file.Mode) {
		SetContentDisposition(w, r, file)
		return 0, nil
	}

	if !file.IsDir {
		return RawFileHandler(w, r, file)
	}

	return RawDirHandler(w, r, d, file)
}

func (rs *BaseResourceService) PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		vars := mux.Vars(r)

		previewSize, err := preview.ParsePreviewSize(vars["size"])
		if err != nil {
			return http.StatusBadRequest, err
		}
		path := "/" + vars["path"]

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.Server.TypeDetectionByHeader,
		})
		if err != nil {
			return common.ErrToStatus(err), err
		}

		SetContentDisposition(w, r, file)

		switch file.Type {
		case "image":
			return HandleImagePreview(w, r, imgSvc, fileCache, file, previewSize, enableThumbnails, resizePreview)
		default:
			return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", file.Type)
		}
	}
}

func (rs *BaseResourceService) PasteSame(action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteDirFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteDirTo(fs afero.Fs, src, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteFileFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) PasteFileTo(fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	return nil, 0, 0, false, fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) MoveDelete(fileCache fileutils.FileCache, src string, ctx context.Context, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) GeneratePathList(db *gorm.DB, pathProcessor PathProcessor) error {
	return fmt.Errorf("Not Implemented")
}

func (rs *BaseResourceService) parsePathToURI(path string) (string, string) {
	return "Error", ""
}
