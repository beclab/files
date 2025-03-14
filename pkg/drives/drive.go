package drives

import (
	"bytes"
	"context"
	"encoding/json"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/preview"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strings"
)

// if cache logic is same as drive, it will be written in this file
type DriveResourceService struct {
	BaseResourceService
}

func generateListingData(listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string, d *common.Data, mountedData []files.DiskInfo) {
	defer close(dataChan)

	var A []*files.FileInfo
	listing.Lock()
	A = append(A, listing.Items...)
	listing.Unlock()

	for len(A) > 0 {
		firstItem := A[0]

		if firstItem.IsDir {
			var file *files.FileInfo
			var err error
			if mountedData != nil {
				file, err = files.NewFileInfoWithDiskInfo(files.FileOptions{
					Fs:         files.DefaultFs,
					Path:       firstItem.Path,
					Modify:     true,
					Expand:     true,
					ReadHeader: d.Server.TypeDetectionByHeader,
					Content:    true,
				}, mountedData)
			} else {
				file, err = files.NewFileInfo(files.FileOptions{
					Fs:         files.DefaultFs,
					Path:       firstItem.Path,
					Modify:     true,
					Expand:     true,
					ReadHeader: d.Server.TypeDetectionByHeader,
					Content:    true,
				})
			}
			if err != nil {
				klog.Error(err)
				return
			}

			var nestedItems []*files.FileInfo
			if file.Listing != nil {
				nestedItems = append(nestedItems, file.Listing.Items...)
			}
			A = append(nestedItems, A[1:]...)
		} else {
			dataChan <- formatSSEvent(firstItem)

			A = A[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func formatSSEvent(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("data: %s\n\n", jsonData)
}

func streamListingItems(w http.ResponseWriter, r *http.Request, listing *files.Listing, d *common.Data, mountedData []files.DiskInfo) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateListingData(listing, stopChan, dataChan, d, mountedData)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case event, ok := <-dataChan:
			if !ok {
				return
			}
			_, err := w.Write([]byte(event))
			if err != nil {
				klog.Error(err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			close(stopChan)
			return
		}
	}
}

func ResourceDriveGetInfo(path string, r *http.Request, d *common.Data) (*files.FileInfo, int, error) {
	xBflUser := r.Header.Get("X-Bfl-User")
	klog.Infoln("X-Bfl-User: ", xBflUser)

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       path,
		Modify:     true,
		Expand:     true,
		ReadHeader: d.Server.TypeDetectionByHeader,
		Content:    true,
	})
	if err != nil {
		return file, common.ErrToStatus(err), err
	}

	if file.IsDir {
		file.Listing.Sorting = files.DefaultSorting
		file.Listing.ApplySort()
		return file, http.StatusOK, nil
	}

	if file.Type == "video" {
		osSystemServer := "system-server.user-system-" + xBflUser

		httpposturl := fmt.Sprintf("http://%s/legacy/v1alpha1/api.intent/v1/server/intent/send", osSystemServer)

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

		body, _ := ioutil.ReadAll(response.Body)
		klog.Infoln("response Body:", string(body))
	}

	return file, http.StatusOK, nil
}

func DriveFileToBuffer(file *files.FileInfo, bufferFilePath string) error {
	path, err := common.UnescapeURLIfEscaped(file.Path)
	if err != nil {
		return err
	}
	klog.Infoln("file.Path:", file.Path, ", path:", path)

	err = fileutils.IoCopyFileWithBufferOs("/data"+path, bufferFilePath, 8*1024*1024)
	if err != nil {
		return err
	}

	return nil
}

func DriveBufferToFile(bufferFilePath string, targetPath string, mode os.FileMode, d *common.Data) (int, error) {
	klog.Infoln("***DriveBufferToFile!")
	klog.Infoln("*** bufferFilePath:", bufferFilePath)
	klog.Infoln("*** targetPath:", targetPath)

	var err error
	targetPath, err = common.UnescapeURLIfEscaped(targetPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Directories creation on POST.
	if strings.HasSuffix(targetPath, "/") {
		if err = files.DefaultFs.MkdirAll(targetPath, mode); err != nil {
			return common.ErrToStatus(err), err
		}
		if err = fileutils.Chown(files.DefaultFs, targetPath, 1000, 1000); err != nil {
			klog.Errorf("can't chown directory %s to user %d: %s", targetPath, 1000, err)
			return common.ErrToStatus(err), err
		}
	}

	_, err = files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       targetPath,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})

	err = fileutils.IoCopyFileWithBufferOs(bufferFilePath, "/data"+targetPath, 8*1024*1024)

	if err != nil {
		_ = files.DefaultFs.RemoveAll(targetPath)
	}

	return common.ErrToStatus(err), err
}

func ResourceDriveDelete(fileCache fileutils.FileCache, path string, ctx context.Context, d *common.Data) (int, error) {
	if path == "/" {
		return http.StatusForbidden, nil
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       path,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})
	if err != nil {
		return common.ErrToStatus(err), err
	}

	// delete thumbnails
	err = preview.DelThumbs(ctx, fileCache, file)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	err = files.DefaultFs.RemoveAll(path)

	if err != nil {
		return common.ErrToStatus(err), err
	}

	return http.StatusOK, nil
}
