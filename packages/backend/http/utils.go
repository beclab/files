package http

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	libErrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
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

//func generateListingData(listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string) {
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

func generateListingData(listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string, d *data, usbData, hddData []files.DiskInfo) {
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

	go generateListingData(listing, stopChan, dataChan, d, usbData, hddData)

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

type Dirent struct {
	Type                 string `json:"type"`                             // 目录项类型（文件或目录）
	ID                   string `json:"id"`                               // 目录项ID
	Name                 string `json:"name"`                             // 目录项名称
	Mtime                int64  `json:"mtime"`                            // 修改时间（Unix时间戳）
	Permission           string `json:"permission"`                       // 权限
	ParentDir            string `json:"parent_dir"`                       // 父目录路径
	Size                 int64  `json:"size"`                             // 目录项大小（对于文件）
	FileSize             int64  `json:"fileSize,omitempty"`               // 文件大小（如果与Size不同）
	NumTotalFiles        int    `json:"numTotalFiles,omitempty"`          // 总文件数（对于目录）
	NumFiles             int    `json:"numFiles,omitempty"`               // 文件数（对于目录）
	NumDirs              int    `json:"numDirs,omitempty"`                // 目录数（对于目录）
	Path                 string `json:"path"`                             // 目录项完整路径
	Starred              bool   `json:"starred"`                          // 是否标记为星标
	ModifierEmail        string `json:"modifier_email,omitempty"`         // 修改者邮箱（对于文件）
	ModifierName         string `json:"modifier_name,omitempty"`          // 修改者名称（对于文件）
	ModifierContactEmail string `json:"modifier_contact_email,omitempty"` // 修改者联系邮箱（对于文件）
}

type DirentResponse struct {
	UserPerm   string   `json:"user_perm"`
	DirID      string   `json:"dir_id"`
	DirentList []Dirent `json:"dirent_list"`
	sync.Mutex
}

func generateDirentsData(body []byte, stopChan <-chan struct{}, dataChan chan<- string, r *http.Request, repoID string) {
	defer close(dataChan)

	var bodyJson DirentResponse
	if err := json.Unmarshal(body, &bodyJson); err != nil {
		fmt.Println(err)
		return
	}

	var A []Dirent
	bodyJson.Lock()
	A = append(A, bodyJson.DirentList...)
	bodyJson.Unlock()

	for len(A) > 0 {
		fmt.Println("len(A): ", len(A))
		firstItem := A[0]
		fmt.Println("firstItem Path: ", firstItem.Path)
		fmt.Println("firstItem Name:", firstItem.Name)

		if firstItem.Type == "dir" {
			path := firstItem.Path
			if path != "/" {
				path += "/"
			}
			path = url.QueryEscape(path)
			firstUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + path + "&with_thumbnail=true"
			fmt.Println(firstUrl)

			firstRequest, err := http.NewRequest("GET", firstUrl, nil)
			if err != nil {
				fmt.Println(err)
				return
			}

			firstRequest.Header = r.Header

			client := http.Client{}
			firstResponse, err := client.Do(firstRequest)
			if err != nil {
				return
			}
			//defer response.Body.Close()

			if firstResponse.StatusCode != http.StatusOK {
				fmt.Println(firstResponse.StatusCode)
				return
			}

			var firstRespBody []byte
			var reader *gzip.Reader = nil
			if firstResponse.Header.Get("Content-Encoding") == "gzip" {
				reader, err = gzip.NewReader(firstResponse.Body)
				if err != nil {
					fmt.Println("Error creating gzip reader:", err)
					return
				}

				firstRespBody, err = ioutil.ReadAll(reader)
				if err != nil {
					fmt.Println("Error reading gzipped response body:", err)
					reader.Close()
					return
				}
			} else {
				firstRespBody, err = ioutil.ReadAll(firstResponse.Body)
				if err != nil {
					fmt.Println("Error reading response body:", err)
					firstResponse.Body.Close()
					return
				}
			}

			var firstBodyJson DirentResponse
			if err := json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
				fmt.Println(err)
				return
			}

			A = append(firstBodyJson.DirentList, A[1:]...)

			if reader != nil {
				reader.Close()
			}
			firstResponse.Body.Close()
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

func streamSyncDirents(w http.ResponseWriter, r *http.Request, body []byte, repoID string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateDirentsData(body, stopChan, dataChan, r, repoID)

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

func outputHeader(r *http.Request) {
	for name, values := range r.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}
	return
}

func getOwner(r *http.Request) (ownerID, ownerName string) {
	BflName := r.Header.Get("X-Bfl-User")
	rawURL := r.Header.Get("Referer")
	if !strings.HasSuffix(rawURL, "/") {
		rawURL += "/"
	}

	// initial return value
	ownerName = BflName
	ownerID = ownerName

	indexName := strings.Index(rawURL, BflName)
	if indexName == -1 {
		fmt.Println("BflName not found in URL")
		return
	}

	indexSlash := strings.Index(rawURL[indexName:], "/")
	if indexSlash == -1 {
		fmt.Println("No '/' found after BflName in URL")
		return
	}

	domainStart := indexName + len(BflName) + 1
	domainEnd := indexName + indexSlash
	domain := rawURL[domainStart:domainEnd]

	email := fmt.Sprintf("%s@%s", BflName, domain)
	fmt.Println("Generated email:", email)
	ownerID = email
	return
}

func stringMD5(s string) string {
	hasher := md5.New()
	hasher.Write([]byte(s))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)
	fmt.Printf("MD5 Hash of '%s': %s\n", s, hashString)
	return hashString
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
