package drives

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/json"
	e "errors"
	"files/pkg/common"
	"files/pkg/errors"
	"files/pkg/fileutils"
	"files/pkg/preview"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/afero"
	"gorm.io/gorm"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func RemoveAdditionalHeaders(header *http.Header) {
	//header.Del("Traceparent")
	//header.Del("Tracestate")
	return
}

type SyncResourceService struct {
	BaseResourceService
}

func (rc *SyncResourceService) GetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	klog.Infof("Request headers: %v", r.Header)

	streamStr := r.URL.Query().Get("stream")
	stream := 0
	var err error
	if streamStr != "" {
		stream, err = strconv.Atoi(streamStr)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	src := strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync)
	src, err = common.UnescapeURLIfEscaped(src)
	if err != nil {
		return http.StatusBadRequest, err
	}
	klog.Infof("r.URL.Path: %s, src Path: %s", r.URL.Path, src)

	if src == "/" {
		// this is for "/sync/" which is listing all repos of mine
		getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/?type=mine"
		klog.Infoln(getUrl)

		_, respBody, err := syncCall(getUrl, "GET", nil, nil, r, nil, true)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, err = w.Write(respBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return common.ErrToStatus(err), err
		}
		return 0, nil
	}

	if !strings.HasPrefix(r.URL.Path, "/"+SrcTypeSync) || strings.HasSuffix(src, "/") {
		src = strings.Trim(src, "/") + "/"
		repoID, prefix, _ := ParseSyncPath(src)

		getUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace(prefix) + "&with_thumbnail=true"
		klog.Infoln(getUrl)

		request, err := http.NewRequest("GET", getUrl, nil)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		request.Header = r.Header.Clone()
		RemoveAdditionalHeaders(&request.Header)

		client := http.Client{}
		response, err := client.Do(request)
		if err != nil {
			return common.ErrToStatus(err), err
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			return response.StatusCode, nil
		}

		// SSE
		if stream == 1 {
			var body []byte
			respReader := SuitableResponseReader(response)
			if respReader == nil {
				return http.StatusBadRequest, nil
			}
			defer respReader.Close()

			body, err = ioutil.ReadAll(respReader)
			streamSyncDirents(w, r, body, repoID)
			return 0, nil
		}

		// non-SSE
		respReader := SuitableResponseReader(response)
		if respReader == nil {
			return http.StatusBadRequest, nil
		}
		defer respReader.Close()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, err = io.Copy(w, respReader)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return common.ErrToStatus(err), err
		}
		return 0, nil
	}

	// file
	repoID, prefix, filename := ParseSyncPath(src)
	getUrl := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file" + common.EscapeURLWithSpace(prefix) + common.EscapeURLWithSpace(filename) + "?dict=1"
	klog.Infoln(getUrl)

	_, respBody, err := syncCall(getUrl, "GET", nil, nil, r, nil, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(respBody)
	//_, err = io.Copy(w, respBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return common.ErrToStatus(err), err
	}
	return 0, nil
}

func (rc *SyncResourceService) DeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		src := strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync)
		repoID, prefix, filename := ParseSyncPath(src)
		if repoID != "" && prefix == "/" && filename == "" {
			// this is for deleting a repos, or else, prefix must not be "/"
			deleteUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/"
			klog.Infoln(deleteUrl)

			statusCode, deleteBody, err := syncCall(deleteUrl, "DELETE", nil, nil, r, nil, true)
			if err != nil {
				return common.ErrToStatus(err), err
			}

			klog.Infoln("Status Code: ", statusCode)
			if statusCode != http.StatusOK {
				klog.Infoln(string(deleteBody))
				return statusCode, fmt.Errorf("file update failed, status code: %d", statusCode)
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, err = w.Write(deleteBody)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return common.ErrToStatus(err), err
			}
			return 0, nil
		}
		return ResourceSyncDelete(strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync), w, r)
	}
}

func (rc *SyncResourceService) PostHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	src := strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync)
	repoID, prefix, filename := ParseSyncPath(src)
	if repoID != "" && prefix == "/" && filename == "" {
		// this is for creating a repos, or else, prefix must not be "/"
		postUrl := "http://127.0.0.1:80/seahub/api2/repos/?from=web"
		klog.Infoln(postUrl)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		_ = writer.WriteField("name", repoID)
		_ = writer.WriteField("passwd", "")
		klog.Infoln("name", repoID)

		if err := writer.Close(); err != nil {
			return common.ErrToStatus(err), err
		}

		header := r.Header.Clone()
		header.Set("Content-Type", writer.FormDataContentType())
		statusCode, postBody, err := syncCall(postUrl, "POST", body.Bytes(), nil, r, &header, true)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		klog.Infoln("Status Code: ", statusCode)
		if statusCode != http.StatusOK {
			klog.Infoln(string(postBody))
			return statusCode, fmt.Errorf("file update failed, status code: %d", statusCode)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, err = w.Write(postBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return common.ErrToStatus(err), err
		}
		return 0, nil
	}

	err := SyncMkdirAll(strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync), 0, true, r)
	return common.ErrToStatus(err), err
}

func (rc *SyncResourceService) PutHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	// Only allow PUT for files.
	var err error
	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	src := strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync)
	src, err = common.UnescapeURLIfEscaped(src)
	if err != nil {
		return http.StatusBadRequest, err
	}
	klog.Infoln("src Path:", src)

	repoID, prefix, filename := ParseSyncPath(src)
	getUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/update-link/?p=/"
	klog.Infoln(getUrl)

	_, getRespBody, err := syncCall(getUrl, "GET", nil, nil, r, nil, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	updateLink := string(getRespBody)
	updateLink = strings.Trim(updateLink, "\"")

	updateUrl := "http://127.0.0.1:80/seahub/" + updateLink
	klog.Infoln(updateUrl)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("target_file", prefix+filename)
	_ = writer.WriteField("filename", filename)
	klog.Infoln("target_file", prefix+filename)

	fileWriter, err := writer.CreateFormFile("files_content", filename)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	if _, err = fileWriter.Write(bodyBytes); err != nil {
		return common.ErrToStatus(err), err
	}

	if err = writer.Close(); err != nil {
		return common.ErrToStatus(err), err
	}

	header := r.Header.Clone()
	header.Set("Content-Type", writer.FormDataContentType())
	statusCode, postBody, err := syncCall(updateUrl, "POST", body.Bytes(), nil, r, &header, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	klog.Infoln("Status Code: ", statusCode)
	if statusCode != http.StatusOK {
		klog.Infoln(string(postBody))
		return statusCode, fmt.Errorf("file update failed, status code: %d", statusCode)
	}
	return 0, nil
}

func (rc *SyncResourceService) PatchHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		// only for rename
		src := strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync)
		dst := strings.TrimPrefix(r.URL.Query().Get("destination"), "/"+SrcTypeSync)

		action := r.URL.Query().Get("action")
		if action != "rename" {
			return http.StatusBadRequest, nil
		}
		var err error
		src, err = common.UnescapeURLIfEscaped(src)
		if err != nil {
			return common.ErrToStatus(err), err
		}
		dst, err = common.UnescapeURLIfEscaped(dst)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		repoID, prefix, filename := ParseSyncPath(src)
		if repoID != "" && prefix == "/" && filename == "" {
			// this is for renaming a repos, or else, prefix must not be "/"
			postUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/?op=rename"
			klog.Infoln(postUrl)

			repoName, _, _ := ParseSyncPath(dst)

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			_ = writer.WriteField("repo_name", repoName)
			klog.Infoln("repo_name", repoName)

			if err := writer.Close(); err != nil {
				return common.ErrToStatus(err), err
			}

			header := r.Header.Clone()
			header.Set("Content-Type", writer.FormDataContentType())
			statusCode, postBody, err := syncCall(postUrl, "POST", body.Bytes(), nil, r, &header, true)
			if err != nil {
				return common.ErrToStatus(err), err
			}

			klog.Infoln("Status Code: ", statusCode)
			if statusCode != http.StatusOK {
				klog.Infoln(string(postBody))
				return statusCode, fmt.Errorf("file update failed, status code: %d", statusCode)
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, err = w.Write(postBody)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return common.ErrToStatus(err), err
			}
			return 0, nil
		}

		statusCode, respBody, err := ResourceSyncPatch(action, src, dst, r)
		if err != nil {
			return common.ErrToStatus(err), err
		}
		// sync will return a 404 when success
		klog.Infof("file rename failed, status code: %d, fail reason: %s", statusCode, string(respBody))
		if statusCode != http.StatusOK && statusCode != http.StatusNotFound {
			return statusCode, fmt.Errorf("file rename failed, status code: %d, fail reason: %s", statusCode, string(respBody))
		}
		return 0, nil
	}
}

func (rs *SyncResourceService) RawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	src := strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync)
	if src == "" {
		return http.StatusBadRequest, fmt.Errorf("empty source path")
	}
	repoID, prefix, filename := ParseSyncPath(src)
	if strings.HasSuffix(r.URL.Path, "/") {
		zipTaskUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/zip-task/"

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fields := []struct {
			key   string
			value string
		}{
			{"parent_dir", path.Dir(strings.TrimSuffix(prefix, "/"))},
			{"dirents", path.Base(strings.TrimSuffix(prefix, "/"))},
		}

		for _, field := range fields {
			if err := writer.WriteField(field.key, field.value); err != nil {
				return http.StatusInternalServerError, fmt.Errorf("write field failed: %s=%s - %w", field.key, field.value, err)
			}
		}

		if err := writer.Close(); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("writer close failed: %w", err)
		}

		header := r.Header.Clone()
		header.Set("Content-Type", writer.FormDataContentType())

		_, zipTaskRespBody, err := syncCall(zipTaskUrl, "POST", body.Bytes(), nil, r, &header, true)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		var zipTaskRespData map[string]interface{}
		if err = json.Unmarshal(zipTaskRespBody, &zipTaskRespData); err != nil {
			return common.ErrToStatus(err), err
		}
		klog.Infof("zipTaskRespData: %v", zipTaskRespData)

		var zipToken string
		var ok bool
		if zipToken, ok = zipTaskRespData["zip_token"].(string); ok {
			klog.Infoln("Extracted Zip Token:", zipToken)
		} else {
			klog.Infoln("Zip Token not found or invalid type")
			return http.StatusBadRequest, e.New("Zip Token not found or invalid type")
		}

		time.Sleep(1 * time.Second) // the most important

		zipUrl := "http://127.0.0.1:80/seafhttp/zip/" + zipToken
		klog.Infoln(zipUrl)

		request, err := http.NewRequest("GET", zipUrl, nil)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		request.Header = r.Header.Clone()
		klog.Infof("~~~Debug Log: reqeust.Header=%v", request.Header)
		RemoveAdditionalHeaders(&request.Header)

		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if response.StatusCode != http.StatusOK {
			return response.StatusCode, fmt.Errorf("unexpected status code from ZIP endpoint: %d", response.StatusCode)
		}
		defer func() {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}()

		klog.Infof("Response Status: %d", response.StatusCode)
		klog.Infof("ZIP Response Headers: %v", response.Header)

		reader := SuitableResponseReader(response)

		zipFilename := path.Base(strings.TrimSuffix(prefix, "/")) + ".zip"
		safeZipFilename := url.QueryEscape(zipFilename)
		safeZipFilename = strings.ReplaceAll(safeZipFilename, "+", "%20")
		klog.Infof("~~~Debug Log: zipFilename=%s, safeZipFilename=%s", zipFilename, safeZipFilename)

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", zipFilename, safeZipFilename))
		w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
		w.Header().Set("Content-Length", response.Header.Get("Content-Length"))

		_, err = io.Copy(w, reader)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		return response.StatusCode, nil
	}

	dlUrl := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file" + common.EscapeAndJoin(prefix+filename, "/") + "/" + "?dl=1"
	klog.Infof("redirect url: %s", dlUrl)

	request, err := http.NewRequest("GET", dlUrl, nil)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	request.Header = r.Header.Clone() // 透传客户端请求头
	klog.Infof("~~~Debug Log: reqeust.Header=%v", request.Header)
	RemoveAdditionalHeaders(&request.Header)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer response.Body.Close()

	klog.Infof("~~~Debug Log: response.Header=%v", response.Header)
	reader := SuitableResponseReader(response)

	safeFilename := url.QueryEscape(filename)
	safeFilename = strings.ReplaceAll(safeFilename, "+", "%20")

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", filename, safeFilename))
	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.Header().Set("Content-Length", response.Header.Get("Content-Length"))

	_, err = io.Copy(w, reader)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return response.StatusCode, nil
}

func (rs *SyncResourceService) PreviewHandler(imgSvc preview.ImgService, fileCache fileutils.FileCache, enableThumbnails, resizePreview bool) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		vars := mux.Vars(r)

		previewSize, err := preview.ParsePreviewSize(vars["size"])
		if err != nil {
			return http.StatusBadRequest, err
		}
		previewSizeStr := ""
		if previewSize == 0 {
			previewSizeStr = "/128"
		} else if previewSize == 1 {
			previewSizeStr = "/1080"
		} else {
			return http.StatusBadRequest, err
		}

		path := "/" + vars["path"]
		path = strings.TrimPrefix(path, "/"+SrcTypeSync)
		if path == "" || strings.HasSuffix(path, "/") {
			return http.StatusBadRequest, fmt.Errorf("empty source path")
		}
		repoID, prefix, filename := ParseSyncPath(path)
		previewUrl := "http://127.0.0.1:80/seahub/thumbnail/" + repoID + previewSizeStr + common.EscapeAndJoin(prefix+filename, "/")
		status, previewRespBody, err := syncCall(previewUrl, "GET", nil, nil, r, nil, true)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(previewRespBody)))
		w.Write(previewRespBody)
		return status, nil
	}
}

func (rc *SyncResourceService) PasteSame(action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)
	dst = strings.TrimPrefix(dst, "/"+SrcTypeSync)
	return PasteSyncPatch(action, src, dst, r)
}

func (rs *SyncResourceService) PasteDirFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	mode := fileMode
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)

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
		fdstBase = filepath.Join(filepath.Dir(filepath.Dir(strings.TrimSuffix(dst, "/"))), driveIdCache[src])
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

	request.Header = r.Header.Clone()
	RemoveAdditionalHeaders(&request.Header)

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

	var data DirentResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return err
	}

	for _, item := range data.DirentList {
		fsrc := filepath.Join(src, item.Name)
		fdst := filepath.Join(fdstBase, item.Name)

		if item.Type == "dir" {
			err := rs.PasteDirFrom(fs, srcType, fsrc, dstType, fdst, d, SyncPermToMode(item.Permission), w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			err := rs.PasteFileFrom(fs, srcType, fsrc, dstType, fdst, d, SyncPermToMode(item.Permission), item.Size, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *SyncResourceService) PasteDirTo(fs afero.Fs, src, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	dst = strings.TrimPrefix(dst, "/"+SrcTypeSync)
	if err := SyncMkdirAll(dst, fileMode, true, r); err != nil {
		return err
	}
	return nil
}

func (rs *SyncResourceService) PasteFileFrom(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)
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
	err = SyncFileToBuffer(src, bufferPath, r)
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

func (rs *SyncResourceService) PasteFileTo(fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	klog.Infoln("Begin to sync paste!")
	dst = strings.TrimPrefix(dst, "/"+SrcTypeSync)
	if err := SyncMkdirAll(dst, fileMode, false, r); err != nil {
		return err
	}
	status, err := SyncBufferToFile(bufferPath, dst, diskSize, r)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		klog.Errorln("Sync paste failed! err: ", err)
		return err
	}
	return nil
}

func (rs *SyncResourceService) GetStat(fs afero.Fs, src string, w http.ResponseWriter,
	r *http.Request) (os.FileInfo, int64, os.FileMode, bool, error) {
	src, err := common.UnescapeURLIfEscaped(src)
	if err != nil {
		return nil, 0, 0, false, err
	}
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)

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

	request.Header = r.Header.Clone()
	RemoveAdditionalHeaders(&request.Header)

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

	var dirResp DirentResponse
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
		mode := SyncPermToMode(fileInfo.Permission)
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

func (rs *SyncResourceService) MoveDelete(fileCache fileutils.FileCache, src string, ctx context.Context, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)
	status, err := ResourceSyncDelete(src, w, r)
	if status != http.StatusOK {
		return os.ErrInvalid
	}
	if err != nil {
		return err
	}
	return nil
}

func (rs *SyncResourceService) GeneratePathList(db *gorm.DB, rootPath string, processor PathProcessor, recordsStatusProcessor RecordsStatusProcessor) error {
	if rootPath == "" {
		rootPath = "/"
	}

	processedPaths := make(map[string]bool)

	for bflName, cookie := range common.BflCookieCache {
		klog.Infof("Key: %s, Value: %s\n", bflName, cookie)
		repoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/?type=mine"

		header := make(http.Header)

		header.Set("Content-Type", "application/json")
		header.Set("X-Bfl-User", bflName)
		header.Set("Cookie", cookie)

		_, repoRespBody, err := syncCall(repoURL, "GET", nil, nil, nil, &header, true)
		if err != nil {
			klog.Errorf("SyncCall failed: %v\n", err)
			return err
		}

		var data RepoResponse
		err = json.Unmarshal(repoRespBody, &data)
		if err != nil {
			klog.Errorf("unmarshal repo response failed: %v\n", err)
			return err
		}

		for _, repo := range data.Repos {
			klog.Infof("repo=%v", repo)

			url := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repo.RepoID + "/dir/?p=" + rootPath + "&with_thumbnail=true"
			klog.Infoln(url)

			var direntRespBody []byte
			_, direntRespBody, err = syncCall(url, "GET", nil, nil, nil, &header, true)
			if err != nil {
				klog.Errorf("fetch repo response failed: %v\n", err)
				return err
			}

			generator := walkSyncDirentsGenerator(direntRespBody, &header, nil, repo.RepoID)

			for dirent := range generator {
				key := fmt.Sprintf("%s:%s", dirent.Drive, dirent.Path)
				processedPaths[key] = true

				_, err = processor(db, dirent.Drive, dirent.Path, dirent.Mtime)
				if err != nil {
					klog.Errorf("generate path list failed: %v\n", err)
					return err
				}
			}
		}
	}

	err := recordsStatusProcessor(db, processedPaths, []string{SrcTypeSync}, 1)
	if err != nil {
		klog.Errorf("records status processor failed: %v\n", err)
		return err
	}

	return nil
}

// just for complement, no need to use now
func (rs *SyncResourceService) parsePathToURI(path string) (string, string) {
	return SrcTypeSync, path
}

func syncCall(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, header *http.Header, returnResp bool) (int, []byte, error) {
	// w is for future use, not used now
	client := &http.Client{}
	request, err := http.NewRequest(method, dst, bytes.NewBuffer(reqBodyJson))
	if err != nil {
		klog.Errorf("create request failed: %v\n", err)
		return -1, nil, err
	}

	if header != nil {
		request.Header = (*header).Clone()
	} else {
		request.Header = r.Header.Clone()
	}

	RemoveAdditionalHeaders(&request.Header)

	response, err := client.Do(request)
	if err != nil {
		klog.Errorf("request failed: %v\n", err)
		return -1, nil, err
	}
	defer response.Body.Close()

	respReader := SuitableResponseReader(response)
	if respReader == nil {
		return -1, nil, fmt.Errorf("response reader is nil")
	}
	defer respReader.Close()

	body, err := ioutil.ReadAll(respReader)
	if err != nil {
		klog.Errorf("read response failed: %v\n", err)
		return -1, nil, err
	}

	//var bodyReader io.Reader = response.Body
	//
	//if response.Header.Get("Content-Encoding") == "gzip" {
	//	gzipReader, err := gzip.NewReader(response.Body)
	//	if err != nil {
	//		klog.Errorf("unzip response failed: %v\n", err)
	//		return nil, err
	//	}
	//	defer gzipReader.Close()
	//
	//	bodyReader = gzipReader
	//}
	//
	//body, err := ioutil.ReadAll(bodyReader)
	//if err != nil {
	//	klog.Errorf("read response failed: %v\n", err)
	//	return nil, err
	//}

	if returnResp {
		return response.StatusCode, body, nil
	}
	return response.StatusCode, nil, nil
}

type Repo struct {
	Type                 string    `json:"type"`
	RepoID               string    `json:"repo_id"`
	RepoName             string    `json:"repo_name"`
	OwnerEmail           string    `json:"owner_email"`
	OwnerName            string    `json:"owner_name"`
	OwnerContactEmail    string    `json:"owner_contact_email"`
	LastModified         time.Time `json:"last_modified"`
	ModifierEmail        string    `json:"modifier_email"`
	ModifierName         string    `json:"modifier_name"`
	ModifierContactEmail string    `json:"modifier_contact_email"`
	Size                 int       `json:"size"`
	Encrypted            bool      `json:"encrypted"`
	Permission           string    `json:"permission"`
	Starred              bool      `json:"starred"`
	Monitored            bool      `json:"monitored"`
	Status               string    `json:"status"`
	Salt                 string    `json:"salt"`
}

type RepoResponse struct {
	Repos []Repo `json:"repos"`
}

func ParseSyncPath(src string) (string, string, string) {
	// prefix is with suffix "/" and prefix "/" while repo_id and filename don't have any prefix and suffix
	src = strings.TrimPrefix(src, "/")

	firstSlashIdx := strings.Index(src, "/")

	repoID := src[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(src, "/")

	filename := src[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}
	if prefix == "" {
		prefix = "/"
	}

	// for url with additional "/" or lack of "/"
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	klog.Infoln("repo-id:", repoID)
	klog.Infoln("prefix:", prefix)
	klog.Infoln("filename:", filename)
	return repoID, prefix, filename
}

type Dirent struct {
	Type                 string `json:"type"`
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Mtime                int64  `json:"mtime"`
	Permission           string `json:"permission"`
	ParentDir            string `json:"parent_dir"`
	Size                 int64  `json:"size"`
	FileSize             int64  `json:"fileSize,omitempty"`
	NumTotalFiles        int    `json:"numTotalFiles,omitempty"`
	NumFiles             int    `json:"numFiles,omitempty"`
	NumDirs              int    `json:"numDirs,omitempty"`
	Path                 string `json:"path"`
	Starred              bool   `json:"starred"`
	ModifierEmail        string `json:"modifier_email,omitempty"`
	ModifierName         string `json:"modifier_name,omitempty"`
	ModifierContactEmail string `json:"modifier_contact_email,omitempty"`
}

type DirentResponse struct {
	UserPerm   string   `json:"user_perm"`
	DirID      string   `json:"dir_id"`
	DirentList []Dirent `json:"dirent_list"`
	sync.Mutex
}

func walkSyncDirentsGenerator(body []byte, header *http.Header, r *http.Request, repoID string) <-chan DirentGeneratedEntry {
	ch := make(chan DirentGeneratedEntry)
	go func() {
		defer close(ch)

		var bodyJson DirentResponse
		if err := json.Unmarshal(body, &bodyJson); err != nil {
			klog.Error(err)
			return
		}

		queue := make([]Dirent, 0)
		bodyJson.Lock()
		queue = append(queue, bodyJson.DirentList...)
		bodyJson.Unlock()

		for len(queue) > 0 {
			firstItem := queue[0]
			queue = queue[1:]

			if firstItem.Type == "dir" {
				fullPath := filepath.Join(SrcTypeSync, repoID, firstItem.Path, firstItem.Name)
				klog.Infof("~~~Temp log: sync fullPath = %s", fullPath)
				entry := DirentGeneratedEntry{
					Drive: SrcTypeSync,
					Path:  fullPath,
					Mtime: time.Unix(firstItem.Mtime, 0),
				}
				ch <- entry

				path := firstItem.Path
				if path != "/" {
					path += "/"
				}
				path = common.EscapeURLWithSpace(path)
				firstUrl := fmt.Sprintf("http://127.0.0.1:80/seahub/api/v2.1/repos/%s/dir/?p=%s&with_thumbnail=true", repoID, path)
				klog.Infoln(firstUrl)

				_, firstRespBody, err := syncCall(firstUrl, "GET", nil, nil, r, header, true)
				if err != nil {
					klog.Error(err)
					continue
				}

				var firstBodyJson DirentResponse
				if err := json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
					klog.Error(err)
					continue
				}
				queue = append(queue, firstBodyJson.DirentList...)
			}
		}
	}()
	return ch
}

func generateDirentsData(body []byte, stopChan <-chan struct{}, dataChan chan<- string, r *http.Request, repoID string) {
	defer close(dataChan)

	var bodyJson DirentResponse
	if err := json.Unmarshal(body, &bodyJson); err != nil {
		klog.Error(err)
		return
	}

	var A []Dirent
	bodyJson.Lock()
	A = append(A, bodyJson.DirentList...)
	bodyJson.Unlock()

	for len(A) > 0 {
		klog.Infoln("len(A): ", len(A))
		firstItem := A[0]
		klog.Infoln("firstItem Path: ", firstItem.Path)
		klog.Infoln("firstItem Name:", firstItem.Name)

		if firstItem.Type == "dir" {
			path := firstItem.Path
			if path != "/" {
				path += "/"
			}
			path = common.EscapeURLWithSpace(path)
			firstUrl := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + path + "&with_thumbnail=true"
			klog.Infoln(firstUrl)

			firstRequest, err := http.NewRequest("GET", firstUrl, nil)
			if err != nil {
				klog.Error(err)
				return
			}

			firstRequest.Header = r.Header.Clone()
			RemoveAdditionalHeaders(&firstRequest.Header)

			client := http.Client{}
			firstResponse, err := client.Do(firstRequest)
			if err != nil {
				return
			}

			if firstResponse.StatusCode != http.StatusOK {
				klog.Infoln(firstResponse.StatusCode)
				return
			}

			var firstRespBody []byte
			var reader *gzip.Reader = nil
			if firstResponse.Header.Get("Content-Encoding") == "gzip" {
				reader, err = gzip.NewReader(firstResponse.Body)
				if err != nil {
					klog.Errorln("Error creating gzip reader:", err)
					return
				}

				firstRespBody, err = ioutil.ReadAll(reader)
				if err != nil {
					klog.Errorln("Error reading gzipped response body:", err)
					reader.Close()
					return
				}
			} else {
				firstRespBody, err = ioutil.ReadAll(firstResponse.Body)
				if err != nil {
					klog.Errorln("Error reading response body:", err)
					firstResponse.Body.Close()
					return
				}
			}

			var firstBodyJson DirentResponse
			if err := json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
				klog.Error(err)
				return
			}

			A = append(firstBodyJson.DirentList, A[1:]...)

			if reader != nil {
				reader.Close()
			}
			firstResponse.Body.Close()
		} else {
			dataChan <- formatSSEvent(firstItem)

			A = A[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func streamSyncDirents(w http.ResponseWriter, r *http.Request, body []byte, repoID string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateDirentsData(body, stopChan, dataChan, r, repoID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case event, ok := <-dataChan:
			if !ok {
				return
			}
			_, err := w.Write([]byte(event))
			if err != nil {
				klog.Error(err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			close(stopChan)
			return
		}
	}
}

func SyncMkdirAll(dst string, mode os.FileMode, isDir bool, r *http.Request) error {
	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
		return err
	}

	firstSlashIdx := strings.Index(dst, "/")

	repoID := dst[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(dst, "/")

	prefix := ""
	if isDir {
		prefix = dst[firstSlashIdx+1:]

	} else {
		if firstSlashIdx != lastSlashIdx {
			prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
		}
	}

	client := &http.Client{}

	// Split the prefix by '/' and generate the URLs
	prefixParts := strings.Split(prefix, "/")
	for i := 0; i < len(prefixParts); i++ {
		curPrefix := strings.Join(prefixParts[:i+1], "/")
		curInfoURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace("/"+curPrefix) + "&with_thumbnail=true"
		getRequest, err := http.NewRequest("GET", curInfoURL, nil)
		if err != nil {
			klog.Errorf("create request failed: %v\n", err)
			return err
		}
		getRequest.Header = r.Header.Clone()
		RemoveAdditionalHeaders(&getRequest.Header)
		getResponse, err := client.Do(getRequest)
		if err != nil {
			klog.Errorf("request failed: %v\n", err)
			return err
		}
		defer getResponse.Body.Close()
		if getResponse.StatusCode == 200 {
			continue
		} else {
			klog.Infoln(getResponse.Status)
		}

		type CreateDirRequest struct {
			Operation string `json:"operation"`
		}

		curCreateURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace("/"+curPrefix)

		createDirReq := CreateDirRequest{
			Operation: "mkdir",
		}
		jsonBody, err := json.Marshal(createDirReq)
		if err != nil {
			klog.Errorf("failed to serialize the request body: %v\n", err)
			return err
		}

		request, err := http.NewRequest("POST", curCreateURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			klog.Errorf("create request failed: %v\n", err)
			return err
		}

		request.Header = r.Header.Clone()
		request.Header.Set("Content-Type", "application/json")
		RemoveAdditionalHeaders(&request.Header)

		response, err := client.Do(request)
		if err != nil {
			klog.Errorf("request failed: %v\n", err)
			return err
		}
		defer response.Body.Close()

		// Handle the response as needed
		if response.StatusCode != 200 && response.StatusCode != 201 {
			err = e.New("mkdir failed")
			return err
		}
	}
	return nil
}

func SyncFileToBuffer(src string, bufferFilePath string, r *http.Request) error {
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

	dlUrl := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file/" + common.EscapeURLWithSpace(prefix+filename) + "/" + "?dl=1"

	request, err := http.NewRequest("GET", dlUrl, nil)
	if err != nil {
		return err
	}

	request.Header = r.Header.Clone()
	RemoveAdditionalHeaders(&request.Header)

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

	return nil
}

func generateUniqueIdentifier(relativePath string) string {
	h := md5.New()
	io.WriteString(h, relativePath+time.Now().String())
	return fmt.Sprintf("%x%s", h.Sum(nil), relativePath)
}

func SyncBufferToFile(bufferFilePath string, dst string, size int64, r *http.Request) (int, error) {
	// Step1: deal with URL
	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := e.New("invalid path format: path must contain at least one '/'")
		klog.Errorln("Error:", err)
		return common.ErrToStatus(err), err
	}
	dst, err := common.UnescapeURLIfEscaped(dst)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	firstSlashIdx := strings.Index(dst, "/")

	repoID := dst[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(dst, "/")

	filename := dst[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = dst[firstSlashIdx+1 : lastSlashIdx+1]
	}

	klog.Infoln("dst:", dst)
	klog.Infoln("repo-id:", repoID)
	klog.Infoln("prefix:", prefix)
	klog.Infoln("filename:", filename)

	extension := path.Ext(filename)
	mimeType := "application/octet-stream"
	if extension != "" {
		mimeType = mime.TypeByExtension(extension)
	}

	// step2: GET upload URL
	getUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/upload-link/?p=" + common.EscapeAndJoin("/"+prefix, "/") + "&from=api"
	klog.Infoln(getUrl)

	getRequest, err := http.NewRequest("GET", getUrl, nil)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	getRequest.Header = r.Header.Clone()
	RemoveAdditionalHeaders(&getRequest.Header)

	getClient := http.Client{}
	getResponse, err := getClient.Do(getRequest)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	defer getResponse.Body.Close()

	if getResponse.StatusCode != http.StatusOK {
		err = fmt.Errorf("request failed，status code：%d", getResponse.StatusCode)
		return common.ErrToStatus(err), err
	}

	// Read the response body as a string
	getBody, err := io.ReadAll(getResponse.Body)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	uploadLink := string(getBody)
	uploadLink = strings.Trim(uploadLink, "\"")

	// step3: deal with upload URL
	targetURL := "http://127.0.0.1:80" + uploadLink + "?ret-json=1"
	klog.Infoln(targetURL)

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
	identifier := generateUniqueIdentifier(common.EscapeAndJoin(filename, "/"))

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

		//klog.Infoln("Identifier: ", identifier)
		//klog.Infoln("Parent Dir: ", "/"+prefix)
		//klog.Infoln("resumableChunkNumber: ", strconv.FormatInt(chunkNumber, 10))
		//klog.Infoln("resumableChunkSize: ", strconv.FormatInt(chunkSize, 10))
		//klog.Infoln("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		//klog.Infoln("resumableTotalSize", strconv.FormatInt(size, 10))
		//klog.Infoln("resumableType", mimeType)
		//klog.Infoln("resumableFilename", filename)
		//klog.Infoln("resumableRelativePath", filename)
		//klog.Infoln("resumableTotalChunks", strconv.FormatInt(totalChunks, 10), "\n")

		writer.WriteField("resumableChunkNumber", strconv.FormatInt(chunkNumber, 10))
		writer.WriteField("resumableChunkSize", strconv.FormatInt(chunkSize, 10))
		writer.WriteField("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		writer.WriteField("resumableTotalSize", strconv.FormatInt(size, 10))
		writer.WriteField("resumableType", mimeType)
		writer.WriteField("resumableIdentifier", identifier)
		writer.WriteField("resumableFilename", filename)
		writer.WriteField("resumableRelativePath", filename)
		writer.WriteField("resumableTotalChunks", strconv.FormatInt(totalChunks, 10))
		writer.WriteField("parent_dir", "/"+prefix)

		part, err := writer.CreateFormFile("file", common.EscapeAndJoin(filename, "/"))
		if err != nil {
			klog.Errorln("Create Form File error: ", err)
			return http.StatusInternalServerError, err
		}

		_, err = part.Write(chunkData[:bytesRead])
		if err != nil {
			klog.Errorln("Write Chunk Data error: ", err)
			return http.StatusInternalServerError, err
		}

		err = writer.Close()
		if err != nil {
			klog.Errorln("Write Close error: ", err)
			return http.StatusInternalServerError, err
		}

		request, err := http.NewRequest("POST", targetURL, body)
		if err != nil {
			klog.Errorln("New Request error: ", err)
			return http.StatusInternalServerError, err
		}

		request.Header = r.Header.Clone()
		RemoveAdditionalHeaders(&request.Header)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Content-Disposition", "attachment; filename=\""+common.EscapeAndJoin(filename, "/")+"\"")
		request.Header.Set("Content-Range", "bytes "+strconv.FormatInt(chunkStart, 10)+"-"+strconv.FormatInt(chunkStart+int64(bytesRead)-1, 10)+"/"+strconv.FormatInt(size, 10))
		chunkStart += int64(bytesRead)

		client := http.Client{}
		response, err := client.Do(request)
		klog.Infoln("Do Request")
		if err != nil {
			klog.Errorln("Do Request error: ", err)
			return http.StatusInternalServerError, err
		}
		defer response.Body.Close()

		// Read the response body as a string
		postBody, err := io.ReadAll(response.Body)
		klog.Infoln("ReadAll")
		if err != nil {
			klog.Errorln("ReadAll error: ", err)
			return common.ErrToStatus(err), err
		}

		klog.Infoln("Status Code: ", response.StatusCode)
		if response.StatusCode != http.StatusOK {
			klog.Infoln(string(postBody))
			return response.StatusCode, fmt.Errorf("file upload failed, status code: %d", response.StatusCode)
		}
	}
	klog.Infoln("sync buffer to file success!")
	return 0, nil
}

func ResourceSyncDelete(path string, w http.ResponseWriter, r *http.Request) (int, error) {
	repoID, prefix, filename := ParseSyncPath(path)
	p := prefix + filename
	if strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	var deleteUrl string
	if filename == "" {
		deleteUrl = "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + common.EscapeURLWithSpace(p)
	} else {
		deleteUrl = "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/file/?p=" + common.EscapeURLWithSpace(p)
	}
	klog.Infoln(deleteUrl)

	statusCode, deleteBody, err := syncCall(deleteUrl, "DELETE", nil, nil, r, nil, true)
	if err != nil {
		return common.ErrToStatus(err), err
	}

	klog.Infoln("Status Code: ", statusCode)
	if statusCode != http.StatusOK {
		klog.Infoln(string(deleteBody))
		return statusCode, fmt.Errorf("file update failed, status code: %d", statusCode)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(deleteBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return common.ErrToStatus(err), err
	}
	return 0, nil

	//path = strings.Trim(path, "/")
	//if !strings.Contains(path, "/") {
	//	err := e.New("invalid path format: path must contain at least one '/'")
	//	klog.Errorln("Error:", err)
	//	return common.ErrToStatus(err), err
	//}
	//
	//firstSlashIdx := strings.Index(path, "/")
	//
	//repoID := path[:firstSlashIdx]
	//
	//lastSlashIdx := strings.LastIndex(path, "/")
	//
	//filename := path[lastSlashIdx+1:]
	//
	//prefix := ""
	//if firstSlashIdx != lastSlashIdx {
	//	prefix = path[firstSlashIdx+1 : lastSlashIdx+1]
	//}
	//
	//if prefix != "" {
	//	prefix = "/" + prefix + "/"
	//} else {
	//	prefix = "/"
	//}
	//
	//targetURL := "http://127.0.0.1:80/seahub/api/v2.1/repos/batch-delete-item/"
	//requestBody := map[string]interface{}{
	//	"dirents":    []string{filename},
	//	"parent_dir": prefix,
	//	"repo_id":    repoID,
	//}
	//jsonBody, err := json.Marshal(requestBody)
	//if err != nil {
	//	return http.StatusInternalServerError, err
	//}
	//
	//request, err := http.NewRequest("DELETE", targetURL, bytes.NewBuffer(jsonBody))
	//if err != nil {
	//	return http.StatusInternalServerError, err
	//}
	//
	//request.Header = r.Header.Clone()
	//request.Header.Set("Content-Type", "application/json")
	//RemoveAdditionalHeaders(&request.Header)
	//
	//client := &http.Client{
	//	Timeout: 10 * time.Second,
	//}
	//
	//response, err := client.Do(request)
	//if err != nil {
	//	return http.StatusInternalServerError, err
	//}
	//defer response.Body.Close()
	//
	//if response.StatusCode != http.StatusOK {
	//	return response.StatusCode, fmt.Errorf("file delete failed with status: %d", response.StatusCode)
	//}
	//
	//return 0, nil
}

func SyncPermToMode(permStr string) os.FileMode {
	perm := os.FileMode(0)
	if permStr == "r" {
		perm = perm | 0555
	} else if permStr == "w" {
		perm = perm | 0311
	} else if permStr == "x" {
		perm = perm | 0111
	} else if permStr == "rw" {
		perm = perm | 0755
	} else if permStr == "rx" {
		perm = perm | 0555
	} else if permStr == "wx" {
		perm = perm | 0311
	} else if permStr == "rwx" {
		perm = perm | 0755
	} else {
		klog.Infoln("invalid permission string")
		return 0
	}

	return perm
}

func ResourceSyncPatch(action, src, dst string, r *http.Request) (int, []byte, error) {
	var apiName = "/file/"
	if strings.HasSuffix(src, "/") {
		apiName = "/dir/"
	}

	repoID, prefix, filename := ParseSyncPath(src)
	_, _, newFilename := ParseSyncPath(dst)
	postUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + apiName + "?p=" + common.EscapeAndJoin(prefix+filename, "/")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fields := []struct {
		key   string
		value string
	}{
		{"operation", action},
		{"newname", newFilename},
	}

	for _, field := range fields {
		if err := writer.WriteField(field.key, field.value); err != nil {
			return http.StatusInternalServerError, nil, fmt.Errorf("write field failed: %s=%s - %w", field.key, field.value, err)
		}
	}

	if err := writer.Close(); err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("writer close failed: %w", err)
	}

	header := r.Header.Clone()
	header.Set("Content-Type", writer.FormDataContentType())
	statusCode, respBody, err := syncCall(postUrl, "POST", body.Bytes(), nil, r, &header, true)
	if err != nil {
		return common.ErrToStatus(err), nil, err
	}
	klog.Infof("statusCode: %d, respBody: %s", statusCode, string(respBody))
	return statusCode, respBody, nil
}

func PasteSyncPatch(action, src, dst string, r *http.Request) error {
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
	if err := SyncMkdirAll(dst, 0, false, r); err != nil {
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

	request.Header = r.Header.Clone()
	request.Header.Set("Content-Type", "application/json")
	RemoveAdditionalHeaders(&request.Header)

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
