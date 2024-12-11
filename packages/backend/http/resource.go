package http

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/my_redis"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/fileutils"
)

// func recursiveSize(file *files.FileInfo) {
// 	if file.IsDir {
// 		for _, info := range file.Items {
// 			//fmt.Println(info)
// 			recursiveSize(info)
// 		}
// 		if file.Listing != nil {
// 			file.Size += file.Listing.Size
// 			file.FileSize += file.Listing.FileSize
// 		}
// 	}
// 	return
// }

func resourceGetSync(w http.ResponseWriter, r *http.Request, stream int) (int, error) {
	// src is like [repo-id]/path/filename
	src := r.URL.Path
	src, err := url.QueryUnescape(src)
	if err != nil {
		return http.StatusBadRequest, err
	}
	fmt.Println("src Path:", src)
	src = strings.Trim(src, "/") + "/"
	//if !strings.Contains(src, "/") {
	//	err := e.New("invalid path format: path must contain at least one '/'")
	//	fmt.Println("Error:", err)
	//	return errToStatus(err), err
	//}

	firstSlashIdx := strings.Index(src, "/")

	repoID := src[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(src, "/")

	// don't use, because this is only used for folders
	filename := src[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}
	if prefix == "" {
		prefix = "/"
	}
	prefix = url.QueryEscape(prefix)

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
		//body, _ := ioutil.ReadAll(response.Body)
		streamSyncDirents(w, r, body, repoID)
		return 0, nil
	}

	// non-SSE
	_, err = io.Copy(w, response.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return errToStatus(err), err
	}

	return 0, nil
}

var resourceGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	start := time.Now()
	fmt.Println("Function resourceGetHandler starts at", start)

	streamStr := r.URL.Query().Get("stream")
	stream := 0

	var err error
	if streamStr != "" {
		stream, err = strconv.Atoi(streamStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}
	fmt.Println("stream: ", stream)

	metaStr := r.URL.Query().Get("meta")
	meta := 0
	if metaStr != "" {
		meta, err = strconv.Atoi(metaStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}
	fmt.Println("meta: ", meta)

	srcType := r.URL.Query().Get("src")
	if srcType == "sync" {
		return resourceGetSync(w, r, stream)
	} else if srcType == "google" {
		return resourceGetGoogle(w, r, stream, meta)
	} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		return resourceGetAwss3(w, r, stream, meta)
	}

	xBflUser := r.Header.Get("X-Bfl-User")
	fmt.Println("X-Bfl-GoogleDriveListResponseUser: ", xBflUser)

	var usbData []files.DiskInfo = nil
	var hddData []files.DiskInfo = nil
	if files.TerminusdHost != "" {
		urls := []string{
			"http://" + files.TerminusdHost + "/system/mounted-usb-incluster",
			"http://" + files.TerminusdHost + "/system/mounted-hdd-incluster",
		}

		headers := r.Header.Clone()
		headers.Set("Content-Type", "application/json")
		headers.Set("X-Signature", "temp_signature")
		//headers := map[string]string{
		//	"X-Signature": "temp_signature",
		//}

		for _, url := range urls {
			data, err := files.FetchDiskInfo(url, headers)
			if err != nil {
				log.Printf("Failed to fetch data from %s: %v", url, err)
				continue
			}

			if url == urls[0] {
				usbData = data
			} else if url == urls[1] {
				hddData = data
			}
		}

		fmt.Println("USB Awss3MetaResponseData:", usbData)
		fmt.Println("HDD Awss3MetaResponseData:", hddData)
	}

	var file *files.FileInfo
	//var err error
	if usbData != nil || hddData != nil {
		file, err = files.NewFileInfoWithDiskInfo(files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     true,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
			Content:    true,
		}, usbData, hddData)
	} else {
		file, err = files.NewFileInfo(files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     true,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
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
		// fmt.Println(file)
		// file.Size = file.Listing.Size
		// recursiveSize(file)
		if files.CheckPath(file.Path, files.ExternalPrefix, "/") {
			file.ExternalType = files.GetExternalType(file.Path, usbData, hddData)
		}
		file.Listing.Sorting = d.user.Sorting
		file.Listing.ApplySort()
		if stream == 1 {
			//return streamJSON(w, r, file)
			streamListingItems(w, r, file.Listing, d, usbData, hddData)
			elapsed := time.Since(start)
			fmt.Printf("Function resourceGetHandler execution time: %v\n", elapsed)
			return 0, nil
		} else {
			elapsed := time.Since(start)
			fmt.Printf("Function resourceGetHandler execution time: %v\n", elapsed)
			return renderJSON(w, r, file)
		}
		//return renderJSON(w, r, file)
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
		//osSystemServer := v.Get("OS_SYSTEM_SERVER")
		//if osSystemServer == nil {
		//	log.Println("need env OS_SYSTEM_SERVER")
		//}

		/*
				showLogUrl := fmt.Sprintf("http://%s/legacy/v1alpha1/api.intent/v1/server/intent/send", os.Getenv("OS_SYSTEM_SERVER"))
			listTobeSend := "{\"action\": \"view\",\"category\": \"container_log\",\"data\": {\"statefulset\": \"geth\",\"container\": \"geth\"}}"
			showLogRequestOption := &HttpCallRequestOption{
				// Header:  map[string]string{"X-Access-Token": accessToken, "type":"application/json"},
				Header:  map[string]string{"type":"application/json"},
				Url:     showLogUrl,
				Timeout: 1000 * 10,
				JsonStr: listTobeSend,
				TestOKFun: func(bodyStr string) bool {
					log.Println("bodyis:", bodyStr)
					code := gjson.Get(bodyStr, "code").Int()
					return code == 0
				},
			}
			showLogBody, listOk, showLogErr := NewHttpCall().PostJsonCall(showLogRequestOption)
			if showLogErr != nil {
				err = showLogErr
				return
			}
			if !listOk {
				err = errors.WithMessage(GetNoEmptyError(err), "show log error")
				return
			}
			message = showLogBody
		*/

		httpposturl := fmt.Sprintf("http://%s/legacy/v1alpha1/api.intent/v1/server/intent/send", osSystemServer) // os.Getenv("OS_SYSTEM_SERVER"))

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
	elapsed := time.Since(start)
	fmt.Printf("Function resourceGetHandler execution time: %v\n", elapsed)
	return renderJSON(w, r, file)
})

func resourceDeleteHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.URL.Path == "/" || !d.user.Perm.Delete {
			return http.StatusForbidden, nil
		}

		srcType := r.URL.Query().Get("src")
		if srcType == "google" {
			_, status, err := resourceDeleteGoogle(fileCache, "", w, r, false)
			return status, err
		} else if srcType == "awss3" {
			_, status, err := resourceDeleteAwss3(fileCache, "", w, r, false)
			return status, err
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err != nil {
			return errToStatus(err), err
		}

		// delete thumbnails
		err = delThumbs(r.Context(), fileCache, file)
		if err != nil {
			return errToStatus(err), err
		}

		err = d.RunHook(func() error {
			return d.user.Fs.RemoveAll(r.URL.Path)
		}, "delete", r.URL.Path, "", d.user)

		if err != nil {
			return errToStatus(err), err
		}

		return http.StatusOK, nil
	})
}

func resourceUnmountHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.URL.Path == "/" || !d.user.Perm.Delete {
			return http.StatusForbidden, nil
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err != nil {
			return errToStatus(err), err
		}

		// delete thumbnails
		//err = delThumbs(r.Context(), fileCache, file)
		//if err != nil {
		//	return errToStatus(err), err
		//}

		//err = d.RunHook(func() error {
		//	return d.user.Fs.RemoveAll(r.URL.Path)
		//}, "delete", r.URL.Path, "", d.user)

		respJson, err := files.UnmountUSBIncluster(r, file.Path)
		if err != nil {
			return errToStatus(err), err
		}

		return renderJSON(w, r, respJson)
		//return http.StatusOK, nil
	})
}

func resourcePostHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Create || !d.Check(r.URL.Path) {
			return http.StatusForbidden, nil
		}

		srcType := r.URL.Query().Get("src")
		if srcType == "google" {
			_, status, err := resourcePostGoogle("", w, r, false)
			return status, err
		} else if srcType == "awss3" {
			_, status, err := resourcePostAwss3("", w, r, false)
			return status, err
		}

		modeParam := r.URL.Query().Get("mode")

		mode, err := strconv.ParseUint(modeParam, 8, 32)
		if err != nil || modeParam == "" {
			mode = 0775
		}

		fileMode := os.FileMode(mode)

		// Directories creation on POST.
		if strings.HasSuffix(r.URL.Path, "/") {
			err := d.user.Fs.MkdirAll(r.URL.Path, fileMode) // 0775) //nolint:gomnd
			return errToStatus(err), err
		}

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err == nil {
			if r.URL.Query().Get("override") != "true" {
				return http.StatusConflict, nil
			}

			// Permission for overwriting the file
			if !d.user.Perm.Modify {
				return http.StatusForbidden, nil
			}

			err = delThumbs(r.Context(), fileCache, file)
			if err != nil {
				return errToStatus(err), err
			}
		}

		err = d.RunHook(func() error {
			info, writeErr := writeFile(d.user.Fs, r.URL.Path, r.Body)
			if writeErr != nil {
				return writeErr
			}

			etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
			w.Header().Set("ETag", etag)
			return nil
		}, "upload", r.URL.Path, "", d.user)

		if err != nil {
			_ = d.user.Fs.RemoveAll(r.URL.Path)
		}

		return errToStatus(err), err
	})
}

var resourcePutHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Modify || !d.Check(r.URL.Path) {
		return http.StatusForbidden, nil
	}

	// Only allow PUT for files.
	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	exists, err := afero.Exists(d.user.Fs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	err = d.RunHook(func() error {
		info, writeErr := writeFile(d.user.Fs, r.URL.Path, r.Body)
		if writeErr != nil {
			return writeErr
		}

		etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
		w.Header().Set("ETag", etag)
		return nil
	}, "save", r.URL.Path, "", d.user)

	return errToStatus(err), err
})

func resourcePatchHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		srcType := r.URL.Query().Get("src")
		if srcType == "google" {
			return resourcePatchGoogle(fileCache, w, r)
		} else if srcType == "awss3" {
			return resourcePatchAwss3(fileCache, w, r)
		}

		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		action := r.URL.Query().Get("action")
		dst, err := url.QueryUnescape(dst)

		if !d.Check(src) || !d.Check(dst) {
			return http.StatusForbidden, nil
		}
		if err != nil {
			return errToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		err = checkParent(src, dst)
		if err != nil {
			return http.StatusBadRequest, err
		}

		override := r.URL.Query().Get("override") == "true"
		rename := r.URL.Query().Get("rename") == "true"
		if !override && !rename {
			if _, err = d.user.Fs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		if rename {
			dst = addVersionSuffix(dst, d.user.Fs)
		}

		// Permission for overwriting the file
		if override && !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}

		fmt.Println("Before patch action:", src, dst, action, override, rename)
		err = d.RunHook(func() error {
			return patchAction(r.Context(), action, src, dst, d, fileCache)
		}, action, src, dst, d.user)

		return errToStatus(err), err
	})
}

func checkParent(src, dst string) error {
	rel, err := filepath.Rel(src, dst)
	if err != nil {
		return err
	}

	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") && rel != ".." && rel != "." {
		return errors.ErrSourceIsParent
	}

	return nil
}

func addVersionSuffix(source string, fs afero.Fs) string {
	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for {
		if _, err := fs.Stat(source); err != nil {
			break
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}

	return source
}

func writeFile(fs afero.Fs, dst string, in io.Reader) (os.FileInfo, error) {
	fmt.Println("Before open ", dst)
	dir, _ := path.Split(dst)
	err := fs.MkdirAll(dir, 0775) //nolint:gomnd
	if err != nil {
		return nil, err
	}

	fmt.Println("Open ", dst)
	file, err := fs.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775) //nolint:gomnd
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fmt.Println("Copy file!")
	_, err = io.Copy(file, in)
	if err != nil {
		return nil, err
	}

	fmt.Println("Get stat")
	// Gets the info about the file.
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fmt.Println(info)
	return info, nil
}

func delThumbs(ctx context.Context, fileCache FileCache, file *files.FileInfo) error {
	for _, previewSizeName := range PreviewSizeNames() {
		size, _ := ParsePreviewSize(previewSizeName)
		cacheKey := previewCacheKey(file, size)
		if err := fileCache.Delete(ctx, cacheKey); err != nil {
			return err
		}
		err := my_redis.DelThumbRedisKey(my_redis.GetFileName(cacheKey))
		if err != nil {
			return err
		}
	}

	return nil
}

func patchAction(ctx context.Context, action, src, dst string, d *data, fileCache FileCache) error {
	switch action {
	// TODO: use enum
	case "copy":
		if !d.user.Perm.Create {
			return errors.ErrPermissionDenied
		}

		return fileutils.Copy(d.user.Fs, src, dst)
	case "rename":
		if !d.user.Perm.Rename {
			return errors.ErrPermissionDenied
		}
		src = path.Clean("/" + src)
		dst = path.Clean("/" + dst)

		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         d.user.Fs,
			Path:       src,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: false,
			Checker:    d,
		})
		if err != nil {
			return err
		}

		// delete thumbnails
		err = delThumbs(ctx, fileCache, file)
		if err != nil {
			return err
		}

		return fileutils.MoveFile(d.user.Fs, src, dst)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
}

type DiskUsageResponse struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

var diskUsage = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         d.user.Fs,
		Path:       r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: false,
		Checker:    d,
		Content:    false,
	})
	if err != nil {
		return errToStatus(err), err
	}
	fPath := file.RealPath()
	if !file.IsDir {
		return renderJSON(w, r, &DiskUsageResponse{
			Total: 0,
			Used:  0,
		})
	}

	usage, err := disk.UsageWithContext(r.Context(), fPath)
	if err != nil {
		return errToStatus(err), err
	}
	return renderJSON(w, r, &DiskUsageResponse{
		Total: usage.Total,
		Used:  usage.Used,
	})
})
