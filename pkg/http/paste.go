package http

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/fileutils"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"files/pkg/errors"
	"files/pkg/files"
	"github.com/spf13/afero"
)

func pasteAddVersionSuffix(source string, dstType string, fs afero.Fs, w http.ResponseWriter, r *http.Request) string {
	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	renamed := ""

	for {
		var isDir bool
		var err error
		if _, _, _, isDir, err = getStat(fs, dstType, source, w, r); err != nil {
			break
		}
		if !isDir {
			renamed = fmt.Sprintf("%s(%d)%s", base, counter, ext)
		} else {
			renamed = fmt.Sprintf("%s(%d)", name, counter)
		}
		source = path.Join(dir, renamed)
		counter++
	}

	return source
}

func resourcePasteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
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

		validSrcTypes := map[string]bool{
			"drive":   true,
			"sync":    true,
			"cache":   true,
			"cloud":   true,
			"google":  true,
			"awss3":   true,
			"dropbox": true,
			"tencent": true,
		}

		if !validSrcTypes[srcType] {
			klog.Infoln("Src type is invalid!")
			return http.StatusForbidden, nil
		}
		if !validSrcTypes[dstType] {
			klog.Infoln("Dst type is invalid!")
			return http.StatusForbidden, nil
		}
		if srcType == dstType {
			klog.Infoln("Src and dst are of same arch!")
		} else {
			klog.Infoln("Src and dst are of different arches!")
		}
		action := r.URL.Query().Get("action")
		var err error
		klog.Infoln("src:", src)
		src, err = common.UnescapeURLIfEscaped(src)
		klog.Infoln("src:", src, "err:", err)
		klog.Infoln("dst:", dst)
		dst, err = common.UnescapeURLIfEscaped(dst)
		klog.Infoln("dst:", dst, "err:", err)
		if err != nil {
			return common.ErrToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		if dstType == "sync" && strings.Contains(dst, "\\") {
			response := map[string]interface{}{
				"code": -1,
				"msg":  "Sync does not support directory entries with backslashes in their names.",
			}
			return common.RenderJSON(w, r, response)
		}

		override := r.URL.Query().Get("override") == "true"
		rename := r.URL.Query().Get("rename") == "true"
		if !override && !rename {
			if _, err := files.DefaultFs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		if srcType == "google" && dstType != "google" {
			srcInfo, err := drives.GetGoogleDriveIdFocusedMetaInfos(src, w, r)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			srcName := srcInfo.Name
			formattedSrcName := common.RemoveSlash(srcName)
			dst = strings.ReplaceAll(dst, srcName, formattedSrcName)

			if !srcInfo.CanDownload {
				if srcInfo.CanExport {
					dst += srcInfo.ExportSuffix
				} else {
					response := map[string]interface{}{
						"code": -1,
						"msg":  "Google drive cannot export this file.",
					}
					return common.RenderJSON(w, r, response)
				}
			}
		}
		if rename && dstType != "google" {
			dst = pasteAddVersionSuffix(dst, dstType, files.DefaultFs, w, r)
		}
		// Permission for overwriting the file
		if override {
			return http.StatusForbidden, nil
		}
		var same = srcType == dstType
		// all cloud drives of two users must be seen as diff archs
		var srcName, dstName string
		if srcType == "google" {
			_, srcName, _, _ = drives.ParseGoogleDrivePath(src)
		} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			_, srcName, _ = drives.ParseCloudDrivePath(src, true)
		}
		if dstType == "google" {
			_, dstName, _, _ = drives.ParseGoogleDrivePath(dst)
		} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
			_, dstName, _ = drives.ParseCloudDrivePath(dst, true)
		}
		if srcName != dstName {
			same = false
		}

		if same {
			err = pasteActionSameArch(r.Context(), action, srcType, src, dstType, dst, d, fileCache, override, rename, w, r)
		} else {
			err = pasteActionDiffArch(r.Context(), action, srcType, src, dstType, dst, d, fileCache, w, r)
		}
		if common.ErrToStatus(err) == http.StatusRequestEntityTooLarge {
			fmt.Fprintln(w, err.Error())
		}
		return common.ErrToStatus(err), err
	}
}

func getStat(fs afero.Fs, srcType, src string, w http.ResponseWriter, r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	// we need only size, fileMode and isDir for the time being for all arch
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}

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
		metaInfo, err := drives.GetGoogleDriveIdFocusedMetaInfos(src, w, r)
		if err != nil {
			return nil, 0, 0, false, err
		}
		return nil, metaInfo.Size, 0755, metaInfo.IsDir, nil
	} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		src = strings.TrimSuffix(src, "/")
		metaInfo, err := drives.GetCloudDriveFocusedMetaInfos(src, w, r)
		if err != nil {
			return nil, 0, 0, false, err
		}
		return nil, metaInfo.Size, 0755, metaInfo.IsDir, nil
	} else if srcType == "cache" {
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
	} else if srcType == "sync" {
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
			mode := drives.SyncPermToMode(fileInfo.Permission)
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
func copyDir(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, fileMode os.FileMode, w http.ResponseWriter,
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
		if err := fs.MkdirAll(dst, mode); err != nil {
			return err
		}
		if err := fileutils.Chown(fs, dst, 1000, 1000); err != nil {
			klog.Errorf("can't chown directory %s to user %d: %s", dst, 1000, err)
			return err
		}
	} else if dstType == "google" {
		respBody, _, err := drives.ResourcePostGoogle(dst, w, r, true)
		var bodyJson drives.GoogleDrivePostResponse
		if err = json.Unmarshal(respBody, &bodyJson); err != nil {
			klog.Error(err)
			return err
		}
		driveIdCache[src] = bodyJson.Data.Meta.ID
		if err != nil {
			return err
		}
	} else if dstType == "cloud" || dstType == "awss3" || dstType == "tencent" || dstType == "dropbox" {
		_, _, err := drives.ResourcePostCloudDrive(dst, w, r, false)
		if err != nil {
			return err
		}
	} else if dstType == "cache" {
		if err := drives.CacheMkdirAll(dst, fileMode, r); err != nil {
			return err
		}
		// cache chown in the function of itself
	} else if dstType == "sync" {
		if err := drives.SyncMkdirAll(dst, fileMode, true, r); err != nil {
			return err
		}
		// sync doesn't need to chown
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

		srcDrive, srcName, pathId, _ := drives.ParseGoogleDrivePath(src)

		param := drives.GoogleDriveListParam{
			Path:  pathId,
			Drive: srcDrive,
			Name:  srcName,
		}

		jsonBody, err := json.Marshal(param)
		if err != nil {
			klog.Errorln("Error marshalling JSON:", err)
			return err
		}
		klog.Infoln("Google Drive List Params:", string(jsonBody))
		var respBody []byte
		respBody, err = drives.GoogleDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
		if err != nil {
			klog.Errorln("Error calling drive/ls:", err)
			return err
		}
		var bodyJson drives.GoogleDriveListResponse
		if err = json.Unmarshal(respBody, &bodyJson); err != nil {
			klog.Error(err)
			return err
		}
		for _, item := range bodyJson.Data {
			fsrc := filepath.Dir(strings.TrimSuffix(src, "/")) + "/" + item.Meta.ID
			fdst := filepath.Join(fdstBase, item.Name)
			klog.Infoln(fsrc, fdst)
			if item.IsDir {
				err = copyDir(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), w, r, driveIdCache)
				if err != nil {
					return err
				}
			} else {
				fdst += item.ExportSuffix
				err = copyFile(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(0755), item.FileSize, w, r, driveIdCache)
				if err != nil {
					return err
				}
			}
		}
	} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		srcDrive, srcName, srcPath := drives.ParseCloudDrivePath(src, true)

		param := drives.CloudDriveListParam{
			Path:  srcPath,
			Drive: srcDrive,
			Name:  srcName,
		}

		jsonBody, err := json.Marshal(param)
		if err != nil {
			klog.Errorln("Error marshalling JSON:", err)
			return err
		}
		klog.Infoln("Cloud Drive List Params:", string(jsonBody))
		var respBody []byte
		respBody, err = drives.CloudDriveCall("/drive/ls", "POST", jsonBody, w, r, true)
		if err != nil {
			klog.Errorln("Error calling drive/ls:", err)
			return err
		}
		var bodyJson drives.CloudDriveListResponse
		if err = json.Unmarshal(respBody, &bodyJson); err != nil {
			klog.Error(err)
			return err
		}
		for _, item := range bodyJson.Data {
			fsrc := filepath.Join(src, item.Name)
			fdst := filepath.Join(fdstBase, item.Name)
			klog.Infoln(fsrc, fdst)
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
				err := copyDir(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), w, r, driveIdCache)
				if err != nil {
					return err
				}
			} else {
				err := copyFile(fs, srcType, fsrc, dstType, fdst, d, os.FileMode(item.Mode), item.Size, w, r, driveIdCache)
				if err != nil {
					return err
				}
			}
		}
		return nil
	} else if srcType == "sync" {
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
				err := copyDir(fs, srcType, fsrc, dstType, fdst, d, drives.SyncPermToMode(item.Permission), w, r, driveIdCache)
				if err != nil {
					return err
				}
			} else {
				err := copyFile(fs, srcType, fsrc, dstType, fdst, d, drives.SyncPermToMode(item.Permission), item.Size, w, r, driveIdCache)
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
func copyFile(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, mode os.FileMode, diskSize int64,
	w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return os.ErrPermission
	}

	extRemains := dstType == "google" || dstType == "cloud" || dstType == "awss3" || dstType == "tencent" || dstType == "dropbox"
	var bufferPath string
	// copy/move
	if srcType == "drive" {
		fileInfo, status, err := drives.ResourceDriveGetInfo(src, r, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		diskSize = fileInfo.Size
		_, err = common.CheckBufferDiskSpace(diskSize)
		if err != nil {
			return err
		}
		bufferPath, err = common.GenerateBufferFileName(src, bflName, extRemains)
		if err != nil {
			return err
		}

		err = common.MakeDiskBuffer(bufferPath, diskSize, false)
		if err != nil {
			return err
		}
		err = drives.DriveFileToBuffer(fileInfo, bufferPath)
		if err != nil {
			return err
		}
	} else if srcType == "google" {
		var err error
		_, err = common.CheckBufferDiskSpace(diskSize)
		if err != nil {
			return err
		}

		srcInfo, err := drives.GetGoogleDriveIdFocusedMetaInfos(src, w, r)
		bufferFilePath, err := common.GenerateBufferFolder(srcInfo.Path, bflName)
		if err != nil {
			return err
		}
		bufferFileName := common.RemoveSlash(srcInfo.Name) + srcInfo.ExportSuffix
		bufferPath = filepath.Join(bufferFilePath, bufferFileName)
		klog.Infoln("Buffer file path: ", bufferFilePath)
		klog.Infoln("Buffer path: ", bufferPath)
		err = common.MakeDiskBuffer(bufferPath, diskSize, true)
		if err != nil {
			return err
		}
		_, err = drives.GoogleFileToBuffer(src, bufferFilePath, bufferFileName, w, r)
		if err != nil {
			return err
		}
	} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		var err error
		_, err = common.CheckBufferDiskSpace(diskSize)
		if err != nil {
			return err
		}

		srcInfo, err := drives.GetCloudDriveFocusedMetaInfos(src, w, r)
		bufferFilePath, err := common.GenerateBufferFolder(srcInfo.Path, bflName)
		if err != nil {
			return err
		}
		bufferPath = filepath.Join(bufferFilePath, srcInfo.Name)
		klog.Infoln("Buffer file path: ", bufferFilePath)
		klog.Infoln("Buffer path: ", bufferPath)
		err = common.MakeDiskBuffer(bufferPath, diskSize, true)
		if err != nil {
			return err
		}
		err = drives.CloudDriveFileToBuffer(src, bufferFilePath, w, r)
		if err != nil {
			return err
		}
	} else if srcType == "cache" {
		var err error
		_, err = common.CheckBufferDiskSpace(diskSize)
		if err != nil {
			return err
		}
		bufferPath, err = common.GenerateBufferFileName(src, bflName, extRemains)
		if err != nil {
			return err
		}

		err = common.MakeDiskBuffer(bufferPath, diskSize, false)
		if err != nil {
			return err
		}
		err = drives.CacheFileToBuffer(src, bufferPath)
		if err != nil {
			return err
		}
	} else if srcType == "sync" {
		var err error
		_, err = common.CheckBufferDiskSpace(diskSize)
		if err != nil {
			return err
		}
		bufferPath, err = common.GenerateBufferFileName(src, bflName, extRemains)
		if err != nil {
			return err
		}

		err = common.MakeDiskBuffer(bufferPath, diskSize, false)
		if err != nil {
			return err
		}
		err = drives.SyncFileToBuffer(src, bufferPath, r)
		if err != nil {
			return err
		}
	}

	rename := r.URL.Query().Get("rename") == "true"
	if rename && dstType != "google" && srcType == "google" {
		dst = pasteAddVersionSuffix(dst, dstType, files.DefaultFs, w, r)
	}

	// paste
	if dstType == "drive" {
		status, err := drives.DriveBufferToFile(bufferPath, dst, mode, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		common.RemoveDiskBuffer(bufferPath, srcType)
	} else if dstType == "google" {
		klog.Infoln("Begin to paste!")
		klog.Infoln("dst: ", dst)
		status, err := drives.GoogleBufferToFile(bufferPath, dst, w, r)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		klog.Infoln("Begin to remove buffer")
		common.RemoveDiskBuffer(bufferPath, srcType)
	} else if dstType == "cloud" || dstType == "awss3" || dstType == "tencent" || dstType == "dropbox" {
		klog.Infoln("Begin to paste!")
		klog.Infoln("dst: ", dst)
		status, err := drives.CloudDriveBufferToFile(bufferPath, dst, w, r)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		klog.Infoln("Begin to remove buffer")
		common.RemoveDiskBuffer(bufferPath, srcType)
	} else if dstType == "cache" {
		status, err := drives.CacheBufferToFile(bufferPath, dst, mode, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		common.RemoveDiskBuffer(bufferPath, srcType)
	} else if dstType == "sync" {
		klog.Infoln("Begin to sync paste!")
		if err := drives.SyncMkdirAll(dst, mode, false, r); err != nil {
			return err
		}
		status, err := drives.SyncBufferToFile(bufferPath, dst, diskSize, r)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			klog.Errorln("Sync paste failed! err: ", err)
			return err
		}
		klog.Infoln("Begin to remove buffer")
		common.RemoveDiskBuffer(bufferPath, srcType)
	}
	return nil
}

func doPaste(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, w http.ResponseWriter, r *http.Request) error {
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

func moveDelete(fileCache fileutils.FileCache, srcType, src string, ctx context.Context, d *common.Data, w http.ResponseWriter, r *http.Request) error {
	if srcType == "drive" {
		status, err := drives.ResourceDriveDelete(fileCache, src, ctx, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	} else if srcType == "google" {
		_, status, err := drives.ResourceDeleteGoogle(fileCache, src, w, r, true)
		if status != http.StatusOK && status != 0 {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		_, status, err := drives.ResourceDeleteCloudDrive(fileCache, src, w, r, true)
		if status != http.StatusOK && status != 0 {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	} else if srcType == "cache" {
		status, err := drives.ResourceCacheDelete(fileCache, src, ctx, d)
		if status != http.StatusOK {
			return os.ErrInvalid
		}
		if err != nil {
			return err
		}
		return nil
	} else if srcType == "sync" {
		status, err := drives.ResourceSyncDelete(src, r)
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

func pasteActionSameArch(ctx context.Context, action, srcType, src, dstType, dst string, d *common.Data, fileCache fileutils.FileCache, override, rename bool, w http.ResponseWriter, r *http.Request) error {
	klog.Infoln("Now deal with ", action, " for same arch ", dstType)
	klog.Infoln("src: ", src, ", dst: ", dst, ", override: ", override)
	if srcType == "drive" || srcType == "cache" {
		patchUrl := "http://127.0.0.1:80/api/resources/" + common.EscapeURLWithSpace(strings.TrimLeft(src, "/")) + "?action=" + action + "&destination=" + common.EscapeURLWithSpace(dst) + "&override=" + strconv.FormatBool(override) + "&rename=" + strconv.FormatBool(rename)
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
	} else if srcType == "google" {
		switch action {
		case "copy":
			if !strings.HasSuffix(src, "/") {
				src += "/"
			}
			metaInfo, err := drives.GetGoogleDriveIdFocusedMetaInfos(src, w, r)
			if err != nil {
				return err
			}

			if metaInfo.IsDir {
				return drives.CopyGoogleDriveFolder(src, dst, w, r, metaInfo.Path)
			}
			return drives.CopyGoogleDriveSingleFile(src, dst, w, r)
		case "rename":
			if !strings.HasSuffix(src, "/") {
				src += "/"
			}
			return drives.MoveGoogleDriveFolderOrFiles(src, dst, w, r)
		}
	} else if srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		switch action {
		case "copy":
			if strings.HasSuffix(src, "/") {
				src = strings.TrimSuffix(src, "/")
			}
			metaInfo, err := drives.GetCloudDriveFocusedMetaInfos(src, w, r)
			if err != nil {
				return err
			}

			if metaInfo.IsDir {
				return drives.CopyCloudDriveFolder(src, dst, w, r, metaInfo.Path, metaInfo.Name)
			}
			return drives.CopyCloudDriveSingleFile(src, dst, w, r)
		case "rename":
			if !strings.HasSuffix(src, "/") {
				src += "/"
			}
			return drives.MoveCloudDriveFolderOrFiles(src, dst, w, r)
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

		// It seems that we can't mkdir althrough when using sync-bacth-copy/move-item, so we must use false for isDir here.
		if err := drives.SyncMkdirAll(dst, 0, false, r); err != nil {
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
	return nil
}

func pasteActionDiffArch(ctx context.Context, action, srcType, src, dstType, dst string, d *common.Data, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	// In this function, context if tied up to src, because src is in the URL
	switch action {
	case "copy":
		return doPaste(files.DefaultFs, srcType, src, dstType, dst, d, w, r)
	case "rename":
		err := doPaste(files.DefaultFs, srcType, src, dstType, dst, d, w, r)
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
