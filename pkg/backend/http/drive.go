package http

import (
	"bytes"
	"files/pkg/backend/errors"
	"files/pkg/backend/files"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// if cache logic is same as drive, it will be written in this file

type resourceGetDriveCacheHandler struct{}

func (h *resourceGetDriveCacheHandler) Handle(w http.ResponseWriter, r *http.Request, stream, meta int, d *data) (int, error) {
	return resourceGetDriveCache(w, r, stream, d)
}

func resourceGetDriveCache(w http.ResponseWriter, r *http.Request, stream int, d *data) (int, error) {
	xBflUser := r.Header.Get("X-Bfl-User")
	fmt.Println("X-Bfl-User: ", xBflUser)

	var mountedData []files.DiskInfo = nil
	var err error = nil
	if files.TerminusdHost != "" {
		// for 1.12: path-incluster URL exists, won't err in normal condition
		// for 1.11: path-incluster URL may not exist, if err, use usb-incluster and hdd-incluster for system functional
		url := "http://" + files.TerminusdHost + "/system/mounted-path-incluster"

		headers := r.Header.Clone()
		headers.Set("Content-Type", "application/json")
		headers.Set("X-Signature", "temp_signature")

		mountedData, err = files.FetchDiskInfo(url, headers)
		if err != nil {
			log.Printf("Failed to fetch data from %s: %v", url, err)
			usbUrl := "http://" + files.TerminusdHost + "/system/mounted-usb-incluster"

			usbHeaders := r.Header.Clone()
			usbHeaders.Set("Content-Type", "application/json")
			usbHeaders.Set("X-Signature", "temp_signature")

			usbData, err := files.FetchDiskInfo(usbUrl, usbHeaders)
			if err != nil {
				log.Printf("Failed to fetch data from %s: %v", usbUrl, err)
			}

			fmt.Println("USB Data:", usbData)

			hddUrl := "http://" + files.TerminusdHost + "/system/mounted-hdd-incluster"

			hddHeaders := r.Header.Clone()
			hddHeaders.Set("Content-Type", "application/json")
			hddHeaders.Set("X-Signature", "temp_signature")

			hddData, err := files.FetchDiskInfo(hddUrl, hddHeaders)
			if err != nil {
				log.Printf("Failed to fetch data from %s: %v", hddUrl, err)
			}

			fmt.Println("HDD Data:", hddData)

			for _, item := range usbData {
				item.Type = "usb"
				mountedData = append(mountedData, item)
			}

			for _, item := range hddData {
				item.Type = "hdd"
				mountedData = append(mountedData, item)
			}
		}
		fmt.Println("Mounted Data:", mountedData)
	}

	var file *files.FileInfo
	if mountedData != nil {
		file, err = files.NewFileInfoWithDiskInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.server.TypeDetectionByHeader,
			Content:    true,
		}, mountedData)
	} else {
		file, err = files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       r.URL.Path,
			Modify:     true,
			Expand:     true,
			ReadHeader: d.server.TypeDetectionByHeader,
			Content:    true,
		})
	}
	if err != nil {
		if errToStatus(err) == http.StatusNotFound && r.URL.Path == "/External/" {
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

			return renderJSON(w, r, file)
		}
		return errToStatus(err), err
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
			return renderJSON(w, r, file)
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

		fmt.Println("HTTP JSON POST URL:", httpposturl)

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

		fmt.Println("response Status:", response.Status)
		fmt.Println("response Headers:", response.Header)
		body, _ := ioutil.ReadAll(response.Body)
		fmt.Println("response Body:", string(body))
	}
	return renderJSON(w, r, file)
}
