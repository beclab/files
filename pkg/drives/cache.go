package drives

import (
	"context"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"fmt"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strings"
)

type CacheResourceService struct {
	BaseResourceService
}

func CacheMkdirAll(dst string, mode os.FileMode, r *http.Request) error {
	targetURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(dst) + "/?mode=" + mode.String()

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

func CacheFileToBuffer(src string, bufferFilePath string) error {
	newSrc := strings.Replace(src, "AppData/", "appcache/", 1)
	newPath, err := common.UnescapeURLIfEscaped(newSrc)
	if err != nil {
		return err
	}
	klog.Infoln("newSrc:", newSrc, ", newPath:", newPath)

	err = fileutils.IoCopyFileWithBuffer(files.DefaultFs, newPath, bufferFilePath, 8*1024*1024)
	if err != nil {
		return err
	}

	return nil
}

func CacheBufferToFile(bufferFilePath string, targetPath string, mode os.FileMode, d *common.Data) (int, error) {
	// Directories creation on POST.
	if strings.HasSuffix(targetPath, "/") {
		err := files.DefaultFs.MkdirAll(targetPath, mode)
		return common.ErrToStatus(err), err
	}

	_, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       targetPath,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})

	newTargetPath := strings.Replace(targetPath, "AppData/", "appcache/", 1)
	err = fileutils.IoCopyFileWithBuffer(files.DefaultFs, bufferFilePath, newTargetPath, 8*1024*1024)

	if err != nil {
		err = os.RemoveAll(newTargetPath)
		if err == nil {
			klog.Errorln("Rollback Failed:", err)
		}
		klog.Infoln("Rollback success")
	}

	return common.ErrToStatus(err), err
}

func ResourceCacheDelete(fileCache fileutils.FileCache, path string, ctx context.Context, d *common.Data) (int, error) {
	if path == "/" {
		return http.StatusForbidden, nil
	}

	newTargetPath := strings.Replace(path, "AppData/", "appcache/", 1)
	err := os.RemoveAll(newTargetPath)

	if err != nil {
		return common.ErrToStatus(err), err
	}

	return http.StatusOK, nil
}
