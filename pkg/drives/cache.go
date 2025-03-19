package drives

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"fmt"
	"github.com/spf13/afero"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CacheResourceService struct {
	BaseResourceService
}

func (*CacheResourceService) PasteSame(action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	patchUrl := "http://127.0.0.1:80/api/resources/" + common.EscapeURLWithSpace(strings.TrimLeft(src, "/")) + "?action=" + action + "&destination=" + common.EscapeURLWithSpace(dst) + "&rename=" + strconv.FormatBool(rename)
	method := "PATCH"
	payload := []byte(``)
	klog.Infoln(patchUrl)

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

	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	return nil
}

func (rs *CacheResourceService) PasteDirFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
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
		fdstBase = filepath.Dir(filepath.Dir(dst)) + "/" + driveIdCache[src]
	}

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

	infoURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(src)

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

	for _, item := range data.Items {
		fsrc := filepath.Join(src, item.Name)
		fdst := filepath.Join(fdstBase, item.Name)

		if item.IsDir {
			err := rs.PasteDirFrom(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			err := rs.PasteFileFrom(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), item.Size, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *CacheResourceService) PasteDirTo(fs afero.Fs, src, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	if err := CacheMkdirAll(dst, fileMode, r); err != nil {
		return err
	}
	return nil
}

func (rs *CacheResourceService) PasteFileFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
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
	err = CacheFileToBuffer(src, bufferPath)
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

func (rs *CacheResourceService) PasteFileTo(fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	status, err := CacheBufferToFile(bufferPath, dst, fileMode, d)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *CacheResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}

	infoURL := "http://127.0.0.1:80/api/resources" + common.EscapeURLWithSpace(src)

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
		klog.Errorf("parse response failed: %v\n", err)
		return nil, 0, 0, false, err
	}

	return nil, fileInfo.Size, fileInfo.Mode, fileInfo.IsDir, nil
}

func (rs *CacheResourceService) MoveDelete(fileCache fileutils.FileCache, src string, ctx context.Context, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	status, err := ResourceCacheDelete(fileCache, src, ctx, d)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
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

	err = fileutils.IoCopyFileWithBufferOs(newPath, bufferFilePath, 8*1024*1024)
	if err != nil {
		return err
	}

	return nil
}

func CacheBufferToFile(bufferFilePath string, targetPath string, mode os.FileMode, d *common.Data) (int, error) {
	// Directories creation on POST.
	if strings.HasSuffix(targetPath, "/") {
		if err := files.DefaultFs.MkdirAll(targetPath, mode); err != nil {
			return common.ErrToStatus(err), err
		}
		if err := fileutils.Chown(files.DefaultFs, targetPath, 1000, 1000); err != nil {
			klog.Errorf("can't chown directory %s to user %d: %s", targetPath, 1000, err)
			return common.ErrToStatus(err), err
		}
	}

	_, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       targetPath,
		Modify:     true,
		Expand:     false,
		ReadHeader: d.Server.TypeDetectionByHeader,
	})

	newTargetPath := strings.Replace(targetPath, "AppData/", "appcache/", 1)
	err = fileutils.IoCopyFileWithBufferOs(bufferFilePath, newTargetPath, 8*1024*1024)

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
