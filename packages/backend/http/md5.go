package http

import (
	"compress/gzip"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/common"
	"github.com/filebrowser/filebrowser/v2/files"
	"io"
	"net/http"
	"strings"
)

func downloadAndComputeMD5(r *http.Request, url string) (string, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	request.Header = r.Header

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status code: %d", resp.StatusCode)
	}

	hasher := md5.New()
	var reader io.Reader = resp.Body

	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	buf := make([]byte, 8192)
	totalBytesRead := 0

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
			totalBytesRead += n
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %w", err)
		}
	}

	md5Sum := hasher.Sum(nil)
	fmt.Printf("MD5 calculated based on %d bytes of data\n", totalBytesRead)

	return fmt.Sprintf("%x", md5Sum), nil
}

func md5Sync(w http.ResponseWriter, r *http.Request) (int, error) {
	// src is like [repo-id]/path/filename
	src := r.URL.Path
	src = strings.Trim(src, "/")
	if !strings.Contains(src, "/") {
		err := errors.New("invalid path format: path must contain at least one '/'")
		fmt.Println("Error:", err)
		return errToStatus(err), err
	}

	firstSlashIdx := strings.Index(src, "/")

	repoID := src[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(src, "/")

	filename := src[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}

	fmt.Println("repo-id:", repoID)
	fmt.Println("prefix:", prefix)
	fmt.Println("filename:", filename)

	url := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file/" + prefix + filename + "/" + "?dl=1"
	fmt.Println(url)

	md5, err := downloadAndComputeMD5(r, url)
	if err != nil {
		return errToStatus(err), err
	}

	responseData := make(map[string]interface{})
	responseData["md5"] = md5
	return renderJSON(w, r, responseData)
}

func md5FileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
	//fd, err := file.Fs.Open(file.Path)
	//if err != nil {
	//	return http.StatusInternalServerError, err
	//}
	//defer fd.Close()

	var err error
	responseData := make(map[string]interface{})
	responseData["md5"], err = common.Md5File(file.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, responseData)
}

var md5Handler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	srcType := r.URL.Query().Get("src")
	if srcType == "sync" {
		return md5Sync(w, r)
	}

	if !d.user.Perm.Download {
		return http.StatusAccepted, nil
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

	if file.IsDir {
		err = errors.New("only support md5 for file")
		return http.StatusForbidden, err
	}

	return md5FileHandler(w, r, file)
})
