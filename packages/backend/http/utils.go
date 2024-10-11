package http

import (
	"encoding/json"
	"errors"
	"fmt"
	libErrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func renderJSON(w http.ResponseWriter, _ *http.Request, data interface{}) (int, error) {
	marsh, err := json.Marshal(data)

	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(marsh); err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

//func generateData(listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string) {
//	defer close(dataChan)
//
//	listing.Lock()
//	for _, item := range listing.Items {
//		select {
//		case <-stopChan:
//			listing.Unlock()
//			return
//		case dataChan <- formatSSEvent(item):
//		}
//	}
//	listing.Unlock()
//
//	itemCount := len(listing.Items)
//	for itemCount < 100000 {
//		select {
//		case <-stopChan:
//			return
//		default:
//			item := &files.FileInfo{
//				Path: fmt.Sprintf("/path/to/item%d", itemCount),
//				Name: fmt.Sprintf("item%d", itemCount),
//				Size: int64(itemCount * 100),
//			}
//			dataChan <- formatSSEvent(item)
//			itemCount++
//
//			time.Sleep(100 * time.Millisecond)
//		}
//	}
//}

func generateData(listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string, d *data, usbData, hddData []files.DiskInfo) {
	defer close(dataChan)

	var A []*files.FileInfo
	listing.Lock()
	A = append(A, listing.Items...)
	listing.Unlock()

	for len(A) > 0 {
		//fmt.Println("len(A): ", len(A))
		firstItem := A[0]
		//fmt.Println("firstItem: ", firstItem.Path)

		if firstItem.IsDir {
			var file *files.FileInfo
			var err error
			if usbData != nil || hddData != nil {
				file, err = files.NewFileInfoWithDiskInfo(files.FileOptions{
					Fs:         d.user.Fs,
					Path:       firstItem.Path, //r.URL.Path,
					Modify:     d.user.Perm.Modify,
					Expand:     true,
					ReadHeader: d.server.TypeDetectionByHeader,
					Checker:    d,
					Content:    true,
				}, usbData, hddData)
			} else {
				file, err = files.NewFileInfo(files.FileOptions{
					Fs:         d.user.Fs,
					Path:       firstItem.Path, //r.URL.Path,
					Modify:     d.user.Perm.Modify,
					Expand:     true,
					ReadHeader: d.server.TypeDetectionByHeader,
					Checker:    d,
					Content:    true,
				})
			}
			if err != nil {
				fmt.Println(err)
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

func streamListingItems(w http.ResponseWriter, r *http.Request, listing *files.Listing, d *data, usbData, hddData []files.DiskInfo) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateData(listing, stopChan, dataChan, d, usbData, hddData)

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
				fmt.Println(err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			close(stopChan)
			return
		}
	}
}

func errToStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case os.IsPermission(err):
		return http.StatusForbidden
	case os.IsNotExist(err), err == libErrors.ErrNotExist:
		return http.StatusNotFound
	case os.IsExist(err), err == libErrors.ErrExist:
		return http.StatusConflict
	case errors.Is(err, libErrors.ErrPermissionDenied):
		return http.StatusForbidden
	case errors.Is(err, libErrors.ErrInvalidRequestParams):
		return http.StatusBadRequest
	case errors.Is(err, libErrors.ErrRootUserDeletion):
		return http.StatusForbidden
	case err.Error() == "file size exceeds 4GB":
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

// This is an addaptation if http.StripPrefix in which we don't
// return 404 if the page doesn't have the needed prefix.
func stripPrefix(prefix string, h http.Handler) http.Handler {
	if prefix == "" || prefix == "/" {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, prefix)
		rp := strings.TrimPrefix(r.URL.RawPath, prefix)
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p
		r2.URL.RawPath = rp
		h.ServeHTTP(w, r2)
	})
}
