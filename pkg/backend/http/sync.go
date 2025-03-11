package http

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	e "errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type resourceGetSyncHandler struct{}

func (h *resourceGetSyncHandler) Handle(w http.ResponseWriter, r *http.Request, stream, meta int, d *data) (int, error) {
	return resourceGetSync(w, r, stream)
}

func resourceGetSync(w http.ResponseWriter, r *http.Request, stream int) (int, error) {
	src := r.URL.Path
	src, err := unescapeURLIfEscaped(src)
	if err != nil {
		return http.StatusBadRequest, err
	}
	fmt.Println("src Path:", src)
	src = strings.Trim(src, "/") + "/"

	firstSlashIdx := strings.Index(src, "/")

	repoID := src[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(src, "/")

	// won't use, because this func is only used for folders
	filename := src[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}
	if prefix == "" {
		prefix = "/"
	}
	prefix = escapeURLWithSpace(prefix)

	fmt.Println("repo-id:", repoID)
	fmt.Println("prefix:", prefix)
	fmt.Println("filename:", filename)

	url := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + prefix + "&with_thumbnail=true"
	fmt.Println(url)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return errToStatus(err), err
	}

	request.Header = r.Header

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return errToStatus(err), err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil
	}

	// SSE
	if stream == 1 {
		var body []byte
		if response.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(response.Body)
			defer reader.Close()
			if err != nil {
				fmt.Println("Error creating gzip reader:", err)
				return errToStatus(err), err
			}

			body, err = ioutil.ReadAll(reader)
			if err != nil {
				fmt.Println("Error reading gzipped response body:", err)
				reader.Close()
				return errToStatus(err), err
			}
		} else {
			body, err = ioutil.ReadAll(response.Body)
			if err != nil {
				fmt.Println("Error reading response body:", err)
				return errToStatus(err), err
			}
		}
		streamSyncDirents(w, r, body, repoID)
		return 0, nil
	}

	// non-SSE
	var responseBody io.Reader = response.Body
	if response.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(response.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return errToStatus(err), err
		}
		defer reader.Close()
		responseBody = reader
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = io.Copy(w, responseBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return errToStatus(err), err
	}

	return 0, nil
}

type Dirent struct {
	Type                 string `json:"type"`
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Mtime                int64  `json:"mtime"`
	Permission           string `json:"permission"`
	ParentDir            string `json:"parent_dir"`
	Size                 int64  `json:"size"`
	FileSize             int64  `json:"fileSize,omitempty"`
	NumTotalFiles        int    `json:"numTotalFiles,omitempty"`
	NumFiles             int    `json:"numFiles,omitempty"`
	NumDirs              int    `json:"numDirs,omitempty"`
	Path                 string `json:"path"`
	Starred              bool   `json:"starred"`
	ModifierEmail        string `json:"modifier_email,omitempty"`
	ModifierName         string `json:"modifier_name,omitempty"`
	ModifierContactEmail string `json:"modifier_contact_email,omitempty"`
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
			path = escapeURLWithSpace(path)
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

func syncMkdirAll(dst string, mode os.FileMode, isDir bool, r *http.Request) error {
	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return err
	}

	firstSlashIdx := strings.Index(dst, "/")

	repoID := dst[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(dst, "/")

	prefix := ""
	if isDir {
		prefix = dst[firstSlashIdx+1:]

	} else {
		if firstSlashIdx != lastSlashIdx {
			prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
		}
	}

	client := &http.Client{}

	// Split the prefix by '/' and generate the URLs
	prefixParts := strings.Split(prefix, "/")
	for i := 0; i < len(prefixParts); i++ {
		curPrefix := strings.Join(prefixParts[:i+1], "/")
		curInfoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace("/"+curPrefix) + "&with_thumbnail=true"
		getRequest, err := http.NewRequest("GET", curInfoURL, nil)
		if err != nil {
			fmt.Printf("create request failed: %v\n", err)
			return err
		}
		getRequest.Header = r.Header
		getResponse, err := client.Do(getRequest)
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			return err
		}
		defer getResponse.Body.Close()
		if getResponse.StatusCode == 200 {
			continue
		} else {
			fmt.Println(getResponse.Status)
		}

		type CreateDirRequest struct {
			Operation string `json:"operation"`
		}

		curCreateURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace("/"+curPrefix)

		createDirReq := CreateDirRequest{
			Operation: "mkdir",
		}
		jsonBody, err := json.Marshal(createDirReq)
		if err != nil {
			fmt.Printf("failed to serialize the request body: %v\n", err)
			return err
		}

		request, err := http.NewRequest("POST", curCreateURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			fmt.Printf("create request failed: %v\n", err)
			return err
		}

		request.Header = r.Header
		request.Header.Set("Content-Type", "application/json")

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			return err
		}
		defer response.Body.Close()

		// Handle the response as needed
		if response.StatusCode != 200 && response.StatusCode != 201 {
			err = e.New("mkdir failed")
			return err
		}
	}
	return nil
}

func syncFileToBuffer(src string, bufferFilePath string, r *http.Request) error {
	src = strings.Trim(src, "/")
	if !strings.Contains(src, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return err
	}

	firstSlashIdx := strings.Index(src, "/")

	repoID := src[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(src, "/")

	filename := src[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}

	dlUrl := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file/" + escapeURLWithSpace(prefix+filename) + "/" + "?dl=1"

	request, err := http.NewRequest("GET", dlUrl, nil)
	if err != nil {
		return err
	}

	request.Header = r.Header

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed，status code：%d", response.StatusCode)
	}

	contentDisposition := response.Header.Get("Content-Disposition")
	if contentDisposition == "" {
		return fmt.Errorf("unrecognizable response format")
	}

	_, params, err := mime.ParseMediaType(contentDisposition)
	if err != nil {
		return err
	}
	filename = params["filename"]

	bufferFile, err := os.OpenFile(bufferFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer bufferFile.Close()

	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()

		_, err = io.Copy(bufferFile, gzipReader)
		if err != nil {
			return err
		}
	} else {
		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}

		_, err = io.Copy(bufferFile, bytes.NewReader(bodyBytes))
		if err != nil {
			return err
		}
	}

	return nil
}

func generateUniqueIdentifier(relativePath string) string {
	h := md5.New()
	io.WriteString(h, relativePath+time.Now().String())
	return fmt.Sprintf("%x%s", h.Sum(nil), relativePath)
}

func syncBufferToFile(bufferFilePath string, dst string, size int64, r *http.Request) (int, error) {
	// Step1: deal with URL
	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return errToStatus(err), err
	}
	dst, err := unescapeURLIfEscaped(dst)
	if err != nil {
		return errToStatus(err), err
	}

	firstSlashIdx := strings.Index(dst, "/")

	repoID := dst[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(dst, "/")

	filename := dst[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
	}

	fmt.Println("dst:", dst)
	fmt.Println("repo-id:", repoID)
	fmt.Println("prefix:", prefix)
	fmt.Println("filename:", filename)

	extension := path.Ext(filename)
	mimeType := "application/octet-stream"
	if extension != "" {
		mimeType = mime.TypeByExtension(extension)
	}

	// step2: GET upload URL
	getUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/upload-link/?p=" + escapeAndJoin("/"+prefix, "/") + "&from=api"
	fmt.Println(getUrl)

	getRequest, err := http.NewRequest("GET", getUrl, nil)
	if err != nil {
		return errToStatus(err), err
	}

	getRequest.Header = r.Header

	getClient := http.Client{}
	getResponse, err := getClient.Do(getRequest)
	if err != nil {
		return errToStatus(err), err
	}
	defer getResponse.Body.Close()

	if getResponse.StatusCode != http.StatusOK {
		err = fmt.Errorf("request failed，status code：%d", getResponse.StatusCode)
		return errToStatus(err), err
	}

	// Read the response body as a string
	getBody, err := io.ReadAll(getResponse.Body)
	if err != nil {
		return errToStatus(err), err
	}
	uploadLink := string(getBody)
	uploadLink = strings.Trim(uploadLink, "\"")

	// step3: deal with upload URL
	targetURL := "http://127.0.0.1:80" + uploadLink + "?ret-json=1"
	fmt.Println(targetURL)

	bufferFile, err := os.Open(bufferFilePath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer bufferFile.Close()

	fileInfo, err := bufferFile.Stat()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	fileSize := fileInfo.Size()

	chunkSize := int64(5 * 1024 * 1024) // 5 MB
	totalChunks := (fileSize + chunkSize - 1) / chunkSize
	identifier := generateUniqueIdentifier(escapeAndJoin(filename, "/"))

	var chunkStart int64 = 0
	for chunkNumber := int64(1); chunkNumber <= totalChunks; chunkNumber++ {
		offset := (chunkNumber - 1) * chunkSize
		chunkData := make([]byte, chunkSize)
		bytesRead, err := bufferFile.ReadAt(chunkData, offset)
		if err != nil && err != io.EOF {
			return http.StatusInternalServerError, err
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fmt.Println("Identifier: ", identifier)
		fmt.Println("Parent Dir: ", "/"+prefix)
		fmt.Println("resumableChunkNumber: ", strconv.FormatInt(chunkNumber, 10))
		fmt.Println("resumableChunkSize: ", strconv.FormatInt(chunkSize, 10))
		fmt.Println("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		fmt.Println("resumableTotalSize", strconv.FormatInt(size, 10))
		fmt.Println("resumableType", mimeType)
		fmt.Println("resumableFilename", filename)
		fmt.Println("resumableRelativePath", filename)
		fmt.Println("resumableTotalChunks", strconv.FormatInt(totalChunks, 10), "\n")

		writer.WriteField("resumableChunkNumber", strconv.FormatInt(chunkNumber, 10))
		writer.WriteField("resumableChunkSize", strconv.FormatInt(chunkSize, 10))
		writer.WriteField("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		writer.WriteField("resumableTotalSize", strconv.FormatInt(size, 10))
		writer.WriteField("resumableType", mimeType)
		writer.WriteField("resumableIdentifier", identifier)
		writer.WriteField("resumableFilename", filename)
		writer.WriteField("resumableRelativePath", filename)
		writer.WriteField("resumableTotalChunks", strconv.FormatInt(totalChunks, 10))
		writer.WriteField("parent_dir", "/"+prefix)

		part, err := writer.CreateFormFile("file", escapeAndJoin(filename, "/"))
		if err != nil {
			fmt.Println("Create Form File error: ", err)
			return http.StatusInternalServerError, err
		}

		_, err = part.Write(chunkData[:bytesRead])
		if err != nil {
			fmt.Println("Write Chunk Data error: ", err)
			return http.StatusInternalServerError, err
		}

		err = writer.Close()
		if err != nil {
			fmt.Println("Write Close error: ", err)
			return http.StatusInternalServerError, err
		}

		request, err := http.NewRequest("POST", targetURL, body)
		if err != nil {
			fmt.Println("New Request error: ", err)
			return http.StatusInternalServerError, err
		}

		request.Header = r.Header
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Content-Disposition", "attachment; filename=\""+escapeAndJoin(filename, "/")+"\"")
		request.Header.Set("Content-Range", "bytes "+strconv.FormatInt(chunkStart, 10)+"-"+strconv.FormatInt(chunkStart+int64(bytesRead)-1, 10)+"/"+strconv.FormatInt(size, 10))
		chunkStart += int64(bytesRead)

		client := http.Client{}
		response, err := client.Do(request)
		fmt.Println("Do Request")
		if err != nil {
			fmt.Println("Do Request error: ", err)
			return http.StatusInternalServerError, err
		}
		defer response.Body.Close()

		// Read the response body as a string
		postBody, err := io.ReadAll(response.Body)
		fmt.Println("ReadAll")
		if err != nil {
			fmt.Println("ReadAll error: ", err)
			return errToStatus(err), err
		}

		fmt.Println("Status Code: ", response.StatusCode)
		if response.StatusCode != http.StatusOK {
			fmt.Println(string(postBody))
			return response.StatusCode, fmt.Errorf("file upload failed, status code: %d", response.StatusCode)
		}
	}
	fmt.Println("sync buffer to file success!")
	return http.StatusOK, nil
}

func resourceSyncDelete(path string, r *http.Request) (int, error) {
	path = strings.Trim(path, "/")
	if !strings.Contains(path, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return errToStatus(err), err
	}

	firstSlashIdx := strings.Index(path, "/")

	repoID := path[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(path, "/")

	filename := path[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = path[firstSlashIdx+1 : lastSlashIdx+1]
	}

	if prefix != "" {
		prefix = "/" + prefix + "/"
	} else {
		prefix = "/"
	}

	targetURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/batch-delete-item/"
	requestBody := map[string]interface{}{
		"dirents":    []string{filename},
		"parent_dir": prefix,
		"repo_id":    repoID,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	request, err := http.NewRequest("DELETE", targetURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	request.Header = r.Header
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	response, err := client.Do(request)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return response.StatusCode, fmt.Errorf("file delete failed with status: %d", response.StatusCode)
	}

	return http.StatusOK, nil
}

func syncPermToMode(permStr string) os.FileMode {
	perm := os.FileMode(0)
	if permStr == "r" {
		perm = perm | 0555
	} else if permStr == "w" {
		perm = perm | 0311
	} else if permStr == "x" {
		perm = perm | 0111
	} else if permStr == "rw" {
		perm = perm | 0755
	} else if permStr == "rx" {
		perm = perm | 0555
	} else if permStr == "wx" {
		perm = perm | 0311
	} else if permStr == "rwx" {
		perm = perm | 0755
	} else {
		fmt.Println("invalid permission string")
		return 0
	}

	return perm
}
