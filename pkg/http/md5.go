package http

import (
	"compress/gzip"
	"crypto/md5"
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

func downloadAndComputeMD5(r *http.Request, url string) (string, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	request.Header = r.Header.Clone()

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

func md5Sync(fileParam *models.FileParam, w http.ResponseWriter, r *http.Request) (int, error) {
	repoID := fileParam.Extend
	prefix, filename := filepath.Split(fileParam.Path)

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

func Md5Handler(w http.ResponseWriter, r *http.Request) (int, error) {
	var prefix = "/api/md5"

	var p = r.URL.Path
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return http.StatusBadRequest, errors.New("path invalid")
	}

	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return http.StatusBadRequest, errors.New("user not found")
	}
	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if fileParam.FileType == common.Sync {
		return md5Sync(fileParam, w, r)
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return http.StatusBadRequest, err
	}
	urlPath := uri + fileParam.Path
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       strings.TrimPrefix(urlPath, "/data"),
		Modify:     true,
		Expand:     false,
		ReadHeader: true,
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
