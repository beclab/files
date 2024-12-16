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
	"strconv"
	"strings"
	"time"

	"github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/spf13/afero"
)

func ioCopyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

func resourceDriveGetInfo(path string, r *http.Request, d *data) (*files.FileInfo, int, error) {
	xBflUser := r.Header.Get("X-Bfl-User")
	fmt.Println("X-Bfl-GoogleDriveListResponseUser: ", xBflUser)

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

func generateBufferFileName(originalFilePath, bflName string, extRemains bool) (string, error) {
	// 获取当前时间戳
	timestamp := time.Now().Unix()

	// 获取原始文件名的扩展名
	extension := filepath.Ext(originalFilePath)

	// 去掉原始文件名的扩展名
	originalFileName := strings.TrimSuffix(filepath.Base(originalFilePath), extension)

	// 构建新的文件名
	var bufferFileName string
	var bufferFolderPath string
	if extRemains {
		bufferFileName = originalFileName + extension
		bufferFolderPath = "/data/" + bflName + "/buffer/" + fmt.Sprintf("%d", timestamp)
	} else {
		bufferFileName = fmt.Sprintf("%d_%s.bin", timestamp, originalFileName)
		bufferFolderPath = "/data/" + bflName + "/buffer"
	}

	err := os.MkdirAll(bufferFolderPath, 0755)
	if err != nil {
		return "", err
	}
	bufferFilePath := filepath.Join(bufferFolderPath, bufferFileName)

	return bufferFilePath, nil
}

func generateBufferGoogleFileName(originalFilePath, bflName string) (string, error) {
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

func generateBufferFolder(originalFilePath, bflName string) (string, error) {
	// 获取当前时间戳
	timestamp := time.Now().Unix()

	// 获得原始文件夹路径
	originalPathName := filepath.Base(strings.TrimSuffix(originalFilePath, "/"))
	extension := filepath.Ext(originalPathName)
	if len(extension) > 0 {
		originalPathName = strings.TrimSuffix(originalPathName, extension) + "_" + extension[1:]
	}
	originalPathName = url.QueryEscape(originalPathName)

	// 构建新的文件名
	bufferPathName := fmt.Sprintf("%d_%s", timestamp, originalPathName) // as parent folder
	//bufferPathName := fmt.Sprintf("%d", timestamp)
	if len(bufferPathName) > 30 {
		bufferPathName = bufferPathName[:30]
	}
	bufferFolderPath := "/data/" + bflName + "/buffer" + "/" + bufferPathName
	err := os.MkdirAll(bufferFolderPath, 0755)
	if err != nil {
		return "", err
	}
	return bufferFolderPath, nil
}

func makeDiskBuffer(filePath string, bufferSize int64, delete bool) error {
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

	if delete {
		err = os.Remove(filePath)
		if err != nil {
			fmt.Printf("Error removing test buffer: %v\n", err)
			return err
		}

		fmt.Println("Test buffer removed successfully")
	}
	return nil
}

func removeDiskBuffer(filePath string, srcType string) {
	//bufferFilePath := "buffer.bin"

	err := os.Remove(filePath)
	if err != nil {
		fmt.Println("Failed to delete buffer file:", err)
		return
	}
	if srcType == "google" || srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		dir := filepath.Dir(filePath)
		err = os.Remove(dir)
		if err != nil {
			fmt.Println("Failed to delete buffer file dir:", err)
			return
		}
	}

	fmt.Println("Buffer file deleted.")
}

func driveFileToBuffer(file *files.FileInfo, bufferFilePath string) error {
	fd, err := file.Fs.Open(file.Path)
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
	targetPath, err = url.QueryUnescape(targetPath)
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
		err := ioCopyFile(bufferFilePath, "/data"+targetPath)
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
	targetURL := "http://127.0.0.1:80/api/resources" + dst + "/?mode=" + mode.String() //strconv.FormatUint(uint64(mode), 10)
	//fmt.Println(targetURL)

	// 创建一个 POST 请求
	request, err := http.NewRequest("POST", targetURL, nil)
	if err != nil {
		return err
	}

	// 设置请求的 Content-Type
	request.Header = r.Header
	request.Header.Set("Content-Type", "application/octet-stream")

	// 发送请求
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// 检查响应状态码
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
	fmt.Println(newSrc)
	fd, err := os.Open(newSrc)
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
//	// 设置请求头
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
//		return fmt.Errorf("请求失败，状态码：%d", response.StatusCode)
//	}
//
//	contentDisposition := response.Header.Get("Content-Disposition")
//	if contentDisposition == "" {
//		return fmt.Errorf("无法识别的响应格式")
//	}
//
//	// 从Content-Disposition头中获取文件名
//	_, params, err := mime.ParseMediaType(contentDisposition)
//	if err != nil {
//		return err
//	}
//	filename := params["filename"]
//	fmt.Println("下载的文件名: ", filename)
//
//	bufferFile, err := os.OpenFile(bufferFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
//	if err != nil {
//		return err
//	}
//	defer bufferFile.Close()
//
//	// 检查Content-Encoding是否为gzip
//	if response.Header.Get("Content-Encoding") == "gzip" {
//		// 创建gzip Reader
//		gzipReader, err := gzip.NewReader(response.Body)
//		if err != nil {
//			return err
//		}
//		defer gzipReader.Close()
//
//		// 将解压缩后的响应体写入文件
//		_, err = io.Copy(bufferFile, gzipReader)
//		if err != nil {
//			return err
//		}
//	} else {
//		// 读取整个响应体
//		bodyBytes, err := ioutil.ReadAll(response.Body)
//		if err != nil {
//			return err
//		}
//
//		// 将响应体写入文件
//		_, err = io.Copy(bufferFile, bytes.NewReader(bodyBytes))
//		if err != nil {
//			return err
//		}
//	}
//
//	// 打印状态码
//	//fmt.Println("状态码:", response.StatusCode)
//
//	// 打印响应头
//	//fmt.Println("响应头:")
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
	fmt.Println(newTargetPath)
	//fmt.Println("Going to write file!")
	err = d.RunHook(func() error {
		err := ioCopyFile(bufferFilePath, newTargetPath)
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
//	// 创建一个 POST 请求
//	request, err := http.NewRequest("POST", targetURL, bufferFile)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//
//	// 设置请求的 Content-Type
//	request.Header = r.Header
//	request.Header.Set("Content-Type", "application/octet-stream")
//
//	// 发送请求
//	client := http.Client{}
//	response, err := client.Do(request)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//	defer response.Body.Close()
//
//	// 检查响应状态码
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
		fmt.Println(newTargetPath)
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
//	// 创建一个带超时的 HTTP 客户端
//	client := http.Client{
//		Timeout: time.Second * 10,
//	}
//
//	// 创建 DELETE 请求
//	request, err := http.NewRequest("DELETE", targetURL, nil)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//
//	// 设置请求头，仅包含必要的信息
//	request.Header = r.Header
//
//	// 发送请求
//	response, err := client.Do(request)
//	if err != nil {
//		return http.StatusInternalServerError, err
//	}
//	defer response.Body.Close()
//
//	// 检查响应状态码
//	if response.StatusCode != http.StatusOK {
//		return response.StatusCode, fmt.Errorf("file delete failed with status: %d", response.StatusCode)
//	}
//
//	return http.StatusOK, nil
//}

func syncMkdirAll(dst string, mode os.FileMode, isDir bool, r *http.Request) error {
	// 去除路径开头和结尾的斜杠
	dst = strings.Trim(dst, "/")
	// 检查路径中是否包含斜杠
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return err
	}

	// 获取第一个斜杠的索引
	firstSlashIdx := strings.Index(dst, "/")

	// 获取repo-id
	repoID := dst[:firstSlashIdx]

	// 获取最后一个斜杠的索引
	lastSlashIdx := strings.LastIndex(dst, "/")

	filename := ""
	prefix := ""
	if isDir {
		prefix = dst[firstSlashIdx+1:]

	} else {
		// 获取filename
		filename = dst[lastSlashIdx+1:]

		// 获取prefix
		if firstSlashIdx != lastSlashIdx {
			prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
		}
	}

	fmt.Println("repo-id:", repoID)
	fmt.Println("prefix:", prefix)
	fmt.Println("filename:", filename)

	//infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=/"
	//fmt.Println(infoURL)

	client := &http.Client{}

	// Split the prefix by '/' and generate the URLs
	prefixParts := strings.Split(prefix, "/")
	for i := 0; i < len(prefixParts); i++ {
		curPrefix := strings.Join(prefixParts[:i+1], "/")
		curInfoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + url.QueryEscape("/"+curPrefix) + "&with_thumbnail=true"
		fmt.Println("!!! Try to mkdir through: ", curInfoURL)
		getRequest, err := http.NewRequest("GET", curInfoURL, nil)
		if err != nil {
			fmt.Printf("创建请求失败: %v\n", err)
			return err
		}
		getRequest.Header = r.Header
		//fmt.Println("请求头:")
		//for key, values := range getRequest.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}
		getResponse, err := client.Do(getRequest)
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
			return err
		}
		defer getResponse.Body.Close()
		if getResponse.StatusCode == 200 {
			fmt.Println(curPrefix, " already exist! Don't need to create!")
			continue
		} else {
			fmt.Println(getResponse.Status)
		}

		type CreateDirRequest struct {
			Operation string `json:"operation"`
		}

		curCreateURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + url.QueryEscape("/"+curPrefix)
		fmt.Println(curCreateURL)

		// 创建请求体
		createDirReq := CreateDirRequest{
			Operation: "mkdir",
		}
		jsonBody, err := json.Marshal(createDirReq)
		if err != nil {
			fmt.Printf("序列化请求体失败: %v\n", err)
			return err
		}

		request, err := http.NewRequest("POST", curCreateURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			fmt.Printf("创建请求失败: %v\n", err)
			return err
		}

		// 设置请求头
		request.Header = r.Header
		request.Header.Set("Content-Type", "application/json")

		//fmt.Println("请求头:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
			return err
		}
		defer response.Body.Close()

		// Handle the response as needed
		fmt.Println("GoogleDriveListResponse status:", response.Status)
		if response.StatusCode != 200 && response.StatusCode != 201 {
			err = e.New("mkdir failed")
			return err
		}
	}
	return nil
}

func syncFileToBuffer(src string, bufferFilePath string, r *http.Request) error {
	// src is like [repo-id]/path/filename
	// 去除路径开头和结尾的斜杠
	src = strings.Trim(src, "/")
	// 检查路径中是否包含斜杠
	if !strings.Contains(src, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return err
	}

	// 获取第一个斜杠的索引
	firstSlashIdx := strings.Index(src, "/")

	// 获取repo-id
	repoID := src[:firstSlashIdx]

	// 获取最后一个斜杠的索引
	lastSlashIdx := strings.LastIndex(src, "/")

	// 获取filename
	filename := src[lastSlashIdx+1:]

	// 获取prefix
	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}

	fmt.Println("repo-id:", repoID)
	fmt.Println("prefix:", prefix)
	fmt.Println("filename:", filename)

	url := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file/" + prefix + filename + "/" + "?dl=1"
	fmt.Println(url)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// 设置请求头
	request.Header = r.Header

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("请求失败，状态码：%d", response.StatusCode)
	}

	contentDisposition := response.Header.Get("Content-Disposition")
	if contentDisposition == "" {
		return fmt.Errorf("无法识别的响应格式")
	}

	// 从Content-Disposition头中获取文件名
	_, params, err := mime.ParseMediaType(contentDisposition)
	if err != nil {
		return err
	}
	filename = params["filename"]
	fmt.Println("下载的文件名: ", filename)

	bufferFile, err := os.OpenFile(bufferFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer bufferFile.Close()

	// 检查Content-Encoding是否为gzip
	if response.Header.Get("Content-Encoding") == "gzip" {
		// 创建gzip Reader
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()

		// 将解压缩后的响应体写入文件
		_, err = io.Copy(bufferFile, gzipReader)
		if err != nil {
			return err
		}
	} else {
		// 读取整个响应体
		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}

		// 将响应体写入文件
		_, err = io.Copy(bufferFile, bytes.NewReader(bodyBytes))
		if err != nil {
			return err
		}
	}

	// 打印状态码
	//fmt.Println("状态码:", response.StatusCode)

	// 打印响应头
	//fmt.Println("响应头:")
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
	// 去除路径开头和结尾的斜杠
	dst = strings.Trim(dst, "/")
	// 检查路径中是否包含斜杠
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return errToStatus(err), err
	}

	// 获取第一个斜杠的索引
	firstSlashIdx := strings.Index(dst, "/")

	// 获取repo-id
	repoID := dst[:firstSlashIdx]

	// 获取最后一个斜杠的索引
	lastSlashIdx := strings.LastIndex(dst, "/")

	// 获取filename
	filename := dst[lastSlashIdx+1:]
	//filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]

	// 获取prefix
	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
	}

	fmt.Println("repo-id:", repoID)
	fmt.Println("prefix:", prefix)
	fmt.Println("filename:", filename)

	extension := getFileExtension(filename)
	fmt.Println("extension:", extension)
	mimeType := "application/octet-stream"
	if extension != "" {
		mimeType = mime.TypeByExtension(extension)
	}
	fmt.Println("MIME Type:", mimeType)

	// step2: GET upload URL
	//getUrl := "http://seafile/api2/repos/" + repoID + "/upload-link/?p=/" + prefix //+ "&from=web"
	getUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/upload-link/?p=" + url.QueryEscape("/"+prefix) + "&from=api"
	fmt.Println(getUrl)

	getRequest, err := http.NewRequest("GET", getUrl, nil)
	if err != nil {
		return errToStatus(err), err
	}

	// 设置请求头
	getRequest.Header = r.Header

	getClient := http.Client{}
	getResponse, err := getClient.Do(getRequest)
	if err != nil {
		return errToStatus(err), err
	}
	defer getResponse.Body.Close()

	if getResponse.StatusCode != http.StatusOK {
		err = fmt.Errorf("请求失败，状态码：%d", getResponse.StatusCode)
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
	fmt.Println("Upload link:", uploadLink)

	// step3: deal with upload URL
	//targetURL := "http://seafile:8082" + uploadLink[9:] + "?ret-json=1"
	targetURL := "http://127.0.0.1:80" + uploadLink + "?ret-json=1"
	fmt.Println(targetURL)

	// 打开要上传的文件
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
		// 读取当前分片的数据
		offset := (chunkNumber - 1) * chunkSize
		chunkData := make([]byte, chunkSize)
		bytesRead, err := bufferFile.ReadAt(chunkData, offset)
		if err != nil && err != io.EOF {
			return http.StatusInternalServerError, err
		}

		// 创建一个新的多部分表单数据请求
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// 添加表单字段
		fmt.Println("Identifier: ", identifier)
		fmt.Println("Parent Dir: ", "/"+prefix)
		fmt.Println("resumableChunkNumber: ", strconv.FormatInt(chunkNumber, 10))
		fmt.Println("resumableChunkSize: ", strconv.FormatInt(chunkSize, 10))
		fmt.Println("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		fmt.Println("resumableTotalSize", strconv.FormatInt(size, 10)) // "169")
		fmt.Println("resumableType", mimeType)
		fmt.Println("resumableFilename", filename)     // "response")
		fmt.Println("resumableRelativePath", filename) // "response")
		fmt.Println("resumableTotalChunks", strconv.FormatInt(totalChunks, 10), "\n")

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

		// 将缓冲区内容作为字符串输出
		content := body.String()
		fmt.Println(content)

		// 将文件分片添加到表单
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		_, err = part.Write(chunkData[:bytesRead])
		if err != nil {
			return http.StatusInternalServerError, err
		}

		// 关闭表单写入器
		err = writer.Close()
		if err != nil {
			return http.StatusInternalServerError, err
		}

		// 创建 HTTP 请求
		request, err := http.NewRequest("POST", targetURL, body)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		// 设置请求头
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

		// 发送请求
		client := http.Client{}
		response, err := client.Do(request)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		defer response.Body.Close()

		// Read the response body as a string
		postBody, err := io.ReadAll(response.Body)
		if err != nil {
			return errToStatus(err), err
		}

		// 检查响应状态码
		if response.StatusCode != http.StatusOK {
			fmt.Println(string(postBody))
			return response.StatusCode, fmt.Errorf("文件上传失败,状态码: %d", response.StatusCode)
		}
	}
	return http.StatusOK, nil
}

func resourceSyncDelete(path string, r *http.Request) (int, error) {
	// 去除路径开头和结尾的斜杠
	path = strings.Trim(path, "/")
	// 检查路径中是否包含斜杠
	if !strings.Contains(path, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return errToStatus(err), err
	}

	// 获取第一个斜杠的索引
	firstSlashIdx := strings.Index(path, "/")

	// 获取repo-id
	repoID := path[:firstSlashIdx]

	// 获取最后一个斜杠的索引
	lastSlashIdx := strings.LastIndex(path, "/")

	// 获取filename
	filename := path[lastSlashIdx+1:]
	//filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]

	// 获取prefix
	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = path[firstSlashIdx+1 : lastSlashIdx+1]
	}

	if prefix != "" {
		prefix = "/" + prefix + "/"
	} else {
		prefix = "/"
	}

	fmt.Println("repo-id:", repoID)
	fmt.Println("prefix:", prefix)
	fmt.Println("filename:", filename)

	targetURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/batch-delete-item/"
	// 创建请求体
	requestBody := map[string]interface{}{
		"dirents":    []string{filename}, // 将 filename 放入数组中
		"parent_dir": prefix,
		"repo_id":    repoID,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	fmt.Println(jsonBody)

	// 创建 HTTP 请求
	request, err := http.NewRequest("DELETE", targetURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// 设置请求头
	request.Header = r.Header
	request.Header.Set("Content-Type", "application/json")

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 发送请求
	response, err := client.Do(request)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer response.Body.Close()

	// 检查响应状态码
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, fmt.Errorf("file delete failed with status: %d", response.StatusCode)
	}

	return http.StatusOK, nil
}

func pasteAddVersionSuffix(source string, dstType string, fs afero.Fs, w http.ResponseWriter, r *http.Request) string {
	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for {
		//if _, err := fs.Stat(source); err != nil {
		if _, _, _, _, err := getStat(fs, dstType, source, w, r); err != nil {
			break
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}

	return source
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
		fmt.Println(srcType, src, dstType, dst)

		validSrcTypes := map[string]bool{
			"drive":   true,
			"sync":    true,
			"cache":   true,
			"google":  true,
			"awss3":   true,
			"dropbox": true,
			"tencent": true,
		}

		if !validSrcTypes[srcType] {
			fmt.Println("Src type is invalid!")
			return http.StatusForbidden, nil
		}
		if !validSrcTypes[dstType] {
			fmt.Println("Dst type is invalid!")
			return http.StatusForbidden, nil
		}
		if srcType == dstType {
			fmt.Println("Src and dst are of same arch!")
		} else {
			fmt.Println("Src and dst are of different arches!")
		}
		//if srcType == "google" || dstType == "google" {
		//	err := GoogleDriveCall("/api/resources%2FHome%2FDocuments%2F", "GET", nil, w, r)
		//	return errToStatus(err), err
		//}
		action := r.URL.Query().Get("action")
		var err error
		src, err = url.QueryUnescape(src)
		dst, err = url.QueryUnescape(dst)
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
		if rename && dstType != "google" {
			dst = pasteAddVersionSuffix(dst, dstType, d.user.Fs, w, r)
		}
		// Permission for overwriting the file
		if override && !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}
		var same = srcType == dstType
		// google drive, awss3 of two users must be seen as diff archs
		var srcName, dstName, dstFilename string
		if srcType == "google" {
			_, srcName, _, _ = parseGoogleDrivePath(src)
		} else if srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			_, srcName, _ = parseAwss3Path(src, true)
		}
		if dstType == "google" {
			_, dstName, _, dstFilename = parseGoogleDrivePath(dst)
			if srcType != "google" && strings.Contains(dstFilename, "/") {
				strings.Replace(dstFilename, "/", "-", -1)
			}
			if srcType != "google" && strings.Contains(dstFilename, "%2F") {
				strings.Replace(dstFilename, "%2F", "-", -1)
			}
		} else if srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			_, dstName, _ = parseAwss3Path(dst, true)
		}
		if srcName != dstName {
			same = false
		}

		if same {
			err = d.RunHook(func() error {
				return pasteActionSameArch(r.Context(), action, srcType, src, dstType, dst, d, fileCache, override, rename, w, r)
			}, action, src, dst, d.user)
		} else {
			err = d.RunHook(func() error {
				return pasteActionDiffArch(r.Context(), action, srcType, src, dstType, dst, d, fileCache, w, r)
			}, action, src, dst, d.user)
		}
		if errToStatus(err) == http.StatusRequestEntityTooLarge {
			fmt.Fprintln(w, err.Error())
		}
		return errToStatus(err), err
	})
}

func syncPermToMode(permStr string) os.FileMode {
	// 将字符串权限转换为 os.FileMode
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
		fmt.Println("无效的权限字符串")
		return 0
	}

	fmt.Println("转换后的权限:", perm)
	return perm
}

func syncModeToPermString(fileMode os.FileMode) string {
	permStr := ""

	//if fileMode&os.ModeDir != 0 {
	//	// 目录权限
	//	permStr += "d"
	//} else {
	//	// 文件权限
	//	permStr += "-"
	//}

	// 所有者权限
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

	// 所属组权限
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

	// 其他用户权限
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

func getStat(fs afero.Fs, srcType, src string, w http.ResponseWriter, r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	// we need only size, fileMode and isDir for the time being for all arch
	if srcType == "drive" {
		info, err := fs.Stat(src)
		if err != nil {
			return nil, 0, 0, false, err
		}
		return info, info.Size(), info.Mode(), info.IsDir(), nil
	} else if srcType == "google" {
		if !strings.HasSuffix(src, "/") {
			src += "/"
		}
		metaInfo, err := getGoogleDriveIdFocusedMetaInfos(src, w, r)
		if err != nil {
			return nil, 0, 0, false, err
		}
		return nil, metaInfo.Size, 0755, metaInfo.IsDir, nil
	} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		src = strings.TrimSuffix(src, "/")
		metaInfo, err := getAwss3FocusedMetaInfos(src, w, r)
		if err != nil {
			return nil, 0, 0, false, err
		}
		return nil, metaInfo.Size, 0755, metaInfo.IsDir, nil
	} else if srcType == "cache" {
		//host := r.Host
		//infoUrl := "http://" + host + "/api/resources" + src
		infoURL := "http://127.0.0.1:80/api/resources" + src
		fmt.Println(infoURL)

		client := &http.Client{}
		request, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			fmt.Printf("创建请求失败: %v\n", err)
			return nil, 0, 0, false, err
		}

		request.Header = r.Header
		//fmt.Println("请求头:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
			return nil, 0, 0, false, err
		}
		defer response.Body.Close()

		//fmt.Println("响应头:")
		//for key, values := range response.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		var bodyReader io.Reader = response.Body

		// 检查Content-Encoding是否为gzip
		if response.Header.Get("Content-Encoding") == "gzip" {
			// 创建gzip Reader
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				fmt.Printf("解压缩响应失败: %v\n", err)
				return nil, 0, 0, false, err
			}
			defer gzipReader.Close()

			bodyReader = gzipReader
		}

		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			fmt.Printf("读取响应失败: %v\n", err)
			return nil, 0, 0, false, err
		}

		//fmt.Println("请求URL:", infoURL)
		//fmt.Println("请求头:", request.Header)
		//fmt.Println("响应状态码:", response.StatusCode)
		//fmt.Println("响应内容:", string(body))

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
			fmt.Printf("解析响应失败: %v\n", err)
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
		// 去除路径开头和结尾的斜杠
		src = strings.Trim(src, "/")
		// 检查路径中是否包含斜杠
		if !strings.Contains(src, "/") {
			err := e.New("invalid path format: path must contain at least one '/'")
			fmt.Println("Error:", err)
			return nil, 0, 0, false, err
		}

		// 获取第一个斜杠的索引
		firstSlashIdx := strings.Index(src, "/")

		// 获取repo-id
		repoID := src[:firstSlashIdx]

		// 获取最后一个斜杠的索引
		lastSlashIdx := strings.LastIndex(src, "/")

		// 获取filename
		filename := src[lastSlashIdx+1:]

		// 获取prefix
		prefix := ""
		if firstSlashIdx != lastSlashIdx {
			prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
		}

		fmt.Println("repo-id:", repoID)
		fmt.Println("prefix:", prefix)
		fmt.Println("filename:", filename)

		infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + url.QueryEscape("/"+prefix) + "&with_thumbnail=true"
		fmt.Println(infoURL)

		client := &http.Client{}
		request, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			fmt.Printf("创建请求失败: %v\n", err)
			return nil, 0, 0, false, err
		}

		request.Header = r.Header
		//fmt.Println("请求头:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
			return nil, 0, 0, false, err
		}
		defer response.Body.Close()

		//fmt.Println("响应头:")
		//for key, values := range response.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		var bodyReader io.Reader = response.Body

		// 检查Content-Encoding是否为gzip
		if response.Header.Get("Content-Encoding") == "gzip" {
			// 创建gzip Reader
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				fmt.Printf("解压缩响应失败: %v\n", err)
				return nil, 0, 0, false, err
			}
			defer gzipReader.Close()

			bodyReader = gzipReader
		}

		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			fmt.Printf("读取响应失败: %v\n", err)
			return nil, 0, 0, false, err
		}

		//fmt.Println("请求URL:", infoURL)
		//fmt.Println("请求头:", request.Header)
		//fmt.Println("响应状态码:", response.StatusCode)
		//fmt.Println("响应内容:", string(body))

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
			fmt.Printf("解析响应失败: %v\n", err)
			return nil, 0, 0, false, err
		}

		fmt.Println("GoogleDriveListResponseUser Perm:", dirResp.UserPerm)
		fmt.Println("Dir ID:", dirResp.DirID)
		fmt.Println("Dirent List:")
		var found = false
		for _, dirent := range dirResp.DirentList {
			if dirent.Name == filename {
				fileInfo = dirent
				fmt.Println("Type:", dirent.Type)
				fmt.Println("ID:", dirent.ID)
				fmt.Println("Name:", dirent.Name)
				fmt.Println("Mtime:", dirent.Mtime)
				fmt.Println("Permission:", dirent.Permission)
				fmt.Println("Parent Dir:", dirent.ParentDir)
				fmt.Println("Starred:", dirent.Starred)
				fmt.Println("Size:", dirent.Size)
				fmt.Println("FileSize:", dirent.FileSize)
				fmt.Println("NumTotalFiles:", dirent.NumTotalFiles)
				fmt.Println("NumFiles:", dirent.NumFiles)
				fmt.Println("NumDirs:", dirent.NumDirs)
				fmt.Println("Path:", dirent.Path)
				fmt.Println("Modifier Email:", dirent.ModifierEmail)
				fmt.Println("Modifier Name:", dirent.ModifierName)
				fmt.Println("Modifier Contact Email:", dirent.ModifierContactEmail)
				fmt.Println()
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
func copyDir(fs afero.Fs, srcType, src, dstType, dst string, d *data, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, driveIdCache map[string]string) error {
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
	} else if dstType == "google" {
		respBody, _, err := resourcePostGoogle(dst, w, r, true)
		var bodyJson GoogleDrivePostResponse
		if err = json.Unmarshal(respBody, &bodyJson); err != nil {
			fmt.Println(err)
			return err
		}
		driveIdCache[src] = bodyJson.Data.Meta.ID
		if err != nil {
			return err
		}
	} else if dstType == "awss3" || dstType == "tencent" || dstType == "dropbox" {
		_, _, err := resourcePostAwss3(dst, w, r, false)
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

	var fdstBase string = dst
	if driveIdCache[src] != "" {
		fdstBase = filepath.Dir(filepath.Dir(dst)) + "/" + driveIdCache[src]
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
			fdst := fdstBase + "/" + obj.Name()

			fmt.Println(fsrc, fdst)
			if obj.IsDir() {
				// Create sub-directories, recursively.
				err = copyDir(fs, srcType, fsrc, dstType, fdst, d, obj.Mode(), w, r, driveIdCache)
				if err != nil {
					errs = append(errs, err)
				}
			} else {
				// Perform the file copy.
				err = copyFile(fs, srcType, fsrc, dstType, fdst, d, obj.Mode(), obj.Size(), w, r, driveIdCache)
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
	} else if srcType == "google" {
		if !strings.HasSuffix(src, "/") {
			src += "/"
		}

		srcDrive, srcName, pathId, _ := parseGoogleDrivePath(src)

		param := GoogleDriveListParam{
			Path:  pathId,
			Drive: srcDrive,
			Name:  srcName,
		}

		// 将数据序列化为 JSON
		jsonBody, err := json.Marshal(param)
		if err != nil {
			fmt.Println("Error marshalling JSON:", err)
			return err
		}
		fmt.Println("Google Drive List Params:", string(jsonBody))
		var respBody []byte
		respBody, err = GoogleDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/ls:", err)
			return err
		}
		var bodyJson GoogleDriveListResponse
		if err = json.Unmarshal(respBody, &bodyJson); err != nil {
			fmt.Println(err)
			return err
		}
		for _, item := range bodyJson.Data {
			//fsrc := filepath.Join(src, item.Name)
			fsrc := filepath.Dir(strings.TrimSuffix(src, "/")) + "/" + item.Meta.ID
			fdst := filepath.Join(fdstBase, item.Name)
			fmt.Println(fsrc, fdst)
			if item.IsDir {
				// 创建子目录，递归处理
				err = copyDir(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), w, r, driveIdCache)
				if err != nil {
					return err
				}
			} else {
				// 执行文件复制
				err = copyFile(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), item.FileSize, w, r, driveIdCache)
				if err != nil {
					return err
				}
			}
		}
	} else if srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		//src = strings.TrimSuffix(src, "/")

		srcDrive, srcName, srcPath := parseAwss3Path(src, true)

		param := Awss3ListParam{
			Path:  srcPath,
			Drive: srcDrive,
			Name:  srcName,
		}

		jsonBody, err := json.Marshal(param)
		if err != nil {
			fmt.Println("Error marshalling JSON:", err)
			return err
		}
		fmt.Println("Awss3 List Params:", string(jsonBody))
		var respBody []byte
		respBody, err = Awss3Call("/drive/ls", "POST", jsonBody, w, r, true)
		if err != nil {
			fmt.Println("Error calling drive/ls:", err)
			return err
		}
		var bodyJson Awss3ListResponse
		if err = json.Unmarshal(respBody, &bodyJson); err != nil {
			fmt.Println(err)
			return err
		}
		for _, item := range bodyJson.Data {
			fsrc := filepath.Join(src, item.Name)
			fdst := filepath.Join(fdstBase, item.Name)
			fmt.Println(fsrc, fdst)
			if item.IsDir {
				err = copyDir(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), w, r, driveIdCache)
				if err != nil {
					return err
				}
			} else {
				err = copyFile(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), item.FileSize, w, r, driveIdCache)
				if err != nil {
					return err
				}
			}
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

		infoURL := "http://127.0.0.1:80/api/resources" + src
		fmt.Println(infoURL)

		client := &http.Client{}
		request, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			fmt.Printf("创建请求失败: %v\n", err)
			return err
		}

		request.Header = r.Header
		//fmt.Println("请求头:")
		//for key, values := range request.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
			return err
		}
		defer response.Body.Close()

		//fmt.Println("响应头:")
		//for key, values := range response.Header {
		//	for _, value := range values {
		//		fmt.Printf("%s: %s\n", key, value)
		//	}
		//}

		var bodyReader io.Reader = response.Body

		// 检查Content-Encoding是否为gzip
		if response.Header.Get("Content-Encoding") == "gzip" {
			// 创建gzip Reader
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				fmt.Printf("解压缩响应失败: %v\n", err)
				return err
			}
			defer gzipReader.Close()

			bodyReader = gzipReader
		}

		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			fmt.Printf("读取响应失败: %v\n", err)
			return err
		}

		//fmt.Println("请求URL:", infoURL)
		//fmt.Println("请求头:", request.Header)
		//fmt.Println("响应状态码:", response.StatusCode)
		//fmt.Println("响应内容:", string(body))

		var data ResponseData
		//err = json.NewDecoder(response.Body).Decode(&data)
		err = json.Unmarshal(body, &data)
		if err != nil {
			return err
		}

		for _, item := range data.Items {
			fsrc := filepath.Join(src, item.Name)
			fdst := filepath.Join(fdstBase, item.Name)

			fmt.Println(fsrc, fdst)
			if item.IsDir {
				// 创建子目录，递归处理
				err := copyDir(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), w, r, driveIdCache)
				if err != nil {
					return err
				}
			} else {
				// 执行文件复制
				err := copyFile(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), item.Size, w, r, driveIdCache)
				if err != nil {
					return err
				}
			}
		}
		return nil
	} else if srcType == "sync" {
		fmt.Println("Sync copy/move dir!")
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

		// 去除路径开头和结尾的斜杠
		src = strings.Trim(src, "/")
		// 检查路径中是否包含斜杠
		if !strings.Contains(src, "/") {
			err := e.New("invalid path format: path must contain at least one '/'")
			fmt.Println("Error:", err)
			return err
		}

		// 获取第一个斜杠的索引
		firstSlashIdx := strings.Index(src, "/")

		// 获取repo-id
		repoID := src[:firstSlashIdx]

		// 获取最后一个斜杠的索引
		lastSlashIdx := strings.LastIndex(src, "/")

		// 获取filename
		filename := src[lastSlashIdx+1:]

		// 获取prefix
		prefix := ""
		if firstSlashIdx != lastSlashIdx {
			prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
		}

		fmt.Println("repo-id:", repoID)
		fmt.Println("prefix:", prefix)
		fmt.Println("filename:", filename)

		infoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + url.QueryEscape("/"+prefix+"/"+filename) + "&with_thumbnail=true"
		fmt.Println(infoURL)

		client := &http.Client{}
		request, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			fmt.Printf("创建请求失败: %v\n", err)
			return err
		}

		request.Header = r.Header
		fmt.Println("请求头:")
		for key, values := range request.Header {
			for _, value := range values {
				fmt.Printf("%s: %s\n", key, value)
			}
		}

		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
			return err
		}
		defer response.Body.Close()

		var bodyReader io.Reader = response.Body

		// 检查Content-Encoding是否为gzip
		if response.Header.Get("Content-Encoding") == "gzip" {
			// 创建gzip Reader
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				fmt.Printf("解压缩响应失败: %v\n", err)
				return err
			}
			defer gzipReader.Close()

			bodyReader = gzipReader
		}

		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			fmt.Printf("读取响应失败: %v\n", err)
			return err
		}

		fmt.Println("请求URL:", infoURL)
		fmt.Println("请求头:", request.Header)
		fmt.Println("响应状态码:", response.StatusCode)
		fmt.Println("响应内容:", string(body))

		var data ResponseData
		//err = json.NewDecoder(response.Body).Decode(&data)
		err = json.Unmarshal(body, &data)
		if err != nil {
			return err
		}

		for _, item := range data.DirentList {
			fsrc := filepath.Join(src, item.Name)
			fdst := filepath.Join(fdstBase, item.Name)

			fmt.Println(fsrc, fdst)
			if item.Type == "dir" {
				// 创建子目录，递归处理
				err := copyDir(fs, srcType, fsrc, dstType, fdst, d, syncPermToMode(item.Permission), w, r, driveIdCache)
				if err != nil {
					return err
				}
			} else {
				// 执行文件复制
				err := copyFile(fs, srcType, fsrc, dstType, fdst, d, syncPermToMode(item.Permission), item.Size, w, r, driveIdCache)
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
func copyFile(fs afero.Fs, srcType, src, dstType, dst string, d *data, mode os.FileMode, diskSize int64,
	w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}
	extRemains := dstType == "google" || dstType == "awss3" || dstType == "tencent" || dstType == "dropbox"
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
		// Won't deal a file which is bigger than 4G for the time being
		if diskSize >= 4*1024*1024*1024 {
			fmt.Println("file size exceeds 4GB")
			return e.New("file size exceeds 4GB") //os.ErrPermission
		}
		fmt.Println("Will reserve disk size: ", diskSize)
		bufferPath, err = generateBufferFileName(src, bflName, extRemains)
		if err != nil {
			return err
		}
		fmt.Println("Buffer file path: ", bufferPath)
		err = makeDiskBuffer(bufferPath, diskSize, false)
		if err != nil {
			return err
		}
		err = driveFileToBuffer(fileInfo, bufferPath)
		if err != nil {
			return err
		}
	} else if srcType == "google" {
		var err error
		if diskSize >= 4*1024*1024*1024 {
			fmt.Println("file size exceeds 4GB")
			return e.New("file size exceeds 4GB") //os.ErrPermission
		}
		fmt.Println("Will reserve disk size: ", diskSize)
		srcInfo, err := getGoogleDriveIdFocusedMetaInfos(src, w, r)
		bufferFilePath, err := generateBufferFolder(srcInfo.Path, bflName)
		if err != nil {
			return err
		}
		bufferPath = filepath.Join(bufferFilePath, url.QueryEscape(srcInfo.Name))
		fmt.Println("Buffer file path: ", bufferFilePath)
		fmt.Println("Buffer path: ", bufferPath)
		err = makeDiskBuffer(bufferPath, diskSize, true)
		if err != nil {
			return err
		}
		_, err = googleFileToBuffer(src, bufferFilePath, w, r)
		//bufferPath = filepath.Join(bufferFilePath, bufferFilename)
		//fmt.Println("Buffer file path: ", bufferFilePath)
		//fmt.Println("Buffer path: ", bufferPath)
		if err != nil {
			return err
		}
	} else if srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		var err error
		if diskSize >= 4*1024*1024*1024 {
			fmt.Println("file size exceeds 4GB")
			return e.New("file size exceeds 4GB") //os.ErrPermission
		}
		fmt.Println("Will reserve disk size: ", diskSize)
		srcInfo, err := getAwss3FocusedMetaInfos(src, w, r)
		bufferFilePath, err := generateBufferFolder(srcInfo.Path, bflName)
		if err != nil {
			return err
		}
		bufferPath = filepath.Join(bufferFilePath, srcInfo.Name)
		fmt.Println("Buffer file path: ", bufferFilePath)
		fmt.Println("Buffer path: ", bufferPath)
		err = makeDiskBuffer(bufferPath, diskSize, true)
		if err != nil {
			return err
		}
		err = awss3FileToBuffer(src, bufferFilePath, w, r)
		if err != nil {
			return err
		}
	} else if srcType == "cache" {
		var err error
		if diskSize >= 4*1024*1024*1024 {
			fmt.Println("file size exceeds 4GB")
			return e.New("file size exceeds 4GB") //os.ErrPermission
		}
		fmt.Println("Will reserve disk size: ", diskSize)
		bufferPath, err = generateBufferFileName(src, bflName, extRemains)
		if err != nil {
			return err
		}
		fmt.Println("Buffer file path: ", bufferPath)
		err = makeDiskBuffer(bufferPath, diskSize, false)
		if err != nil {
			return err
		}
		err = cacheFileToBuffer(src, bufferPath)
		if err != nil {
			return err
		}
	} else if srcType == "sync" {
		var err error
		if diskSize >= 4*1024*1024*1024 {
			fmt.Println("file size exceeds 4GB")
			return e.New("file size exceeds 4GB") //os.ErrPermission
		}
		fmt.Println("Will reserve disk size: ", diskSize)
		bufferPath, err = generateBufferFileName(src, bflName, extRemains)
		if err != nil {
			return err
		}
		fmt.Println("Buffer file path: ", bufferPath)
		err = makeDiskBuffer(bufferPath, diskSize, false)
		if err != nil {
			return err
		}
		err = syncFileToBuffer(src, bufferPath, r)
		if err != nil {
			return err
		}
	}

	rename := r.URL.Query().Get("rename") == "true"
	if rename && dstType != "google" && srcType == "google" {
		dst = pasteAddVersionSuffix(dst, dstType, d.user.Fs, w, r)
	}

	// paste
	if dstType == "drive" {
		fmt.Println("Begin to paste!")
		status, err := driveBufferToFile(bufferPath, dst, mode, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		fmt.Println("Begin to remove buffer")
		removeDiskBuffer(bufferPath, srcType)
	} else if dstType == "google" {
		fmt.Println("Begin to paste!")
		//key := filepath.Dir(strings.TrimSuffix(src, "/"))
		//dstPathId := driveIdCache[key]
		//var newDst string
		//if dstPathId != "" {
		//	tempDst := strings.TrimSuffix(dstPathId, "/")
		//	dstFilename := filepath.Base(tempDst)
		//	newDst = filepath.Dir(filepath.Dir(tempDst)) + "/" + dstPathId + "/" + dstFilename
		//} else {
		//	// for single file
		//	newDst = dst
		//}
		//fmt.Println("newDst: ", newDst)
		fmt.Println("dst: ", dst)
		status, err := googleBufferToFile(bufferPath, dst, w, r)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		fmt.Println("Begin to remove buffer")
		removeDiskBuffer(bufferPath, srcType)
	} else if dstType == "awss3" || dstType == "tencent" || dstType == "dropbox" {
		fmt.Println("Begin to paste!")
		fmt.Println("dst: ", dst)
		status, err := awss3BufferToFile(bufferPath, dst, w, r)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		fmt.Println("Begin to remove buffer")
		removeDiskBuffer(bufferPath, srcType)
	} else if dstType == "cache" {
		fmt.Println("Begin to cache paste!")
		status, err := cacheBufferToFile(bufferPath, dst, mode, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		fmt.Println("Begin to remove buffer")
		removeDiskBuffer(bufferPath, srcType)
	} else if dstType == "sync" {
		fmt.Println("Begin to sync paste!")
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
		fmt.Println("Begin to remove buffer")
		removeDiskBuffer(bufferPath, srcType)
	}
	return nil
}

func doPaste(fs afero.Fs, srcType, src, dstType, dst string, d *data, w http.ResponseWriter, r *http.Request) error {
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
	_, size, mode, isDir, err := getStat(fs, srcType, src, w, r)
	if err != nil {
		return err
	}

	var copyTempGoogleDrivePathIdCache = make(map[string]string)

	if isDir {
		err = copyDir(fs, srcType, src, dstType, dst, d, mode, w, r, copyTempGoogleDrivePathIdCache)
	} else {
		err = copyFile(fs, srcType, src, dstType, dst, d, mode, size, w, r, copyTempGoogleDrivePathIdCache)
	}
	if err != nil {
		return err
	}
	return nil
}

func moveDelete(fileCache FileCache, srcType, src string, ctx context.Context, d *data, w http.ResponseWriter, r *http.Request) error {
	if srcType == "drive" {
		status, err := resourceDriveDelete(fileCache, src, ctx, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	} else if srcType == "google" {
		_, status, err := resourceDeleteGoogle(fileCache, src, w, r, true)
		if status != http.StatusOK && status != 0 {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	} else if srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		_, status, err := resourceDeleteAwss3(fileCache, src, w, r, true)
		if status != http.StatusOK && status != 0 {
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

func pasteActionSameArch(ctx context.Context, action, srcType, src, dstType, dst string, d *data, fileCache FileCache, override, rename bool, w http.ResponseWriter, r *http.Request) error {
	fmt.Println("Now deal with ", action, " for same arch ", dstType)
	if srcType == "drive" || srcType == "cache" {
		url := "http://127.0.0.1:80/api/resources/" + strings.TrimLeft(src, "/") + "?action=" + action + "&destination=" + url.QueryEscape(dst) + "&override=" + strconv.FormatBool(override) + "&rename=" + strconv.FormatBool(rename)
		method := "PATCH"
		payload := []byte(``)
		fmt.Println(url)

		client := &http.Client{}
		req, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
		if err != nil {
			return err
		}

		// 获取原始请求的头部信息
		req.Header = r.Header

		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		respBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		fmt.Println(respBody)
		return nil
	} else if srcType == "google" {
		switch action {
		case "copy":
			if !strings.HasSuffix(src, "/") {
				src += "/"
			}
			metaInfo, err := getGoogleDriveIdFocusedMetaInfos(src, w, r)
			if err != nil {
				return err
			}

			if metaInfo.IsDir {
				return copyGoogleDriveFolder(src, dst, w, r, metaInfo.Path, metaInfo.Name)
			}
			return copyGoogleDriveSingleFile(src, dst, w, r)
		case "rename":
			if !strings.HasSuffix(src, "/") {
				src += "/"
			}
			return moveGoogleDriveFolderOrFiles(src, dst, w, r)
		}
	} else if srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		switch action {
		case "copy":
			if strings.HasSuffix(src, "/") {
				src = strings.TrimSuffix(src, "/")
			}
			metaInfo, err := getAwss3FocusedMetaInfos(src, w, r)
			if err != nil {
				return err
			}

			if metaInfo.IsDir {
				// TODO: should wait for creating folder function
				return copyAwss3Folder(src, dst, w, r, metaInfo.Path, metaInfo.Name)
			}
			return copyAwss3SingleFile(src, dst, w, r)
		case "rename":
			if !strings.HasSuffix(src, "/") {
				src += "/"
			}
			return moveAwss3FolderOrFiles(src, dst, w, r)
		}
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

		// 去除路径开头和结尾的斜杠
		src = strings.Trim(src, "/")
		// 检查路径中是否包含斜杠
		if !strings.Contains(src, "/") {
			err := e.New("invalid path format: path must contain at least one '/'")
			fmt.Println("Error:", err)
			return err
		}

		// 获取第一个斜杠的索引
		srcFirstSlashIdx := strings.Index(src, "/")

		// 获取repo-id
		srcRepoID := src[:srcFirstSlashIdx]

		// 获取最后一个斜杠的索引
		srcLastSlashIdx := strings.LastIndex(src, "/")

		// 获取filename
		srcFilename := src[srcLastSlashIdx+1:]
		//filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]

		// 获取prefix
		srcPrefix := ""
		if srcFirstSlashIdx != srcLastSlashIdx {
			srcPrefix = src[srcFirstSlashIdx+1 : srcLastSlashIdx+1]
		}

		if srcPrefix != "" {
			srcPrefix = "/" + srcPrefix
		} else {
			srcPrefix = "/"
		}

		fmt.Println("src repo-id:", srcRepoID)
		fmt.Println("src prefix:", srcPrefix)
		fmt.Println("src filename:", srcFilename)

		// 去除路径开头和结尾的斜杠
		dst = strings.Trim(dst, "/")
		// 检查路径中是否包含斜杠
		if !strings.Contains(dst, "/") {
			err := e.New("invalid path format: path must contain at least one '/'")
			fmt.Println("Error:", err)
			return err
		}

		// 获取第一个斜杠的索引
		dstFirstSlashIdx := strings.Index(dst, "/")

		// 获取repo-id
		dstRepoID := dst[:dstFirstSlashIdx]

		// 获取最后一个斜杠的索引
		dstLastSlashIdx := strings.LastIndex(dst, "/")

		// 获取filename
		dstFilename := dst[dstLastSlashIdx+1:]
		//filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]

		// 获取prefix
		dstPrefix := ""
		if dstFirstSlashIdx != dstLastSlashIdx {
			dstPrefix = dst[dstFirstSlashIdx+1 : dstLastSlashIdx+1]
		}

		if dstPrefix != "" {
			dstPrefix = "/" + dstPrefix
		} else {
			dstPrefix = "/"
		}

		fmt.Println("dst repo-id:", dstRepoID)
		fmt.Println("dst prefix:", dstPrefix)
		fmt.Println("dst filename:", dstFilename)

		targetURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + apiName + "/"
		// 创建请求体
		requestBody := map[string]interface{}{
			"dst_parent_dir": dstPrefix,
			"dst_repo_id":    dstRepoID,
			"src_dirents":    []string{srcFilename}, // 将 filename 放入数组中
			"src_parent_dir": srcPrefix,
			"src_repo_id":    srcRepoID,
		}
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		fmt.Println(jsonBody)

		// 创建 HTTP 请求
		request, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}

		// 设置请求头
		request.Header = r.Header
		request.Header.Set("Content-Type", "application/json")

		// 创建 HTTP 客户端
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		// 发送请求
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		// 检查响应状态码
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("file delete failed with status: %d", response.StatusCode)
		}

		return nil
	}
	return nil
}

func pasteActionDiffArch(ctx context.Context, action, srcType, src, dstType, dst string, d *data, fileCache FileCache, w http.ResponseWriter, r *http.Request) error {
	// In this function, context if tied up to src, because src is in the URL
	switch action {
	// TODO: use enum
	case "copy":
		if !d.user.Perm.Create {
			return errors.ErrPermissionDenied
		}

		return doPaste(d.user.Fs, srcType, src, dstType, dst, d, w, r)
	case "rename":
		if !d.user.Perm.Rename {
			return errors.ErrPermissionDenied
		}
		err := doPaste(d.user.Fs, srcType, src, dstType, dst, d, w, r)
		if err != nil {
			return err
		}

		err = moveDelete(fileCache, srcType, src, ctx, d, w, r)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
	return nil
}
