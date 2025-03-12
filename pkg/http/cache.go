package http

import (
	"context"
	"files/pkg/files"
	"fmt"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strings"
)

func cacheMkdirAll(dst string, mode os.FileMode, r *http.Request) error {
	targetURL := "http://127.0.0.1:80/api/resources" + escapeURLWithSpace(dst) + "/?mode=" + mode.String()

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
		return fmt.Errorf("file upload failed with status: %d", response.StatusCode)
	}

	return nil
}

func cacheFileToBuffer(src string, bufferFilePath string) error {
	newSrc := strings.Replace(src, "AppData/", "appcache/", 1)
	newPath, err := unescapeURLIfEscaped(newSrc)
	if err != nil {
		return err
	}
	klog.Infoln("newSrc:", newSrc, ", newPath:", newPath)

	err = ioCopyFileWithBuffer(newPath, bufferFilePath, 8*1024*1024)
	if err != nil {
		return err
	}

	return nil
}

func cacheBufferToFile(bufferFilePath string, targetPath string, mode os.FileMode, d *data) (int, error) {
	// Directories creation on POST.
	if strings.HasSuffix(targetPath, "/") {
		err := files.DefaultFs.MkdirAll(targetPath, mode)
		return errToStatus(err), err
	}

	_, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       targetPath,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.server.TypeDetectionByHeader,
	})

	newTargetPath := strings.Replace(targetPath, "AppData/", "appcache/", 1)
	err = ioCopyFileWithBuffer(bufferFilePath, newTargetPath, 8*1024*1024)

	if err != nil {
		err = os.RemoveAll(newTargetPath)
		if err == nil {
			klog.Errorln("Rollback Failed:", err)
		}
		klog.Infoln("Rollback success")
	}

	return errToStatus(err), err
}

func resourceCacheDelete(fileCache FileCache, path string, ctx context.Context, d *data) (int, error) {
	if path == "/" {
		return http.StatusForbidden, nil
	}

	newTargetPath := strings.Replace(path, "AppData/", "appcache/", 1)
	err := os.RemoveAll(newTargetPath)

	if err != nil {
		return errToStatus(err), err
	}

	return http.StatusOK, nil
}
