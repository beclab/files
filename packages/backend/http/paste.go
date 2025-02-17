package http

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/json"
	e "errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/spf13/afero"
)

func ioCopyFileWithBuffer(sourcePath, targetPath string, bufferSize int) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	dir := filepath.Dir(targetPath)
	baseName := filepath.Base(targetPath)

	tempFileName := fmt.Sprintf(".uploading_%s", baseName)
	tempFilePath := filepath.Join(dir, tempFileName)

	targetFile, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	buf := make([]byte, bufferSize)
	for {
		n, err := sourceFile.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if _, err := targetFile.Write(buf[:n]); err != nil {
			return err
		}
	}

	if err := targetFile.Sync(); err != nil {
		return err
	}
	return os.Rename(tempFilePath, targetPath)
}

//func ioCopyFile(sourcePath, targetPath string) error {
//	sourceFile, err := os.Open(sourcePath)
//	if err != nil {
//		return err
//	}
//	defer sourceFile.Close()
//
//	// 使用 os.OpenFile 以确保文件被创建且可写
//	targetFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
//	if err != nil {
//		return err
//	}
//	defer targetFile.Close()
//
//	_, err = io.Copy(targetFile, sourceFile)
//	if err != nil {
//		return err
//	}
//
//	err = targetFile.Sync()
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

//func ioCopyFile(sourcePath, targetPath string) error {
//	sourceFile, err := os.Open(sourcePath)
//	if err != nil {
//		return err
//	}
//	defer sourceFile.Close()
//
//	targetFile, err := os.Create(targetPath)
//	if err != nil {
//		return err
//	}
//	defer targetFile.Close()
//
//	_, err = io.Copy(targetFile, sourceFile)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

func resourceDriveGetInfo(path string, r *http.Request, d *data) (*files.FileInfo, int, error) {
	xBflUser := r.Header.Get("X-Bfl-User")
	fmt.Println("X-Bfl-User: ", xBflUser)

	d.user, _ = d.store.Users.Get(d.server.Root, uint(1))
	//fmt.Println(d.user.Fs)
	//fmt.Println(path)
	//fmt.Println(d.user.Perm.Modify)
	//fmt.Println(d.server.TypeDetectionByHeader)
	//fmt.Println(d)
	fmt.Println("d.user.Username: ", d.user.Username)
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         d.user.Fs,
		Path:       path, //r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     true,
		ReadHeader: d.server.TypeDetectionByHeader,
		Checker:    d,
		Content:    true,
	})
	//fmt.Println(file)
	if err != nil {
		return file, errToStatus(err), err
	}

	if file.IsDir {
		//fmt.Println(file)
		//file.Size = file.Listing.Size
		// recursiveSize(file)
		file.Listing.Sorting = d.user.Sorting
		file.Listing.ApplySort()
		return file, http.StatusOK, nil //renderJSON(w, r, file)
	}

	//if checksum := r.URL.Query().Get("checksum"); checksum != "" {
	//	err := file.Checksum(checksum)
	//	if err == errors.ErrInvalidOption {
	//		return file, http.StatusBadRequest, nil
	//	} else if err != nil {
	//		return file, http.StatusInternalServerError, err
	//	}
	//
	//	// do not waste bandwidth if we just want the checksum
	//	file.Content = ""
	//}

	if file.Type == "video" {
		osSystemServer := "system-server.user-system-" + xBflUser
		//osSystemServer := v.Get("OS_SYSTEM_SERVER")
		//if osSystemServer == nil {
		//	log.Println("need env OS_SYSTEM_SERVER")
		//}

		httpposturl := fmt.Sprintf("http://%s/legacy/v1alpha1/api.intent/v1/server/intent/send", osSystemServer) // os.Getenv("OS_SYSTEM_SERVER"))

		//fmt.Println("HTTP JSON POST URL:", httpposturl)

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

		//fmt.Println("response Status:", response.Status)
		//fmt.Println("response Headers:", response.Header)
		body, _ := ioutil.ReadAll(response.Body)
		fmt.Println("response Body:", string(body))
	}

	return file, http.StatusOK, nil //renderJSON(w, r, file)
}

func generateBufferFileName(originalFilePath, bflName string) (string, error) {
	// 获取当前时间戳
	timestamp := time.Now().Unix()

	// 获取原始文件名的扩展名
	extension := filepath.Ext(originalFilePath)

	// 去掉原始文件名的扩展名
	originalFileName := strings.TrimSuffix(filepath.Base(originalFilePath), extension)

	// 构建新的文件名
	bufferFileName := fmt.Sprintf("%d_%s.bin", timestamp, originalFileName)
	bufferFolderPath := "/data/" + bflName + "/buffer"
	err := os.MkdirAll(bufferFolderPath, 0755)
	if err != nil {
		return "", err
	}
	bufferFilePath := filepath.Join(bufferFolderPath, bufferFileName)

	return bufferFilePath, nil
}

func makeDiskBuffer(filePath string, bufferSize int64) error {
	//filePath := "buffer.bin"
	//bufferSize := int64(1024 * 1024) // 1 MB

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Failed to create buffer file:", err)
		return err
	}
	defer file.Close()

	if err = file.Truncate(bufferSize); err != nil {
		fmt.Println("Failed to truncate buffer file:", err)
		return err
	}

	// 输出缓冲区文件的大小
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Failed to get buffer file info:", err)
		return err
	}
	fmt.Println("Buffer file size:", fileInfo.Size(), "bytes")
	return nil
}

func removeDiskBuffer(filePath string) {
	//bufferFilePath := "buffer.bin"

	err := os.Remove(filePath)
	if err != nil {
		fmt.Println("Failed to delete buffer file:", err)
		return
	}

	fmt.Println("Buffer file deleted.")
}

func driveFileToBuffer(file *files.FileInfo, bufferFilePath string) error {
	path, err := unescapeURLIfEscaped(file.Path)
	if err != nil {
		return err
	}
	fd, err := file.Fs.Open(path) // file.Path)
	if err != nil {
		return err
	}
	defer fd.Close()

	//bufferFilePath := "buffer.bin"
	bufferFile, err := os.OpenFile(bufferFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer bufferFile.Close()

	_, err = io.Copy(bufferFile, fd)
	if err != nil {
		return err
	}

	return nil
}

func driveBufferToFile(bufferFilePath string, targetPath string, mode os.FileMode, d *data) (int, error) {
	//d.user, _ = d.store.Users.Get(d.server.Root, uint(1))
	var err error
	targetPath, err = unescapeURLIfEscaped(targetPath) // url.QueryUnescape(targetPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if !d.user.Perm.Create || !d.Check(targetPath) {
		return http.StatusForbidden, nil
	}

	// Directories creation on POST.
	if strings.HasSuffix(targetPath, "/") {
		err := d.user.Fs.MkdirAll(targetPath, mode) // 0775) //nolint:gomnd
		return errToStatus(err), err
	}

	_, err = files.NewFileInfo(files.FileOptions{
		Fs:         d.user.Fs,
		Path:       targetPath,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: d.server.TypeDetectionByHeader,
		Checker:    d,
	})
	if err == nil {
		//if override != "true" {
		//	return http.StatusConflict, nil
		//}

		// Permission for overwriting the file
		if !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}

		//err = delThumbs(r.Context(), fileCache, file)
		//if err != nil {
		//	return errToStatus(err), err
		//}
	}

	//fmt.Println("Going to write file!")
	err = d.RunHook(func() error {
		//fmt.Println("Opening ", bufferFilePath)
		//bufferFile, err := os.Open(bufferFilePath)
		//if err != nil {
		//	return err
		//}
		//defer bufferFile.Close()
		//fmt.Println("Opened ", bufferFilePath)

		//_, writeErr := writeFile(d.user.Fs, targetPath, bufferFile)
		//if writeErr != nil {
		//	fmt.Println(writeErr)
		//	return writeErr
		//}
		err := ioCopyFileWithBuffer(bufferFilePath, "/data"+targetPath, 8*1024*1024)
		if err != nil {
			fmt.Println(err)
			return err
		}
		//fmt.Println("Writer File done!")

		//etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
		//w.Header().Set("ETag", etag)
		return nil
	}, "upload", targetPath, "", d.user)

	if err != nil {
		_ = d.user.Fs.RemoveAll(targetPath)
	}

	return errToStatus(err), err
}

func resourceDriveDelete(fileCache FileCache, path string, ctx context.Context, d *data) (int, error) {
	//d.user, _ = d.store.Users.Get(d.server.Root, uint(1))
	//fmt.Println("deleting", path)
	if path == "/" || !d.user.Perm.Delete {
		return http.StatusForbidden, nil
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         d.user.Fs,
		Path:       path,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: d.server.TypeDetectionByHeader,
		Checker:    d,
	})
	if err != nil {
		return errToStatus(err), err
	}

	// delete thumbnails
	err = delThumbs(ctx, fileCache, file)
	if err != nil {
		return errToStatus(err), err
	}

	err = d.RunHook(func() error {
		return d.user.Fs.RemoveAll(path)
	}, "delete", path, "", d.user)

	if err != nil {
		return errToStatus(err), err
	}

	return http.StatusOK, nil
}

func cacheMkdirAll(dst string, mode os.FileMode, r *http.Request) error {
	targetURL := "http://127.0.0.1:80/api/resources" + escapeURLWithSpace(dst) + "/?mode=" + mode.String() //strconv.FormatUint(uint64(mode), 10)
	//fmt.Println(targetURL)

	request, err := http.NewRequest("POST", targetURL, nil)
	if err != nil {
		return err
	}

	request.Header = r.Header
	request.Header.Set("Content-Type", "application/octet-stream")

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		//fmt.Println(response.StatusCode)
		return fmt.Errorf("file upload failed with status: %d", response.StatusCode)
	}

	return nil
}

func cacheFileToBuffer(src string, bufferFilePath string) error {
	//fd, err := file.Fs.Open(file.Path)
	//if err != nil {
	//	return err
	//}
	//defer fd.Close()

	newSrc := strings.Replace(src, "AppData/", "appcache/", 1)
	newPath, err := unescapeURLIfEscaped(newSrc)
	if err != nil {
		return err
	}
	//fmt.Println(newSrc)
	fd, err := os.Open(newPath) // newSrc)
	if err != nil {
		return err
	}
	defer fd.Close()

	//bufferFilePath := "buffer.bin"
	bufferFile, err := os.OpenFile(bufferFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer bufferFile.Close()

	_, err = io.Copy(bufferFile, fd)
	if err != nil {
		return err
	}

	return nil
}

//func cacheFileToBuffer(src string, bufferFilePath string, r *http.Request) error {
//	url := "http://127.0.0.1:80/api/raw" + src
//
//	request, err := http.NewRequest("GET", url, nil)
//	if err != nil {
//		return err
//	}
//
//	request.Header = r.Header
//
//	client := http.Client{}
//	response, err := client.Do(request)
//	if err != nil {
//		return err
//	}
//	defer response.Body.Close()
//
//	if response.StatusCode != http.StatusOK {
//		return fmt.Errorf("request failed，status code：%d", response.StatusCode)
//	}
//
//	contentDisposition := response.Header.Get("Content-Disposition")
//	if contentDisposition == "" {
//		return fmt.Errorf("unrecognizable response format")
//	}
//
//	_, params, err := mime.ParseMediaType(contentDisposition)
//	if err != nil {
//		return err
//	}
//	filename := params["filename"]
//	fmt.Println("download filename: ", filename)
//
//	bufferFile, err := os.OpenFile(bufferFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
//	if err != nil {
//		return err
//	}
//	defer bufferFile.Close()
//
//	if response.Header.Get("Content-Encoding") == "gzip" {
//		gzipReader, err := gzip.NewReader(response.Body)
//		if err != nil {
//			return err
//		}
//		defer gzipReader.Close()
//
//		_, err = io.Copy(bufferFile, gzipReader)
//		if err != nil {
//			return err
//		}
//	} else {
//		bodyBytes, err := ioutil.ReadAll(response.Body)
//		if err != nil {
//			return err
//		}
//
//		_, err = io.Copy(bufferFile, bytes.NewReader(bodyBytes))
//		if err != nil {
//			return err
//		}
//	}
//
//	//fmt.Println("status code:", response.StatusCode)
//
//	//fmt.Println("response headers:")
//	//for key, values := range response.Header {
//	//	for _, value := range values {
//	//		fmt.Printf("%s: %s\n", key, value)
//	//	}
//	//}
//
//	return nil
//}

func cacheBufferToFile(bufferFilePath string, targetPath string, mode os.FileMode, d *data) (int, error) {
	//d.user, _ = d.store.Users.Get(d.server.Root, uint(1))
	if !d.user.Perm.Create || !d.Check(targetPath) {
		return http.StatusForbidden, nil
	}

	// Directories creation on POST.
	if strings.HasSuffix(targetPath, "/") {
		err := d.user.Fs.MkdirAll(targetPath, mode) // 0775) //nolint:gomnd
		return errToStatus(err), err
	}

	_, err := files.NewFileInfo(files.FileOptions{
		Fs:         d.user.Fs,
		Path:       targetPath,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: d.server.TypeDetectionByHeader,
		Checker:    d,
	})
	if err == nil {
		// Permission for overwriting the file
		if !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}
	}

	newTargetPath := strings.Replace(targetPath, "AppData/", "appcache/", 1)
	//fmt.Println(newTargetPath)
	//fmt.Println("Going to write file!")
	err = d.RunHook(func() error {
		err := ioCopyFileWithBuffer(bufferFilePath, newTargetPath, 8*1024*1024)
		if err != nil {
			fmt.Println(err)
			return err
		}
		return nil
	}, "upload", targetPath, "", d.user)

	if err != nil {
		//_ = d.user.Fs.RemoveAll(targetPath)
		err = os.RemoveAll(newTargetPath)
		if err == nil {
			fmt.Println("Rollback Failed:", err)
		}
		fmt.Println("Rollback success")
	}

	return errToStatus(err), err
}

//func cacheBufferToFile(bufferFilePath string, dst string, mode os.FileMode, r *http.Request) (int, error) {
//	targetURL := "http://127.0.0.1:80/api/resources" + dst + "?mode=" + mode.String() // strconv.FormatUint(uint64(mode), 10)
//	//fmt.Println(targetURL)
//	bufferFile, err := os.Open(bufferFilePath)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//	defer bufferFile.Close()
//
//	request, err := http.NewRequest("POST", targetURL, bufferFile)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//
//	request.Header = r.Header
//	request.Header.Set("Content-Type", "application/octet-stream")
//
//	client := http.Client{}
//	response, err := client.Do(request)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//	defer response.Body.Close()
//
//	if response.StatusCode != http.StatusOK {
//		//fmt.Println(response.StatusCode)
//		return response.StatusCode, fmt.Errorf("file upload failed with status: %d", response.StatusCode)
//	}
//
//	return http.StatusOK, nil
//}

func resourceCacheDelete(fileCache FileCache, path string, ctx context.Context, d *data) (int, error) {
	//d.user, _ = d.store.Users.Get(d.server.Root, uint(1))
	//fmt.Println("deleting", path)
	if path == "/" || !d.user.Perm.Delete {
		return http.StatusForbidden, nil
	}

	// No thumbnails in cache for the time being
	//file, err := files.NewFileInfo(files.FileOptions{
	//	Fs:         d.user.Fs,
	//	Path:       path,
	//	Modify:     d.user.Perm.Modify,
	//	Expand:     false,
	//	ReadHeader: d.server.TypeDetectionByHeader,
	//	Checker:    d,
	//})
	//if err != nil {
	//	return errToStatus(err), err
	//}

	// delete thumbnails
	//err = delThumbs(ctx, fileCache, file)
	//if err != nil {
	//	return errToStatus(err), err
	//}

	err := d.RunHook(func() error {
		newTargetPath := strings.Replace(path, "AppData/", "appcache/", 1)
		//fmt.Println(newTargetPath)
		return os.RemoveAll(newTargetPath)
		//return d.user.Fs.RemoveAll(path)
	}, "delete", path, "", d.user)

	if err != nil {
		return errToStatus(err), err
	}

	return http.StatusOK, nil
}

//func resourceCacheDelete(path string, r *http.Request) (int, error) {
//	targetURL := "http://127.0.0.1:80/api/resources" + path
//
//	client := http.Client{
//		Timeout: time.Second * 10,
//	}
//
//	request, err := http.NewRequest("DELETE", targetURL, nil)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//
//	request.Header = r.Header
//
//	response, err := client.Do(request)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//	defer response.Body.Close()
//
//	if response.StatusCode != http.StatusOK {
//		return response.StatusCode, fmt.Errorf("file delete failed with status: %d", response.StatusCode)
//	}
//
//	return http.StatusOK, nil
//}

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

	//filename := ""
	prefix := ""
	if isDir {
		prefix = dst[firstSlashIdx+1:]

	} else {
		//filename = dst[lastSlashIdx+1:]

		if firstSlashIdx != lastSlashIdx {
			prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
		}
	}

	//fmt.Println("repo-id:", repoID)
	//fmt.Println("prefix:", prefix)
	//fmt.Println("filename:", filename)

	//infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=/"
	//fmt.Println(infoURL)

	client := &http.Client{}

	// Split the prefix by '/' and generate the URLs
	prefixParts := strings.Split(prefix, "/")
	for i := 0; i < len(prefixParts); i++ {
		curPrefix := strings.Join(prefixParts[:i+1], "/")
		//curInfoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + url.QueryEscape("/"+curPrefix) + "&with_thumbnail=true"
		curInfoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace("/"+curPrefix) + "&with_thumbnail=true"
		//fmt.Println("!!! Try to mkdir through: ", curInfoURL)
		getRequest, err := http.NewRequest("GET", curInfoURL, nil)
		if err != nil {
			fmt.Printf("create request failed: %v\n", err)
			return err
		}
		getRequest.Header = r.Header
		//fmt.Println("request headers:")
		//for key, values := range getRequest.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}
		getResponse, err := client.Do(getRequest)
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			return err
		}
		defer getResponse.Body.Close()
		if getResponse.StatusCode == 200 {
			//fmt.Println(curPrefix, " already exist! Don't need to create!")
			continue
		} else {
			fmt.Println(getResponse.Status)
		}

		type CreateDirRequest struct {
			Operation string `json:"operation"`
		}

		//curCreateURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + url.QueryEscape("/"+curPrefix)
		curCreateURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace("/"+curPrefix)
		//fmt.Println(curCreateURL)

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

		//fmt.Println("request headers:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			return err
		}
		defer response.Body.Close()

		// Handle the response as needed
		//fmt.Println("Response status:", response.Status)
		if response.StatusCode != 200 && response.StatusCode != 201 {
			err = e.New("mkdir failed")
			return err
		}
	}
	return nil
}

func syncFileToBuffer(src string, bufferFilePath string, r *http.Request) error {
	// src is like [repo-id]/path/filename
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

	//fmt.Println("repo-id:", repoID)
	//fmt.Println("prefix:", prefix)
	//fmt.Println("filename:", filename)

	dlUrl := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file/" + escapeURLWithSpace(prefix+filename) + "/" + "?dl=1"
	//fmt.Println(url)

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
	//fmt.Println("download filename: ", filename)

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

	//fmt.Println("statud code:", response.StatusCode)

	//fmt.Println("response header:")
	//for key, values := range response.Header {
	//	for _, value := range values {
	//		fmt.Printf("%s: %s\n", key, value)
	//	}
	//}

	return nil
}

func generateUniqueIdentifier(relativePath string) string {
	// 计算 MD5 哈希
	h := md5.New()
	io.WriteString(h, relativePath+time.Now().String())
	return fmt.Sprintf("%x%s", h.Sum(nil), relativePath)
}

func getFileExtension(filename string) string {
	extension := path.Ext(filename)
	//if extension == "" {
	//	extension = "blob"
	//} else {
	//	extension = extension[1:]
	//}
	return extension
}

func syncBufferToFile(bufferFilePath string, dst string, size int64, r *http.Request) (int, error) {
	// Step1: deal with URL
	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return errToStatus(err), err
	}

	firstSlashIdx := strings.Index(dst, "/")

	repoID := dst[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(dst, "/")

	filename := dst[lastSlashIdx+1:]
	//filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
	}

	//fmt.Println("repo-id:", repoID)
	//fmt.Println("prefix:", prefix)
	//fmt.Println("filename:", filename)

	extension := getFileExtension(filename)
	//fmt.Println("extension:", extension)
	mimeType := "application/octet-stream"
	if extension != "" {
		mimeType = mime.TypeByExtension(extension)
	}
	//fmt.Println("MIME Type:", mimeType)

	// step2: GET upload URL
	//getUrl := "http://seafile/api2/repos/" + repoID + "/upload-link/?p=/" + prefix //+ "&from=web"
	//getUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/upload-link/?p=" + url.QueryEscape("/"+prefix) + "&from=api"
	getUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/upload-link/?p=" + escapeURLWithSpace("/"+prefix) + "&from=api"
	//fmt.Println(getUrl)

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

	// Now you can use the 'uploadLink' variable for further processing
	//fmt.Println("Upload link:", uploadLink)

	// step3: deal with upload URL
	//targetURL := "http://seafile:8082" + uploadLink[9:] + "?ret-json=1"
	targetURL := "http://127.0.0.1:80" + uploadLink + "?ret-json=1"
	//fmt.Println(targetURL)

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
	identifier := generateUniqueIdentifier(filename)

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

		//fmt.Println("Identifier: ", identifier)
		//fmt.Println("Parent Dir: ", "/"+prefix)
		//fmt.Println("resumableChunkNumber: ", strconv.FormatInt(chunkNumber, 10))
		//fmt.Println("resumableChunkSize: ", strconv.FormatInt(chunkSize, 10))
		//fmt.Println("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		//fmt.Println("resumableTotalSize", strconv.FormatInt(size, 10)) // "169")
		//fmt.Println("resumableType", mimeType)
		//fmt.Println("resumableFilename", filename)     // "response")
		//fmt.Println("resumableRelativePath", filename) // "response")
		//fmt.Println("resumableTotalChunks", strconv.FormatInt(totalChunks, 10), "\n")

		writer.WriteField("resumableChunkNumber", strconv.FormatInt(chunkNumber, 10))
		writer.WriteField("resumableChunkSize", strconv.FormatInt(chunkSize, 10))
		writer.WriteField("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		writer.WriteField("resumableTotalSize", strconv.FormatInt(size, 10)) // "169")
		writer.WriteField("resumableType", mimeType)
		writer.WriteField("resumableIdentifier", identifier) //"096b7d0f1af58ccf5bfb1dbde97fb51cresponse")
		writer.WriteField("resumableFilename", filename)     // "response")
		writer.WriteField("resumableRelativePath", filename) // "response")
		writer.WriteField("resumableTotalChunks", strconv.FormatInt(totalChunks, 10))
		writer.WriteField("parent_dir", "/"+prefix) //+"//")

		//content := body.String()
		//fmt.Println(content)

		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		_, err = part.Write(chunkData[:bytesRead])
		if err != nil {
			return http.StatusInternalServerError, err
		}

		err = writer.Close()
		if err != nil {
			return http.StatusInternalServerError, err
		}

		request, err := http.NewRequest("POST", targetURL, body)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		request.Header = r.Header
		request.Header.Set("Content-Type", writer.FormDataContentType())
		//Content-Disposition:attachment; filename="2022.zip"
		//Content-Length:5244224
		//Content-Range:bytes 10485760-15728639/61034314
		request.Header.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		//request.Header.Set("Content-Length", strconv.FormatInt(int64(bytesRead), 10))
		request.Header.Set("Content-Range", "bytes "+strconv.FormatInt(chunkStart, 10)+"-"+strconv.FormatInt(chunkStart+int64(bytesRead)-1, 10)+"/"+strconv.FormatInt(size, 10))
		chunkStart += int64(bytesRead)

		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		client := http.Client{}
		response, err := client.Do(request)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		defer response.Body.Close()

		// Read the response body as a string
		//postBody, err := io.ReadAll(response.Body)
		_, err = io.ReadAll(response.Body)
		if err != nil {
			return errToStatus(err), err
		}

		if response.StatusCode != http.StatusOK {
			//fmt.Println(string(postBody))
			return response.StatusCode, fmt.Errorf("file upload failed, status code: %d", response.StatusCode)
		}
	}
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
	//filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = path[firstSlashIdx+1 : lastSlashIdx+1]
	}

	if prefix != "" {
		prefix = "/" + prefix + "/"
	} else {
		prefix = "/"
	}

	//fmt.Println("repo-id:", repoID)
	//fmt.Println("prefix:", prefix)
	//fmt.Println("filename:", filename)

	targetURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/batch-delete-item/"
	requestBody := map[string]interface{}{
		"dirents":    []string{filename}, // 将 filename 放入数组中
		"parent_dir": prefix,
		"repo_id":    repoID,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	//fmt.Println(jsonBody)

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

func pasteAddVersionSuffix(source string, dstType string, fs afero.Fs, r *http.Request) string {
	counter := 1
	dir, name := path.Split(source)
	//ext := filepath.Ext(name)
	//base := strings.TrimSuffix(name, ext)
	ext := ""
	base := name

	for {
		//if _, err := fs.Stat(source); err != nil {
		var isDir bool
		var err error
		if _, _, _, isDir, err = getStat(fs, dstType, source, r); err != nil {
			break
		}
		if !isDir {
			ext = filepath.Ext(base)
			base = strings.TrimSuffix(name, ext)
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}

	return source
}

func testDriveLs(w http.ResponseWriter, r *http.Request) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	origin := r.Header.Get("Origin")
	dstUrl := origin + "/api/resources%2FHome%2FDocuments%2F"
	//fmt.Println("dstUrl:", dstUrl)

	req, err := http.NewRequest("GET", dstUrl, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return err
	}

	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")

	for name, values := range req.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	defer resp.Body.Close()

	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	http.Error(w, "Error reading response body: "+err.Error(), http.StatusInternalServerError)
	//	return err
	//}
	//
	//responseText := string(body)
	//
	//w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	//w.Write([]byte(responseText))

	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	http.Error(w, "Error reading response body: "+err.Error(), http.StatusInternalServerError)
	//	return err
	//}
	//

	//var jsonResponse map[string]interface{}
	//err = json.Unmarshal(body, &jsonResponse)
	//if err != nil {
	//	http.Error(w, "Error unmarshaling JSON response: "+err.Error(), http.StatusInternalServerError)
	//	return err
	//}
	//

	//responseText, err := json.MarshalIndent(jsonResponse, "", "  ")
	//if err != nil {
	//	http.Error(w, "Error marshaling JSON response to text: "+err.Error(), http.StatusInternalServerError)
	//	return err
	//}
	//

	//w.Header().Set("Content-Type", "application/json; charset=utf-8")
	//w.Write([]byte(responseText))

	//fmt.Printf("Response Hedears:\n")
	//for name, values := range resp.Header {
	//	for _, value := range values {
	//		fmt.Printf("%s: %s\n", name, value)
	//	}
	//}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		fmt.Println("Response is not JSON format:", contentType)
	}

	var body []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			return err
		}
		defer reader.Close()

		body, err = ioutil.ReadAll(reader)
		if err != nil {
			fmt.Println("Error reading gzipped response body:", err)
			return err
		}
	} else {
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return err
		}
	}

	var datas map[string]interface{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		fmt.Println("Error unmarshaling JSON response:", err)
		return err
	}

	//fmt.Println("Parsed JSON response:", datas)
	responseText, err := json.MarshalIndent(datas, "", "  ")
	if err != nil {
		http.Error(w, "Error marshaling JSON response to text: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(responseText))
	return nil
}

func testDriveLs2(w http.ResponseWriter, r *http.Request) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	// dstUrl := "http://files-service.user-space-" + bflName + ":8181/ls"
	origin := r.Header.Get("Origin")
	dstUrl := origin + "/drive/ls"
	//fmt.Println("dstUrl:", dstUrl)

	//payload := []byte(`{"path":"/","name":"wangrongxiang@bytetrade.io","drive":"google"}`)
	type RequestPayload struct {
		Path  string `json:"path"`
		Name  string `json:"name"`
		Drive string `json:"drive"`
	}
	payload := RequestPayload{
		Path:  "/",
		Name:  "wangrongxiang@bytetrade.io",
		Drive: "google",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return err
	}

	req, err := http.NewRequest("POST", dstUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return err
	}

	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")

	//for name, values := range req.Header {
	//	for _, value := range values {
	//		fmt.Printf("%s: %s\n", name, value)
	//	}
	//}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response body: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	_, err = w.Write(body)
	if err != nil {
		http.Error(w, "Error writing response body: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	fmt.Println("Error reading response body:", err)
	//	return err
	//}

	// Copy the response body directly to the http.ResponseWriter
	//_, err = io.Copy(w, resp.Body)
	//if err != nil {
	//	http.Error(w, "Error copying response body", http.StatusInternalServerError)
	//	return err
	//}

	//fmt.Println(string(body))
	// Write the response body to the http.ResponseWriter
	//w.Header().Set("Content-Type", "application/json")
	//w.Write(body)
	// Convert the response body to UTF-8 encoding
	//bodyString := string(body)
	//utf8Body := []byte(bodyString)
	//
	//// Set the Content-Type header to indicate JSON data
	//w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//w.Write(utf8Body)

	return nil
}

func isURLEscaped(s string) bool {
	escapePattern := `%[0-9A-Fa-f]{2}`
	re := regexp.MustCompile(escapePattern)

	if re.MatchString(s) {
		decodedStr, err := url.QueryUnescape(s)
		if err != nil {
			return false
		}
		return decodedStr != s
	}
	return false
}

func unescapeURLIfEscaped(s string) (string, error) {
	var result = s
	var err error
	if isURLEscaped(s) {
		result, err = url.QueryUnescape(s)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}

func escapeURLWithSpace(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func resourcePasteHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		// For this func, src starts with src type + /, dst start with dst type + /
		// type is only "drive", "sync" and "cache" for the time being
		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		srcType := r.URL.Query().Get("src_type")
		if srcType == "" {
			srcType = "drive"
		}
		dstType := r.URL.Query().Get("dst_type")
		if dstType == "" {
			dstType = "drive"
		}
		//fmt.Println(srcType, src, dstType, dst)
		if srcType != "drive" && srcType != "sync" && srcType != "cache" && srcType != "google" {
			fmt.Println("Src type is invalid!")
			return http.StatusForbidden, nil
		}
		if dstType != "drive" && dstType != "sync" && dstType != "cache" && srcType != "google" {
			fmt.Println("Dst type is invalid!")
			return http.StatusForbidden, nil
		}
		if srcType == dstType {
			fmt.Println("Src and dst are of same arch!")
		} else {
			fmt.Println("Src and dst are of different arches!")
		}
		if srcType == "google" || dstType == "google" {
			err := testDriveLs(w, r)
			return errToStatus(err), err
			//err = d.RunHook(func() error {
			//	return testDriveLs(w, r)
			//}, action, src, dst, d.user)
		}
		action := r.URL.Query().Get("action")
		var err error
		fmt.Println("src:", src)
		src, err = unescapeURLIfEscaped(src) // url.QueryUnescape(src)
		fmt.Println("src:", src, "err:", err)
		fmt.Println("dst:", dst)
		dst, err = unescapeURLIfEscaped(dst) // url.QueryUnescape(dst)
		fmt.Println("dst:", dst, "err:", err)
		if !d.Check(src) || !d.Check(dst) {
			return http.StatusForbidden, nil
		}
		if err != nil {
			return errToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}
		override := r.URL.Query().Get("override") == "true"
		rename := r.URL.Query().Get("rename") == "true"
		if !override && !rename {
			if _, err := d.user.Fs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		if rename {
			dst = pasteAddVersionSuffix(dst, dstType, d.user.Fs, r)
		}
		// Permission for overwriting the file
		if override && !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}
		if srcType == dstType {
			err = d.RunHook(func() error {
				return pasteActionSameArch(r.Context(), action, srcType, src, dstType, dst, d, fileCache, override, rename, r)
			}, action, src, dst, d.user)
		} else {
			err = d.RunHook(func() error {
				return pasteActionDiffArch(r.Context(), action, srcType, src, dstType, dst, d, fileCache, r)
			}, action, src, dst, d.user)
		}
		if errToStatus(err) == http.StatusRequestEntityTooLarge {
			fmt.Fprintln(w, err.Error())
		}
		return errToStatus(err), err
	})
}

func syncPermToMode(permStr string) os.FileMode {
	perm := os.FileMode(0)
	if permStr == "r" {
		//perm = perm | 0444
		perm = perm | 0555
	} else if permStr == "w" {
		//perm = perm | 0200
		perm = perm | 0311
	} else if permStr == "x" {
		perm = perm | 0111
	} else if permStr == "rw" {
		//perm = perm | 0644
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

	//fmt.Println("transferred permission:", perm)
	return perm
}

func syncModeToPermString(fileMode os.FileMode) string {
	permStr := ""

	//if fileMode&os.ModeDir != 0 {
	//	permStr += "d"
	//} else {
	//	permStr += "-"
	//}

	if fileMode&0400 != 0 {
		permStr += "r"
	}
	//} else {
	//	permStr += "-"
	//}
	if fileMode&0200 != 0 {
		permStr += "w"
	}
	//} else {
	//	permStr += "-"
	//}
	if fileMode&0100 != 0 {
		permStr += "x"
	}
	//} else {
	//	permStr += "-"
	//}

	//if fileMode&040 != 0 {
	//	permStr += "r"
	//} else {
	//	permStr += "-"
	//}
	//if fileMode&020 != 0 {
	//	permStr += "w"
	//} else {
	//	permStr += "-"
	//}
	//if fileMode&010 != 0 {
	//	permStr += "x"
	//} else {
	//	permStr += "-"
	//}

	//if fileMode&04 != 0 {
	//	permStr += "r"
	//} else {
	//	permStr += "-"
	//}
	//if fileMode&02 != 0 {
	//	permStr += "w"
	//} else {
	//	permStr += "-"
	//}
	//if fileMode&01 != 0 {
	//	permStr += "x"
	//} else {
	//	permStr += "-"
	//}

	return permStr
}

func getStat(fs afero.Fs, srcType, src string, r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	// we need only size, fileMode and isDir for the time being for all arch
	if srcType == "drive" {
		info, err := fs.Stat(src)
		if err != nil {
			return nil, 0, 0, false, err
		}
		return info, info.Size(), info.Mode(), info.IsDir(), nil
	} else if srcType == "cache" {
		//host := r.Host
		//infoUrl := "http://" + host + "/api/resources" + src
		infoURL := "http://127.0.0.1:80/api/resources" + escapeURLWithSpace(src)
		//fmt.Println(infoURL)

		client := &http.Client{}
		request, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			fmt.Printf("create request failed: %v\n", err)
			return nil, 0, 0, false, err
		}

		request.Header = r.Header
		//fmt.Println("request headers:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			return nil, 0, 0, false, err
		}
		defer response.Body.Close()

		//fmt.Println("response haeders:")
		//for key, values := range response.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		var bodyReader io.Reader = response.Body

		if response.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				fmt.Printf("unzip response failed: %v\n", err)
				return nil, 0, 0, false, err
			}
			defer gzipReader.Close()

			bodyReader = gzipReader
		}

		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			fmt.Printf("read response failed: %v\n", err)
			return nil, 0, 0, false, err
		}

		//fmt.Println("request URL:", infoURL)
		//fmt.Println("request headers:", request.Header)
		//fmt.Println("response status code:", response.StatusCode)
		//fmt.Println("response body:", string(body))

		var fileInfo struct {
			Size  int64       `json:"size"`
			Mode  os.FileMode `json:"mode"`
			IsDir bool        `json:"isDir"`
			Path  string      `json:"path"`
			Name  string      `json:"name"`
			Type  string      `json:"type"`
		}

		err = json.Unmarshal(body, &fileInfo)
		if err != nil {
			fmt.Printf("parse response failed: %v\n", err)
			return nil, 0, 0, false, err
		}

		//fmt.Println("Size:", fileInfo.Size)
		//fmt.Println("Mode:", fileInfo.Mode)
		//fmt.Println("IsDir:", fileInfo.IsDir)
		//fmt.Println("Path:", fileInfo.Path)
		//fmt.Println("Name:", fileInfo.Name)
		//fmt.Println("Type:", fileInfo.Type)

		return nil, fileInfo.Size, fileInfo.Mode, fileInfo.IsDir, nil
	} else if srcType == "sync" {
		// src is like [repo-id]/path/filename
		src = strings.Trim(src, "/")
		if !strings.Contains(src, "/") {
			err := e.New("invalid path format: path must contain at least one '/'")
			fmt.Println("Error:", err)
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

		//fmt.Println("repo-id:", repoID)
		//fmt.Println("prefix:", prefix)
		//fmt.Println("filename:", filename)

		//infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + url.QueryEscape("/"+prefix) + "&with_thumbnail=true"
		infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace("/"+prefix) + "&with_thumbnail=true"
		//fmt.Println(infoURL)

		client := &http.Client{}
		request, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			fmt.Printf("create request failed: %v\n", err)
			return nil, 0, 0, false, err
		}

		request.Header = r.Header
		//fmt.Println("request headers:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			return nil, 0, 0, false, err
		}
		defer response.Body.Close()

		//fmt.Println("response headers:")
		//for key, values := range response.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		var bodyReader io.Reader = response.Body

		if response.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				fmt.Printf("unzip response failed: %v\n", err)
				return nil, 0, 0, false, err
			}
			defer gzipReader.Close()

			bodyReader = gzipReader
		}

		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			fmt.Printf("read response failed: %v\n", err)
			return nil, 0, 0, false, err
		}

		//fmt.Println("request URL:", infoURL)
		//fmt.Println("request header:", request.Header)
		//fmt.Println("response status code:", response.StatusCode)
		//fmt.Println("response body:", string(body))

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
			fmt.Printf("parse response failed: %v\n", err)
			return nil, 0, 0, false, err
		}

		//fmt.Println("User Perm:", dirResp.UserPerm)
		//fmt.Println("Dir ID:", dirResp.DirID)
		//fmt.Println("Dirent List:")
		var found = false
		for _, dirent := range dirResp.DirentList {
			if dirent.Name == filename {
				fileInfo = dirent
				//fmt.Println("Type:", dirent.Type)
				//fmt.Println("ID:", dirent.ID)
				//fmt.Println("Name:", dirent.Name)
				//fmt.Println("Mtime:", dirent.Mtime)
				//fmt.Println("Permission:", dirent.Permission)
				//fmt.Println("Parent Dir:", dirent.ParentDir)
				//fmt.Println("Starred:", dirent.Starred)
				//fmt.Println("Size:", dirent.Size)
				//fmt.Println("FileSize:", dirent.FileSize)
				//fmt.Println("NumTotalFiles:", dirent.NumTotalFiles)
				//fmt.Println("NumFiles:", dirent.NumFiles)
				//fmt.Println("NumDirs:", dirent.NumDirs)
				//fmt.Println("Path:", dirent.Path)
				//fmt.Println("Modifier Email:", dirent.ModifierEmail)
				//fmt.Println("Modifier Name:", dirent.ModifierName)
				//fmt.Println("Modifier Contact Email:", dirent.ModifierContactEmail)
				//fmt.Println()
				found = true
				break
			}
		}
		if found {
			mode := syncPermToMode(fileInfo.Permission)
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
	// type is checked at the very entrance
	return nil, 0, 0, false, nil
}

// CopyDir copies a directory from source to dest and all
// of its sub-directories. It doesn't stop if it finds an error
// during the copy. Returns an error if any.
func copyDir(fs afero.Fs, srcType, src, dstType, dst string, d *data, fileMode os.FileMode, r *http.Request) error {
	var mode os.FileMode = 0
	// Get properties of source.
	if srcType == "drive" {
		srcinfo, err := fs.Stat(src)
		if err != nil {
			return err
		}
		mode = srcinfo.Mode()
	} else {
		mode = fileMode
	}

	// Create the destination directory.
	if dstType == "drive" {
		err := fs.MkdirAll(dst, mode)
		if err != nil {
			return err
		}
	} else if dstType == "cache" {
		err := cacheMkdirAll(dst, fileMode, r)
		if err != nil {
			return err
		}
	} else if dstType == "sync" {
		err := syncMkdirAll(dst, fileMode, true, r)
		if err != nil {
			return err
		}
	}

	if srcType == "drive" {
		dir, _ := fs.Open(src)
		obs, err := dir.Readdir(-1)
		if err != nil {
			return err
		}

		var errs []error

		for _, obj := range obs {
			fsrc := src + "/" + obj.Name()
			fdst := dst + "/" + obj.Name()

			//fmt.Println(fsrc, fdst)
			if obj.IsDir() {
				// Create sub-directories, recursively.
				err = copyDir(fs, srcType, fsrc, dstType, fdst, d, obj.Mode(), r)
				if err != nil {
					errs = append(errs, err)
				}
			} else {
				// Perform the file copy.
				err = copyFile(fs, srcType, fsrc, dstType, fdst, d, obj.Mode(), obj.Size(), r)
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
		var errString string
		for _, err := range errs {
			errString += err.Error() + "\n"
		}

		if errString != "" {
			return e.New(errString)
		}
	} else if srcType == "cache" {
		type Item struct {
			Path      string `json:"path"`
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			Extension string `json:"extension"`
			Modified  string `json:"modified"`
			Mode      uint32 `json:"mode"`
			IsDir     bool   `json:"isDir"`
			IsSymlink bool   `json:"isSymlink"`
			Type      string `json:"type"`
		}

		type ResponseData struct {
			Items    []Item `json:"items"`
			NumDirs  int    `json:"numDirs"`
			NumFiles int    `json:"numFiles"`
			Sorting  struct {
				By  string `json:"by"`
				Asc bool   `json:"asc"`
			} `json:"sorting"`
			Path      string `json:"path"`
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			Extension string `json:"extension"`
			Modified  string `json:"modified"`
			Mode      uint32 `json:"mode"`
			IsDir     bool   `json:"isDir"`
			IsSymlink bool   `json:"isSymlink"`
			Type      string `json:"type"`
		}

		infoURL := "http://127.0.0.1:80/api/resources" + escapeURLWithSpace(src)
		//fmt.Println(infoURL)

		client := &http.Client{}
		request, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			fmt.Printf("create request failed: %v\n", err)
			return err
		}

		request.Header = r.Header
		//fmt.Println("request headers:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			return err
		}
		defer response.Body.Close()

		//fmt.Println("response headers:")
		//for key, values := range response.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		var bodyReader io.Reader = response.Body

		if response.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				fmt.Printf("unzip response failed: %v\n", err)
				return err
			}
			defer gzipReader.Close()

			bodyReader = gzipReader
		}

		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			fmt.Printf("read response failed: %v\n", err)
			return err
		}

		//fmt.Println("request URL:", infoURL)
		//fmt.Println("request header:", request.Header)
		//fmt.Println("response status code:", response.StatusCode)
		//fmt.Println("response body:", string(body))

		var data ResponseData
		//err = json.NewDecoder(response.Body).Decode(&data)
		err = json.Unmarshal(body, &data)
		if err != nil {
			return err
		}

		for _, item := range data.Items {
			fsrc := filepath.Join(src, item.Name)
			fdst := filepath.Join(dst, item.Name)

			//fmt.Println(fsrc, fdst)
			if item.IsDir {
				err := copyDir(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), r)
				if err != nil {
					return err
				}
			} else {
				err := copyFile(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), item.Size, r)
				if err != nil {
					return err
				}
			}
		}
		return nil
	} else if srcType == "sync" {
		//fmt.Println("Sync copy/move dir!")
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

		//fmt.Println("repo-id:", repoID)
		//fmt.Println("prefix:", prefix)
		//fmt.Println("filename:", filename)

		//infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + url.QueryEscape("/"+prefix+"/"+filename) + "&with_thumbnail=true"
		infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace("/"+prefix+"/"+filename) + "&with_thumbnail=true"
		//fmt.Println(infoURL)

		client := &http.Client{}
		request, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			fmt.Printf("create request failed: %v\n", err)
			return err
		}

		request.Header = r.Header
		//fmt.Println("request headers:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			return err
		}
		defer response.Body.Close()

		var bodyReader io.Reader = response.Body

		if response.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				fmt.Printf("unzip response failed: %v\n", err)
				return err
			}
			defer gzipReader.Close()

			bodyReader = gzipReader
		}

		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			fmt.Printf("read response failed: %v\n", err)
			return err
		}

		//fmt.Println("request URL:", infoURL)
		//fmt.Println("request header:", request.Header)
		//fmt.Println("response status code:", response.StatusCode)
		//fmt.Println("response body:", string(body))

		var data ResponseData
		//err = json.NewDecoder(response.Body).Decode(&data)
		err = json.Unmarshal(body, &data)
		if err != nil {
			return err
		}

		for _, item := range data.DirentList {
			fsrc := filepath.Join(src, item.Name)
			fdst := filepath.Join(dst, item.Name)

			//fmt.Println(fsrc, fdst)
			if item.Type == "dir" {
				err := copyDir(fs, srcType, fsrc, dstType, fdst, d, syncPermToMode(item.Permission), r)
				if err != nil {
					return err
				}
			} else {
				err := copyFile(fs, srcType, fsrc, dstType, fdst, d, syncPermToMode(item.Permission), item.Size, r)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	return nil
}

// CopyFile copies a file from source to dest and returns
// an error if any.
func copyFile(fs afero.Fs, srcType, src, dstType, dst string, d *data, mode os.FileMode, diskSize int64, r *http.Request) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	var bufferPath string
	// copy/move
	if srcType == "drive" {
		fileInfo, status, err := resourceDriveGetInfo(src, r, d)
		//fmt.Println(fileInfo, status, err)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		diskSize = fileInfo.Size
		_, err = checkBufferDiskSpace(diskSize)
		if err != nil {
			return err
		}
		//fmt.Println("Buffer Disk Space check OK, will reserve disk size: ", diskSize)
		// Won't deal a file which is bigger than 4G for the time being
		//if diskSize >= 4*1024*1024*1024 {
		//	fmt.Println("file size exceeds 4GB")
		//	return e.New("file size exceeds 4GB") //os.ErrPermission
		//}
		//fmt.Println("Will reserve disk size: ", diskSize)
		bufferPath, err = generateBufferFileName(src, bflName)
		if err != nil {
			return err
		}
		//fmt.Println("Buffer file path: ", bufferPath)

		err = makeDiskBuffer(bufferPath, diskSize)
		if err != nil {
			return err
		}
		err = driveFileToBuffer(fileInfo, bufferPath)
		if err != nil {
			return err
		}
	} else if srcType == "cache" {
		var err error
		_, err = checkBufferDiskSpace(diskSize)
		if err != nil {
			return err
		}
		//fmt.Println("Buffer Disk Space check OK, will reserve disk size: ", diskSize)
		//if diskSize >= 4*1024*1024*1024 {
		//	fmt.Println("file size exceeds 4GB")
		//	return e.New("file size exceeds 4GB") //os.ErrPermission
		//}
		//fmt.Println("Will reserve disk size: ", diskSize)
		bufferPath, err = generateBufferFileName(src, bflName)
		if err != nil {
			return err
		}
		//fmt.Println("Buffer file path: ", bufferPath)

		err = makeDiskBuffer(bufferPath, diskSize)
		if err != nil {
			return err
		}
		err = cacheFileToBuffer(src, bufferPath)
		if err != nil {
			return err
		}
	} else if srcType == "sync" {
		var err error
		_, err = checkBufferDiskSpace(diskSize)
		if err != nil {
			return err
		}
		//fmt.Println("Buffer Disk Space check OK, will reserve disk size: ", diskSize)
		//if diskSize >= 4*1024*1024*1024 {
		//	fmt.Println("file size exceeds 4GB")
		//	return e.New("file size exceeds 4GB") //os.ErrPermission
		//}
		//fmt.Println("Will reserve disk size: ", diskSize)
		bufferPath, err = generateBufferFileName(src, bflName)
		if err != nil {
			return err
		}
		//fmt.Println("Buffer file path: ", bufferPath)

		err = makeDiskBuffer(bufferPath, diskSize)
		if err != nil {
			return err
		}
		err = syncFileToBuffer(src, bufferPath, r)
		if err != nil {
			return err
		}
	}

	// paste
	if dstType == "drive" {
		//fmt.Println("Begin to paste!")
		status, err := driveBufferToFile(bufferPath, dst, mode, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		//fmt.Println("Begin to remove buffer")
		removeDiskBuffer(bufferPath)
	} else if dstType == "cache" {
		//fmt.Println("Begin to cache paste!")
		status, err := cacheBufferToFile(bufferPath, dst, mode, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		//fmt.Println("Begin to remove buffer")
		removeDiskBuffer(bufferPath)
	} else if dstType == "sync" {
		//fmt.Println("Begin to sync paste!")
		err := syncMkdirAll(dst, mode, false, r)
		if err != nil {
			return err
		}
		status, err := syncBufferToFile(bufferPath, dst, diskSize, r)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		//fmt.Println("Begin to remove buffer")
		removeDiskBuffer(bufferPath)
	}
	return nil
}

func doPaste(fs afero.Fs, srcType, src, dstType, dst string, d *data, r *http.Request) error {
	// path.Clean, only operate on string level, so it fits every src/dst type.
	if src = path.Clean("/" + src); src == "" {
		return os.ErrNotExist
	}

	if dst = path.Clean("/" + dst); dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the virtual root directory.
		return os.ErrInvalid
	}

	// Only when URL and type are both the same, it is not OK.
	if (dst == src) && (dstType == srcType) {
		return os.ErrInvalid
	}

	//info, err := fs.Stat(src)
	//if err != nil {
	//	return err
	//}
	_, size, mode, isDir, err := getStat(fs, srcType, src, r)
	if err != nil {
		return err
	}

	if isDir {
		err = copyDir(fs, srcType, src, dstType, dst, d, mode, r)
	} else {
		err = copyFile(fs, srcType, src, dstType, dst, d, mode, size, r)
	}
	if err != nil {
		return err
	}
	return nil
}

func moveDelete(fileCache FileCache, srcType, src string, ctx context.Context, d *data, r *http.Request) error {
	if srcType == "drive" {
		status, err := resourceDriveDelete(fileCache, src, ctx, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	} else if srcType == "cache" {
		status, err := resourceCacheDelete(fileCache, src, ctx, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	} else if srcType == "sync" {
		status, err := resourceSyncDelete(src, r)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	}
	return os.ErrInvalid
}

func pasteActionSameArch(ctx context.Context, action, srcType, src, dstType, dst string, d *data, fileCache FileCache, override, rename bool, r *http.Request) error {
	fmt.Println("Now deal with ", action, " for same arch ", dstType)
	fmt.Println("src: ", src, ", dst: ", dst, ", override: ", override)
	if srcType == "drive" || srcType == "cache" {
		patchUrl := "http://127.0.0.1:80/api/resources/" + escapeURLWithSpace(strings.TrimLeft(src, "/")) + "?action=" + action + "&destination=" + escapeURLWithSpace(dst) + "&override=" + strconv.FormatBool(override) + "&rename=" + strconv.FormatBool(rename)
		method := "PATCH"
		payload := []byte(``)
		fmt.Println(patchUrl)

		client := &http.Client{}
		req, err := http.NewRequest(method, patchUrl, bytes.NewBuffer(payload))
		if err != nil {
			return err
		}

		req.Header = r.Header

		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		//respBody, err := ioutil.ReadAll(res.Body)
		_, err = ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		//fmt.Println(respBody)
		return nil
	} else if srcType == "sync" {
		var apiName string
		switch action {
		case "copy":
			apiName = "sync-batch-copy-item"
		case "rename":
			apiName = "sync-batch-move-item"
		default:
			return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
		}

		//_, _, mode, isDir, err := getStat(d.user.Fs, srcType, src, r)
		//if err != nil {
		//	return err
		//}
		//err = syncMkdirAll(dst, mode, isDir, r)

		// It seems that we can't mkdir althrough when using sync-bacth-copy/move-item, so we must use false for isDir here.
		err := syncMkdirAll(dst, 0, false, r)
		if err != nil {
			return err
		}

		src = strings.Trim(src, "/")
		if !strings.Contains(src, "/") {
			err := e.New("invalid path format: path must contain at least one '/'")
			fmt.Println("Error:", err)
			return err
		}

		srcFirstSlashIdx := strings.Index(src, "/")

		srcRepoID := src[:srcFirstSlashIdx]

		srcLastSlashIdx := strings.LastIndex(src, "/")

		srcFilename := src[srcLastSlashIdx+1:]
		//filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]

		srcPrefix := ""
		if srcFirstSlashIdx != srcLastSlashIdx {
			srcPrefix = src[srcFirstSlashIdx+1 : srcLastSlashIdx+1]
		}

		if srcPrefix != "" {
			srcPrefix = "/" + srcPrefix
		} else {
			srcPrefix = "/"
		}

		//fmt.Println("src repo-id:", srcRepoID)
		//fmt.Println("src prefix:", srcPrefix)
		//fmt.Println("src filename:", srcFilename)

		dst = strings.Trim(dst, "/")
		if !strings.Contains(dst, "/") {
			err := e.New("invalid path format: path must contain at least one '/'")
			fmt.Println("Error:", err)
			return err
		}

		dstFirstSlashIdx := strings.Index(dst, "/")

		dstRepoID := dst[:dstFirstSlashIdx]

		dstLastSlashIdx := strings.LastIndex(dst, "/")

		//dstFilename := dst[dstLastSlashIdx+1:]
		//filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]

		dstPrefix := ""
		if dstFirstSlashIdx != dstLastSlashIdx {
			dstPrefix = dst[dstFirstSlashIdx+1 : dstLastSlashIdx+1]
		}

		if dstPrefix != "" {
			dstPrefix = "/" + dstPrefix
		} else {
			dstPrefix = "/"
		}

		//fmt.Println("dst repo-id:", dstRepoID)
		//fmt.Println("dst prefix:", dstPrefix)
		//fmt.Println("dst filename:", dstFilename)

		targetURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + apiName + "/"
		requestBody := map[string]interface{}{
			"dst_parent_dir": dstPrefix,
			"dst_repo_id":    dstRepoID,
			"src_dirents":    []string{srcFilename},
			"src_parent_dir": srcPrefix,
			"src_repo_id":    srcRepoID,
		}
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		//fmt.Println(jsonBody)

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

		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("file delete failed with status: %d", response.StatusCode)
		}

		return nil
	}
	return nil
}

func pasteActionDiffArch(ctx context.Context, action, srcType, src, dstType, dst string, d *data, fileCache FileCache, r *http.Request) error {
	// In this function, context if tied up to src, because src is in the URL
	switch action {
	// TODO: use enum
	case "copy":
		if !d.user.Perm.Create {
			return errors.ErrPermissionDenied
		}

		return doPaste(d.user.Fs, srcType, src, dstType, dst, d, r)
	case "rename":
		if !d.user.Perm.Rename {
			return errors.ErrPermissionDenied
		}
		err := doPaste(d.user.Fs, srcType, src, dstType, dst, d, r)
		if err != nil {
			return err
		}

		err = moveDelete(fileCache, srcType, src, ctx, d, r)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
	return nil
}
