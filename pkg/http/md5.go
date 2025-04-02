package http

import (
	"compress/gzip"
	"crypto/md5"
	"errors"
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/files"
	"fmt"
	"io"
	"k8s.io/klog/v2"
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
	klog.Infof("MD5 calculated based on %d bytes of data\n", totalBytesRead)

	return fmt.Sprintf("%x", md5Sum), nil
}

func md5Sync(w http.ResponseWriter, r *http.Request) (int, error) {
	src := r.URL.Path
	src = strings.Trim(src, "/")
	if !strings.Contains(src, "/") {
		err := errors.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
		return common.ErrToStatus(err), err
	}

	firstSlashIdx := strings.Index(src, "/")

	repoID := src[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(src, "/")

	filename := src[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}

	klog.Infoln("repo-id:", repoID)
	klog.Infoln("prefix:", prefix)
	klog.Infoln("filename:", filename)

	url := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file/" + prefix + filename + "/" + "?dl=1"
	klog.Infoln(url)

	md5, err := downloadAndComputeMD5(r, url)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	responseData := make(map[string]interface{})
	responseData["md5"] = md5
	return common.RenderJSON(w, r, responseData)
}

func md5FileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
	var err error
	responseData := make(map[string]interface{})
	responseData["md5"], err = common.Md5File(file.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return common.RenderJSON(w, r, responseData)
}

func md5Handler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	//srcType := r.URL.Query().Get("src")
	srcType, err := ParsePathType(r.URL.Path, r, false, true)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if srcType == drives.SrcTypeSync {
		return md5Sync(w, r)
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       r.URL.Path,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})
	if err != nil {
		return common.ErrToStatus(err), err
	}

	if file.IsDir {
		err = errors.New("only support md5 for file")
		return http.StatusForbidden, err
	}

	return md5FileHandler(w, r, file)
}
