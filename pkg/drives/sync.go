package drives

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/errors"
	"files/pkg/fileutils"
	"files/pkg/preview"
	"fmt"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SyncResourceService struct {
	BaseResourceService
}

func (rc *SyncResourceService) GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	streamStr := r.URL.Query().Get("stream")
	stream := 0
	var err error
	if streamStr != "" {
		stream, err = strconv.Atoi(streamStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	src := r.URL.Path
	src, err = common.UnescapeURLIfEscaped(src)
	if err != nil {
		return http.StatusBadRequest, err
	}
	klog.Infoln("src Path:", src)
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
	prefix = common.EscapeURLWithSpace(prefix)

	klog.Infoln("repo-id:", repoID)
	klog.Infoln("prefix:", prefix)
	klog.Infoln("filename:", filename)

	url := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + prefix + "&with_thumbnail=true"
	klog.Infoln(url)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	request.Header = r.Header

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return common.ErrToStatus(err), err
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
				klog.Errorln("Error creating gzip reader:", err)
				return common.ErrToStatus(err), err
			}

			body, err = ioutil.ReadAll(reader)
			if err != nil {
				klog.Errorln("Error reading gzipped response body:", err)
				reader.Close()
				return common.ErrToStatus(err), err
			}
		} else {
			body, err = ioutil.ReadAll(response.Body)
			if err != nil {
				klog.Errorln("Error reading response body:", err)
				return common.ErrToStatus(err), err
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
			klog.Errorln("Error creating gzip reader:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return common.ErrToStatus(err), err
		}
		defer reader.Close()
		responseBody = reader
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = io.Copy(w, responseBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return common.ErrToStatus(err), err
	}

	return 0, nil
}

func (rc *SyncResourceService) DeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		return ResourceSyncDelete(r.URL.Path, r)
	}
}

func (rc *SyncResourceService) PostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	err := SyncMkdirAll(r.URL.Path, 0, true, r)
	return common.ErrToStatus(err), err
}

func (rc *SyncResourceService) PutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	// TODO: Sync support editing, but not in this structure for the time being. This func is reserving a slot for sync.
	return http.StatusNotImplemented, fmt.Errorf("sync drive does not supoort editing files for the time being")
}

func (rc *SyncResourceService) PatchHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		src := r.URL.Path
		dst := r.URL.Query().Get("destination")

		action := r.URL.Query().Get("action")
		var err error
		src, err = common.UnescapeURLIfEscaped(src)
		if err != nil {
			return common.ErrToStatus(err), err
		}
		dst, err = common.UnescapeURLIfEscaped(dst)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		err = ResourceSyncPatch(action, src, dst, r)
		return common.ErrToStatus(err), err
	}
}

func (rs *SyncResourceService) RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	return http.StatusNotImplemented, nil
}

func (rs *SyncResourceService) PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		return http.StatusNotImplemented, nil
	}
}

func (rc *SyncResourceService) PasteSame(action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	return ResourceSyncPatch(action, src, dst, r)
}

func (rs *SyncResourceService) PasteDirFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	mode := fileMode

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteDirTo(fs, src, dst, mode, w, r, d, driveIdCache)
	if err != nil {
		return err
	}

	var fdstBase string = dst
	if driveIdCache[src] != "" {
		fdstBase = filepath.Join(filepath.Dir(filepath.Dir(strings.TrimSuffix(dst, "/"))), driveIdCache[src])
	}

	type Item struct {
		Type                 string `json:"type"`
		Name                 string `json:"name"`
		ID                   string `json:"id"`
		Mtime                int64  `json:"mtime"`
		Permission           string `json:"permission"`
		Size                 int64  `json:"size,omitempty"`
		ModifierEmail        string `json:"modifier_email,omitempty"`
		ModifierContactEmail string `json:"modifier_contact_email,omitempty"`
		ModifierName         string `json:"modifier_name,omitempty"`
		Starred              bool   `json:"starred,omitempty"`
		FileSize             int64  `json:"fileSize,omitempty"`
		NumTotalFiles        int    `json:"numTotalFiles,omitempty"`
		NumFiles             int    `json:"numFiles,omitempty"`
		NumDirs              int    `json:"numDirs,omitempty"`
		Path                 string `json:"path,omitempty"`
		EncodedThumbnailSrc  string `json:"encoded_thumbnail_src,omitempty"`
	}

	type ResponseData struct {
		UserPerm   string `json:"user_perm"`
		DirID      string `json:"dir_id"`
		DirentList []Item `json:"dirent_list"`
	}

	src = strings.Trim(src, "/")
	if !strings.Contains(src, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
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

	infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace("/"+prefix+"/"+filename) + "&with_thumbnail=true"

	client := &http.Client{}
	request, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		klog.Errorf("create request failed: %v\n", err)
		return err
	}

	request.Header = r.Header

	response, err := client.Do(request)
	if err != nil {
		klog.Errorf("request failed: %v\n", err)
		return err
	}
	defer response.Body.Close()

	var bodyReader io.Reader = response.Body

	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			klog.Errorf("unzip response failed: %v\n", err)
			return err
		}
		defer gzipReader.Close()

		bodyReader = gzipReader
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		klog.Errorf("read response failed: %v\n", err)
		return err
	}

	var data ResponseData
	err = json.Unmarshal(body, &data)
	if err != nil {
		return err
	}

	for _, item := range data.DirentList {
		fsrc := filepath.Join(src, item.Name)
		fdst := filepath.Join(fdstBase, item.Name)

		if item.Type == "dir" {
			err := rs.PasteDirFrom(fs, srcType, fsrc, dstType, fdst, d, SyncPermToMode(item.Permission), w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			err := rs.PasteFileFrom(fs, srcType, fsrc, dstType, fdst, d, SyncPermToMode(item.Permission), item.Size, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *SyncResourceService) PasteDirTo(fs afero.Fs, src, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	if err := SyncMkdirAll(dst, fileMode, true, r); err != nil {
		return err
	}
	return nil
}

func (rs *SyncResourceService) PasteFileFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	extRemains := IsThridPartyDrives(dstType)
	var bufferPath string

	var err error
	_, err = CheckBufferDiskSpace(diskSize)
	if err != nil {
		return err
	}
	bufferPath, err = GenerateBufferFileName(src, bflName, extRemains)
	if err != nil {
		return err
	}

	err = MakeDiskBuffer(bufferPath, diskSize, false)
	if err != nil {
		return err
	}
	err = SyncFileToBuffer(src, bufferPath, r)
	if err != nil {
		return err
	}

	defer func() {
		klog.Infoln("Begin to remove buffer")
		RemoveDiskBuffer(bufferPath, srcType)
	}()

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteFileTo(fs, bufferPath, dst, mode, w, r, d, diskSize)
	if err != nil {
		return err
	}
	return nil
}

func (rs *SyncResourceService) PasteFileTo(fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	klog.Infoln("Begin to sync paste!")
	if err := SyncMkdirAll(dst, fileMode, false, r); err != nil {
		return err
	}
	status, err := SyncBufferToFile(bufferPath, dst, diskSize, r)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		klog.Errorln("Sync paste failed! err: ", err)
		return err
	}
	return nil
}

func (rs *SyncResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}

	src = strings.Trim(src, "/")
	if !strings.Contains(src, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
		return nil, 0, 0, false, err
	}

	firstSlashIdx := strings.Index(src, "/")

	repoID := src[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(src, "/")

	filename := src[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}

	infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace("/"+prefix) + "&with_thumbnail=true"

	client := &http.Client{}
	request, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		klog.Errorf("create request failed: %v\n", err)
		return nil, 0, 0, false, err
	}

	request.Header = r.Header

	response, err := client.Do(request)
	if err != nil {
		klog.Errorf("request failed: %v\n", err)
		return nil, 0, 0, false, err
	}
	defer response.Body.Close()

	var bodyReader io.Reader = response.Body

	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			klog.Errorf("unzip response failed: %v\n", err)
			return nil, 0, 0, false, err
		}
		defer gzipReader.Close()

		bodyReader = gzipReader
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		klog.Errorf("read response failed: %v\n", err)
		return nil, 0, 0, false, err
	}

	type Dirent struct {
		Type                 string `json:"type"`
		ID                   string `json:"id"`
		Name                 string `json:"name"`
		Mtime                int64  `json:"mtime"`
		Permission           string `json:"permission"`
		ParentDir            string `json:"parent_dir"`
		Starred              bool   `json:"starred"`
		Size                 int64  `json:"size"`
		FileSize             int64  `json:"fileSize,omitempty"`
		NumTotalFiles        int    `json:"numTotalFiles,omitempty"`
		NumFiles             int    `json:"numFiles,omitempty"`
		NumDirs              int    `json:"numDirs,omitempty"`
		Path                 string `json:"path,omitempty"`
		ModifierEmail        string `json:"modifier_email,omitempty"`
		ModifierName         string `json:"modifier_name,omitempty"`
		ModifierContactEmail string `json:"modifier_contact_email,omitempty"`
	}

	type Response struct {
		UserPerm   string   `json:"user_perm"`
		DirID      string   `json:"dir_id"`
		DirentList []Dirent `json:"dirent_list"`
	}

	var dirResp Response
	var fileInfo Dirent

	err = json.Unmarshal(body, &dirResp)
	if err != nil {
		klog.Errorf("parse response failed: %v\n", err)
		return nil, 0, 0, false, err
	}

	var found = false
	for _, dirent := range dirResp.DirentList {
		if dirent.Name == filename {
			fileInfo = dirent
			found = true
			break
		}
	}
	if found {
		mode := SyncPermToMode(fileInfo.Permission)
		isDir := false
		if fileInfo.Type == "dir" {
			isDir = true
		}
		return nil, fileInfo.Size, mode, isDir, nil
	} else {
		err = e.New("sync file info not found")
		return nil, 0, 0, false, err
	}
}

func (rs *SyncResourceService) MoveDelete(fileCache fileutils.FileCache, src string, ctx context.Context, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	status, err := ResourceSyncDelete(src, r)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *SyncResourceService) GeneratePathList(db *gorm.DB, processor PathProcessor) error {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)

	go func() {
		for {
			time.Sleep(1 * time.Second)
			mu.Lock()
			if len(common.BflCookieCache) > 0 {
				klog.Info("~~~Temp log: cookie has come")
				cond.Broadcast()
			} else {
				klog.Info("~~~Temp log: cookie hasn't come")
			}
			mu.Unlock()
		}
	}()

	type Repo struct {
		Type                 string    `json:"type"`
		RepoID               string    `json:"repo_id"`
		RepoName             string    `json:"repo_name"`
		OwnerEmail           string    `json:"owner_email"`
		OwnerName            string    `json:"owner_name"`
		OwnerContactEmail    string    `json:"owner_contact_email"`
		LastModified         time.Time `json:"last_modified"`
		ModifierEmail        string    `json:"modifier_email"`
		ModifierName         string    `json:"modifier_name"`
		ModifierContactEmail string    `json:"modifier_contact_email"`
		Size                 int       `json:"size"`
		Encrypted            bool      `json:"encrypted"`
		Permission           string    `json:"permission"`
		Starred              bool      `json:"starred"`
		Monitored            bool      `json:"monitored"`
		Status               string    `json:"status"`
		Salt                 string    `json:"salt"`
	}

	type RepoResponse struct {
		Repos []Repo `json:"repos"`
	}

	for {
		mu.Lock()
		for len(common.BflCookieCache) == 0 {
			klog.Info("~~~Temp log: waiting for cookie")
			cond.Wait()
		}

		klog.Info("~~~Temp log: cookie ready, run!")
		for bflName, cookie := range common.BflCookieCache {
			fmt.Printf("Key: %s, Value: %s\n", bflName, cookie)
			repoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/?type=mine"

			header := make(http.Header)

			header.Set("Content-Type", "application/json")
			header.Set("Cookie", cookie)

			repoRespBody, err := syncCall(repoURL, "GET", nil, nil, nil, &header, true)

			var data RepoResponse
			err = json.Unmarshal(repoRespBody, &data)
			if err != nil {
				return err
			}

			for _, repo := range data.Repos {
				klog.Infof("repo=%v", repo)
			}
		}
		mu.Unlock()
		return nil
	}

	//type Item struct {
	//	Type                 string `json:"type"`
	//	Name                 string `json:"name"`
	//	ID                   string `json:"id"`
	//	Mtime                int64  `json:"mtime"`
	//	Permission           string `json:"permission"`
	//	Size                 int64  `json:"size,omitempty"`
	//	ModifierEmail        string `json:"modifier_email,omitempty"`
	//	ModifierContactEmail string `json:"modifier_contact_email,omitempty"`
	//	ModifierName         string `json:"modifier_name,omitempty"`
	//	Starred              bool   `json:"starred,omitempty"`
	//	FileSize             int64  `json:"fileSize,omitempty"`
	//	NumTotalFiles        int    `json:"numTotalFiles,omitempty"`
	//	NumFiles             int    `json:"numFiles,omitempty"`
	//	NumDirs              int    `json:"numDirs,omitempty"`
	//	Path                 string `json:"path,omitempty"`
	//	EncodedThumbnailSrc  string `json:"encoded_thumbnail_src,omitempty"`
	//}
	//
	//type ResponseData struct {
	//	UserPerm   string `json:"user_perm"`
	//	DirID      string `json:"dir_id"`
	//	DirentList []Item `json:"dirent_list"`
	//}
	//
	//src = strings.Trim(src, "/")
	//if !strings.Contains(src, "/") {
	//	err := e.New("invalid path format: path must contain at least one '/'")
	//	klog.Errorln("Error:", err)
	//	return err
	//}
	//
	//firstSlashIdx := strings.Index(src, "/")
	//
	//repoID := src[:firstSlashIdx]
	//
	//lastSlashIdx := strings.LastIndex(src, "/")
	//
	//filename := src[lastSlashIdx+1:]
	//
	//prefix := ""
	//if firstSlashIdx != lastSlashIdx {
	//	prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	//}
	//
	//infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace("/"+prefix+"/"+filename) + "&with_thumbnail=true"
	//
	//client := &http.Client{}
	//request, err := http.NewRequest("GET", infoURL, nil)
	//if err != nil {
	//	klog.Errorf("create request failed: %v\n", err)
	//	return err
	//}
	//
	//request.Header = r.Header
	//
	//response, err := client.Do(request)
	//if err != nil {
	//	klog.Errorf("request failed: %v\n", err)
	//	return err
	//}
	//defer response.Body.Close()
	//
	//var bodyReader io.Reader = response.Body
	//
	//if response.Header.Get("Content-Encoding") == "gzip" {
	//	gzipReader, err := gzip.NewReader(response.Body)
	//	if err != nil {
	//		klog.Errorf("unzip response failed: %v\n", err)
	//		return err
	//	}
	//	defer gzipReader.Close()
	//
	//	bodyReader = gzipReader
	//}
	//
	//body, err := ioutil.ReadAll(bodyReader)
	//if err != nil {
	//	klog.Errorf("read response failed: %v\n", err)
	//	return err
	//}
	//
	//var data ResponseData
	//err = json.Unmarshal(body, &data)
	//if err != nil {
	//	return err
	//}
	//
	//for _, item := range data.DirentList {
	//	fsrc := filepath.Join(src, item.Name)
	//	fdst := filepath.Join(fdstBase, item.Name)
	//
	//	if item.Type == "dir" {
	//		err := rs.PasteDirFrom(fs, srcType, fsrc, dstType, fdst, d, SyncPermToMode(item.Permission), w, r, driveIdCache)
	//		if err != nil {
	//			return err
	//		}
	//	} else {
	//		err := rs.PasteFileFrom(fs, srcType, fsrc, dstType, fdst, d, SyncPermToMode(item.Permission), item.Size, w, r, driveIdCache)
	//		if err != nil {
	//			return err
	//		}
	//	}
	//}
	//return nil

	//rootPath := "/data"
	//
	//err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
	//	if err != nil {
	//		klog.Errorf("Access error: %v\n", err)
	//		return nil
	//	}
	//
	//	if info.IsDir() {
	//		if info.Mode()&os.ModeSymlink != 0 {
	//			return filepath.SkipDir
	//		}
	//		// Process directory
	//		drive, parsedPath := rs.parsePathToURI(path)
	//		return processor(db, drive, parsedPath, info.ModTime())
	//	}
	//
	//	// Process file (if needed)
	//	// Uncomment the following line if you need to process files
	//	// processFile(db, drive, path, info.ModTime())
	//
	//	return nil
	//})
	//
	//if err != nil {
	//	fmt.Println("Error walking the path:", err)
	//}
	//return err
}

func (rs *SyncResourceService) parsePathToURI(path string) (string, string) {
	pathSplit := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(pathSplit) < 2 {
		return "Unknown", path
	}
	if strings.HasPrefix(pathSplit[1], "pvc-userspace-") {
		if len(pathSplit) == 2 {
			return "Unknown", path
		}
		if pathSplit[2] == "Data" {
			return "data", filepath.Join(pathSplit[1:]...)
		} else if pathSplit[2] == "Home" {
			return "drive", filepath.Join(pathSplit[1:]...)
		}
	}
	if pathSplit[1] == "External" {
		return "External", filepath.Join(pathSplit[2:]...) // TODO: External types
	}
	return "Error", path
}

func syncCall(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, header *http.Header, returnResp bool) ([]byte, error) {
	// w is for future use, not used now
	client := &http.Client{}
	request, err := http.NewRequest(method, dst, bytes.NewBuffer(reqBodyJson))
	if err != nil {
		klog.Errorf("create request failed: %v\n", err)
		return nil, err
	}

	if header != nil {
		request.Header = *header
	} else {
		request.Header = r.Header
	}

	response, err := client.Do(request)
	if err != nil {
		klog.Errorf("request failed: %v\n", err)
		return nil, err
	}
	defer response.Body.Close()

	var bodyReader io.Reader = response.Body

	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			klog.Errorf("unzip response failed: %v\n", err)
			return nil, err
		}
		defer gzipReader.Close()

		bodyReader = gzipReader
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		klog.Errorf("read response failed: %v\n", err)
		return nil, err
	}

	if returnResp {
		return body, nil
	}
	return nil, nil
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
		klog.Error(err)
		return
	}

	var A []Dirent
	bodyJson.Lock()
	A = append(A, bodyJson.DirentList...)
	bodyJson.Unlock()

	for len(A) > 0 {
		klog.Infoln("len(A): ", len(A))
		firstItem := A[0]
		klog.Infoln("firstItem Path: ", firstItem.Path)
		klog.Infoln("firstItem Name:", firstItem.Name)

		if firstItem.Type == "dir" {
			path := firstItem.Path
			if path != "/" {
				path += "/"
			}
			path = common.EscapeURLWithSpace(path)
			firstUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + path + "&with_thumbnail=true"
			klog.Infoln(firstUrl)

			firstRequest, err := http.NewRequest("GET", firstUrl, nil)
			if err != nil {
				klog.Error(err)
				return
			}

			firstRequest.Header = r.Header

			client := http.Client{}
			firstResponse, err := client.Do(firstRequest)
			if err != nil {
				return
			}

			if firstResponse.StatusCode != http.StatusOK {
				klog.Infoln(firstResponse.StatusCode)
				return
			}

			var firstRespBody []byte
			var reader *gzip.Reader = nil
			if firstResponse.Header.Get("Content-Encoding") == "gzip" {
				reader, err = gzip.NewReader(firstResponse.Body)
				if err != nil {
					klog.Errorln("Error creating gzip reader:", err)
					return
				}

				firstRespBody, err = ioutil.ReadAll(reader)
				if err != nil {
					klog.Errorln("Error reading gzipped response body:", err)
					reader.Close()
					return
				}
			} else {
				firstRespBody, err = ioutil.ReadAll(firstResponse.Body)
				if err != nil {
					klog.Errorln("Error reading response body:", err)
					firstResponse.Body.Close()
					return
				}
			}

			var firstBodyJson DirentResponse
			if err := json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
				klog.Error(err)
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

func SyncMkdirAll(dst string, mode os.FileMode, isDir bool, r *http.Request) error {
	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
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
		curInfoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace("/"+curPrefix) + "&with_thumbnail=true"
		getRequest, err := http.NewRequest("GET", curInfoURL, nil)
		if err != nil {
			klog.Errorf("create request failed: %v\n", err)
			return err
		}
		getRequest.Header = r.Header
		getResponse, err := client.Do(getRequest)
		if err != nil {
			klog.Errorf("request failed: %v\n", err)
			return err
		}
		defer getResponse.Body.Close()
		if getResponse.StatusCode == 200 {
			continue
		} else {
			klog.Infoln(getResponse.Status)
		}

		type CreateDirRequest struct {
			Operation string `json:"operation"`
		}

		curCreateURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace("/"+curPrefix)

		createDirReq := CreateDirRequest{
			Operation: "mkdir",
		}
		jsonBody, err := json.Marshal(createDirReq)
		if err != nil {
			klog.Errorf("failed to serialize the request body: %v\n", err)
			return err
		}

		request, err := http.NewRequest("POST", curCreateURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			klog.Errorf("create request failed: %v\n", err)
			return err
		}

		request.Header = r.Header
		request.Header.Set("Content-Type", "application/json")

		response, err := client.Do(request)
		if err != nil {
			klog.Errorf("request failed: %v\n", err)
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

func SyncFileToBuffer(src string, bufferFilePath string, r *http.Request) error {
	src = strings.Trim(src, "/")
	if !strings.Contains(src, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
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

	dlUrl := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file/" + common.EscapeURLWithSpace(prefix+filename) + "/" + "?dl=1"

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

func SyncBufferToFile(bufferFilePath string, dst string, size int64, r *http.Request) (int, error) {
	// Step1: deal with URL
	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
		return common.ErrToStatus(err), err
	}
	dst, err := common.UnescapeURLIfEscaped(dst)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	firstSlashIdx := strings.Index(dst, "/")

	repoID := dst[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(dst, "/")

	filename := dst[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
	}

	klog.Infoln("dst:", dst)
	klog.Infoln("repo-id:", repoID)
	klog.Infoln("prefix:", prefix)
	klog.Infoln("filename:", filename)

	extension := path.Ext(filename)
	mimeType := "application/octet-stream"
	if extension != "" {
		mimeType = mime.TypeByExtension(extension)
	}

	// step2: GET upload URL
	getUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/upload-link/?p=" + common.EscapeAndJoin("/"+prefix, "/") + "&from=api"
	klog.Infoln(getUrl)

	getRequest, err := http.NewRequest("GET", getUrl, nil)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	getRequest.Header = r.Header

	getClient := http.Client{}
	getResponse, err := getClient.Do(getRequest)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	defer getResponse.Body.Close()

	if getResponse.StatusCode != http.StatusOK {
		err = fmt.Errorf("request failed，status code：%d", getResponse.StatusCode)
		return common.ErrToStatus(err), err
	}

	// Read the response body as a string
	getBody, err := io.ReadAll(getResponse.Body)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	uploadLink := string(getBody)
	uploadLink = strings.Trim(uploadLink, "\"")

	// step3: deal with upload URL
	targetURL := "http://127.0.0.1:80" + uploadLink + "?ret-json=1"
	klog.Infoln(targetURL)

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
	identifier := generateUniqueIdentifier(common.EscapeAndJoin(filename, "/"))

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

		//klog.Infoln("Identifier: ", identifier)
		//klog.Infoln("Parent Dir: ", "/"+prefix)
		//klog.Infoln("resumableChunkNumber: ", strconv.FormatInt(chunkNumber, 10))
		//klog.Infoln("resumableChunkSize: ", strconv.FormatInt(chunkSize, 10))
		//klog.Infoln("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		//klog.Infoln("resumableTotalSize", strconv.FormatInt(size, 10))
		//klog.Infoln("resumableType", mimeType)
		//klog.Infoln("resumableFilename", filename)
		//klog.Infoln("resumableRelativePath", filename)
		//klog.Infoln("resumableTotalChunks", strconv.FormatInt(totalChunks, 10), "\n")

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

		part, err := writer.CreateFormFile("file", common.EscapeAndJoin(filename, "/"))
		if err != nil {
			klog.Errorln("Create Form File error: ", err)
			return http.StatusInternalServerError, err
		}

		_, err = part.Write(chunkData[:bytesRead])
		if err != nil {
			klog.Errorln("Write Chunk Data error: ", err)
			return http.StatusInternalServerError, err
		}

		err = writer.Close()
		if err != nil {
			klog.Errorln("Write Close error: ", err)
			return http.StatusInternalServerError, err
		}

		request, err := http.NewRequest("POST", targetURL, body)
		if err != nil {
			klog.Errorln("New Request error: ", err)
			return http.StatusInternalServerError, err
		}

		request.Header = r.Header
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Content-Disposition", "attachment; filename=\""+common.EscapeAndJoin(filename, "/")+"\"")
		request.Header.Set("Content-Range", "bytes "+strconv.FormatInt(chunkStart, 10)+"-"+strconv.FormatInt(chunkStart+int64(bytesRead)-1, 10)+"/"+strconv.FormatInt(size, 10))
		chunkStart += int64(bytesRead)

		client := http.Client{}
		response, err := client.Do(request)
		klog.Infoln("Do Request")
		if err != nil {
			klog.Errorln("Do Request error: ", err)
			return http.StatusInternalServerError, err
		}
		defer response.Body.Close()

		// Read the response body as a string
		postBody, err := io.ReadAll(response.Body)
		klog.Infoln("ReadAll")
		if err != nil {
			klog.Errorln("ReadAll error: ", err)
			return common.ErrToStatus(err), err
		}

		klog.Infoln("Status Code: ", response.StatusCode)
		if response.StatusCode != http.StatusOK {
			klog.Infoln(string(postBody))
			return response.StatusCode, fmt.Errorf("file upload failed, status code: %d", response.StatusCode)
		}
	}
	klog.Infoln("sync buffer to file success!")
	return http.StatusOK, nil
}

func ResourceSyncDelete(path string, r *http.Request) (int, error) {
	path = strings.Trim(path, "/")
	if !strings.Contains(path, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
		return common.ErrToStatus(err), err
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

func SyncPermToMode(permStr string) os.FileMode {
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
		klog.Infoln("invalid permission string")
		return 0
	}

	return perm
}

func ResourceSyncPatch(action, src, dst string, r *http.Request) error {
	var apiName string
	switch action {
	case "copy":
		apiName = "sync-batch-copy-item"
	case "rename":
		apiName = "sync-batch-move-item"
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}

	// It seems that we can't mkdir althrough when using sync-bacth-copy/move-item, so we must use false for isDir here.
	if err := SyncMkdirAll(dst, 0, false, r); err != nil {
		return err
	}

	src = strings.Trim(src, "/")
	if !strings.Contains(src, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
		return err
	}

	srcFirstSlashIdx := strings.Index(src, "/")

	srcRepoID := src[:srcFirstSlashIdx]

	srcLastSlashIdx := strings.LastIndex(src, "/")

	srcFilename := src[srcLastSlashIdx+1:]

	srcPrefix := ""
	if srcFirstSlashIdx != srcLastSlashIdx {
		srcPrefix = src[srcFirstSlashIdx+1 : srcLastSlashIdx+1]
	}

	if srcPrefix != "" {
		srcPrefix = "/" + srcPrefix
	} else {
		srcPrefix = "/"
	}

	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
		return err
	}

	dstFirstSlashIdx := strings.Index(dst, "/")

	dstRepoID := dst[:dstFirstSlashIdx]

	dstLastSlashIdx := strings.LastIndex(dst, "/")

	dstPrefix := ""
	if dstFirstSlashIdx != dstLastSlashIdx {
		dstPrefix = dst[dstFirstSlashIdx+1 : dstLastSlashIdx+1]
	}

	if dstPrefix != "" {
		dstPrefix = "/" + dstPrefix
	} else {
		dstPrefix = "/"
	}

	targetURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + apiName + "/"
	requestBody := map[string]interface{}{
		"dst_parent_dir": dstPrefix,
		"dst_repo_id":    dstRepoID,
		"src_dirents":    []string{srcFilename},
		"src_parent_dir": srcPrefix,
		"src_repo_id":    srcRepoID,
	}
	klog.Infoln(requestBody)
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	request.Header = r.Header
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// Read the response body as a string
	postBody, err := io.ReadAll(response.Body)
	klog.Infoln("ReadAll")
	if err != nil {
		klog.Errorln("ReadAll error: ", err)
		return err
	}

	if response.StatusCode != http.StatusOK {
		klog.Infoln(string(postBody))
		return fmt.Errorf("file paste failed with status: %d", response.StatusCode)
	}

	return nil
}
