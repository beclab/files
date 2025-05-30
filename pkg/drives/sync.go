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
	"files/pkg/parser"
	"files/pkg/pool"
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
		return ResourceSyncDelete(strings.TrimPrefix(r.URL.Path, "/"+SrcTypeSync), w, r, true)
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
	request.Header = r.Header.Clone()
	RemoveAdditionalHeaders(&request.Header)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer response.Body.Close()

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
		if status != http.StatusOK {
			return status, nil
		}
		return 0, nil
	}
}

func (rc *SyncResourceService) PasteSame(task *pool.Task, action, src, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	klog.Infof("Request headers: %v", r.Header)

	ctx, cancel := context.WithCancel(context.Background())
	go SimulateProgress(ctx, 0, 99, task.TotalFileSize, 50000000, task)

	src = strings.TrimPrefix(src, "/"+SrcTypeSync)
	dst = strings.TrimPrefix(dst, "/"+SrcTypeSync)

	err := PasteSyncPatch(task, action, src, dst, r)
	cancel()
	if err != nil {
		TaskLog(task, "info", fmt.Sprintf("%s from %s to %s failed", action, src, dst))
		TaskLog(task, "error", err.Error())
		pool.FailTask(task.ID)
	} else {
		TaskLog(task, "info", fmt.Sprintf("%s from %s to %s successfully", action, src, dst))
		pool.CompleteTask(task.ID)
	}
	return err
}

func (rs *SyncResourceService) PasteDirFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	fileMode os.FileMode, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
	mode := fileMode
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)

	handler, err := GetResourceService(dstType)
	if err != nil {
		return err
	}

	err = handler.PasteDirTo(task, fs, src, dst, mode, fileCount, w, r, d, driveIdCache)
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
			err := rs.PasteDirFrom(task, fs, srcType, fsrc, dstType, fdst, d, SyncPermToMode(item.Permission), fileCount, w, r, driveIdCache)
			if err != nil {
				return err
			}
		} else {
			err := rs.PasteFileFrom(task, fs, srcType, fsrc, dstType, fdst, d, SyncPermToMode(item.Permission), item.Size, fileCount, w, r, driveIdCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *SyncResourceService) PasteDirTo(task *pool.Task, fs afero.Fs, src, dst string, fileMode os.FileMode, fileCount int64, w http.ResponseWriter,
	r *http.Request, d *common.Data, driveIdCache map[string]string) error {
	dst = strings.TrimPrefix(dst, "/"+SrcTypeSync)
	if err := SyncMkdirAll(dst, fileMode, true, r); err != nil {
		return err
	}
	return nil
}

func (rs *SyncResourceService) PasteFileFrom(task *pool.Task, fs afero.Fs, srcType, src, dstType, dst string, d *common.Data,
	mode os.FileMode, diskSize int64, fileCount int64, w http.ResponseWriter, r *http.Request, driveIdCache map[string]string) error {
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
	task.AddBuffer(bufferPath)

	defer func() {
		logMsg := fmt.Sprintf("Remove copy buffer")
		TaskLog(task, "info", logMsg)
		RemoveDiskBuffer(task, bufferPath, srcType)
	}()

	err = MakeDiskBuffer(bufferPath, diskSize, false)
	if err != nil {
		return err
	}

	left, mid, right := CalculateProgressRange(task, diskSize)

	err = SyncFileToBuffer(task, src, bufferPath, r, left, mid, diskSize)
	if err != nil {
		return err
	}

	if task.Status == "running" {
		handler, err := GetResourceService(dstType)
		if err != nil {
			return err
		}

		err = handler.PasteFileTo(task, fs, bufferPath, dst, mode, mid, right, w, r, d, diskSize)
		if err != nil {
			return err
		}
	}

	logMsg := fmt.Sprintf("Copy from %s to %s sucessfully!", src, dst)
	TaskLog(task, "info", logMsg)
	return nil
}

func (rs *SyncResourceService) PasteFileTo(task *pool.Task, fs afero.Fs, bufferPath, dst string, fileMode os.FileMode, left, right int, w http.ResponseWriter,
	r *http.Request, d *common.Data, diskSize int64) error {
	klog.Infoln("Begin to sync paste!")
	dst = strings.TrimPrefix(dst, "/"+SrcTypeSync)
	if err := SyncMkdirAll(dst, fileMode, false, r); err != nil {
		return err
	}

	status, err := SyncBufferToFile(task, bufferPath, dst, diskSize, r, left, right)
	if status != http.StatusOK && status != 0 {
		err = fmt.Errorf("copy to %s write error with status: %d", dst, status)
		return err
	}
	if err != nil {
		klog.Errorln("Sync paste failed! err: ", err)
		return err
	}

	task.Mu.Lock()
	task.Transferred += diskSize
	task.Mu.Unlock()
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

func (rs *SyncResourceService) MoveDelete(task *pool.Task, fileCache fileutils.FileCache, src string, d *common.Data,
	w http.ResponseWriter, r *http.Request) error {
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)
	status, err := ResourceSyncDelete(src, w, r, false)
	if status != http.StatusOK && status != 0 {
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

func (rs *SyncResourceService) GetFileCount(fs afero.Fs, src, countType string, w http.ResponseWriter, r *http.Request) (int64, error) {
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)
	var count int64

	repoID, path, filename := ParseSyncPath(src)
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	path = common.EscapeURLWithSpace(path)

	queue := []string{path}

	for len(queue) > 0 {
		currentPath := queue[0]
		queue = queue[1:]

		url := fmt.Sprintf("http://127.0.0.1:80/seahub/api/v2.1/repos/%s/dir/?p=%s&with_thumbnail=true",
			repoID, currentPath)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return 0, err
		}
		req.Header = r.Header
		RemoveAdditionalHeaders(&req.Header)
		client := http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return 0, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
		}

		respBody, err := io.ReadAll(SuitableResponseReader(resp))
		resp.Body.Close()
		if err != nil {
			return 0, err
		}

		var direntResp DirentResponse
		if err := json.Unmarshal(respBody, &direntResp); err != nil {
			return 0, err
		}

		for _, dirent := range direntResp.DirentList {
			if filename != "" && dirent.Name == filename {
				if countType == "size" {
					count += dirent.Size
				} else {
					count++
				}
				return count, nil
			} else if filename == "" {
				if dirent.Type == "dir" {
					dirPath := dirent.Path
					if dirPath != "/" {
						dirPath += "/"
					}
					queue = append(queue, common.EscapeURLWithSpace(dirPath))
				} else {
					if countType == "size" {
						count += dirent.Size
					} else {
						count++
					}
				}
			}
		}
	}
	return count, nil
}

func (rs *SyncResourceService) GetTaskFileInfo(fs afero.Fs, src string, w http.ResponseWriter, r *http.Request) (isDir bool, fileType string, filename string, err error) {
	src = strings.TrimPrefix(src, "/"+SrcTypeSync)
	fileType = ""
	if strings.HasSuffix(src, "/") {
		isDir = true
		filename = path.Base(strings.TrimSuffix(src, "/"))
	} else {
		isDir = false
		filename = path.Base(src)
		fileType = parser.MimeTypeByExtension(filename)
	}
	return isDir, fileType, filename, nil
}

// just for complement, no need to use now
func (rs *SyncResourceService) parsePathToURI(path string) (string, string) {
	return SrcTypeSync, path
}

func syncCall(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, header *http.Header, returnResp bool) (int, []byte, error) {
	// w is for future use, not used now

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := &http.Client{}
	var err error

	var request *http.Request
	if reqBodyJson != nil {
		request, err = http.NewRequestWithContext(ctx, method, dst, bytes.NewBuffer(reqBodyJson))
	} else {
		request, err = http.NewRequestWithContext(ctx, method, dst, nil)
	}
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
		if e.Is(err, context.DeadlineExceeded) {
			klog.Errorln("Request timed out after 30 seconds")
			return -1, nil, fmt.Errorf("request timed out after 30 seconds")
		}
		klog.Errorf("request failed: %v\n", err)
		return -1, nil, err
	}
	defer response.Body.Close()

	select {
	case <-ctx.Done():
		klog.Errorln("Request timed out after 30 seconds")
		return -1, nil, fmt.Errorf("request timed out after 30 seconds")
	default:
	}

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

func SyncFileToBuffer(task *pool.Task, src string, bufferFilePath string, r *http.Request, left, right int, diskSize int64) error {
	select {
	case <-task.Ctx.Done():
		return nil
	default:
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

	dlUrl := "http://127.0.0.1:80/seahub/lib/" + repoID + "/file/" + common.EscapeURLWithSpace(prefix+filename) + "/" + "?dl=1"

	request, err := http.NewRequestWithContext(task.Ctx, "GET", dlUrl, nil)
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

	var reader io.Reader = response.Body
	if response.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(response.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	buf := make([]byte, 32*1024)
	var totalRead int64
	lastProgress := 0
	for {
		select {
		case <-task.Ctx.Done():
			return nil
		default:
		}

		nr, er := reader.Read(buf)
		if nr > 0 {
			if _, err := bufferFile.Write(buf[:nr]); err != nil {
				return err
			}
			totalRead += int64(nr)

			progress := int(float64(totalRead) / float64(diskSize) * 100)
			if progress > 100 {
				progress = 100
			}

			mappedProgress := left + (progress*(right-left))/100

			if mappedProgress < left {
				mappedProgress = left
			} else if mappedProgress > right {
				mappedProgress = right
			}

			task.Mu.Lock()
			if lastProgress != progress {
				task.Log = append(task.Log, fmt.Sprintf("downloaded from seafile %d/%d with progress %d", totalRead, diskSize, progress))
				lastProgress = progress
			}
			task.Progress = mappedProgress
			task.Mu.Unlock()
		}

		if er != nil {
			if er == io.EOF {
				break
			}
			return er
		}
	}

	return nil
}

func generateUniqueIdentifier(relativePath string) string {
	h := md5.New()
	io.WriteString(h, relativePath+time.Now().String())
	return fmt.Sprintf("%x%s", h.Sum(nil), relativePath)
}

func SyncBufferToFile(task *pool.Task, bufferFilePath string, dst string, size int64, r *http.Request, left, right int) (int, error) {
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
		if task.Status != "running" && task.Status != "pending" {
			return 0, nil
		}

		percent := (chunkNumber * 100) / totalChunks
		rangeSize := right - left
		mappedProgress := left + int((percent*int64(rangeSize))/100)
		finalProgress := mappedProgress
		if finalProgress < left {
			finalProgress = left
		} else if finalProgress > right {
			finalProgress = right
		}
		klog.Infof("finalProgress:%d", finalProgress)

		offset := (chunkNumber - 1) * chunkSize
		chunkData := make([]byte, chunkSize)
		bytesRead, err := bufferFile.ReadAt(chunkData, offset)
		if err != nil && err != io.EOF {
			return http.StatusInternalServerError, err
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

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

		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		maxRetries := 3
		var response *http.Response
		special := false

		for retry := 0; retry < maxRetries; retry++ {
			var req *http.Request
			var err error

			if retry == 0 {
				req, err = http.NewRequest(request.Method, request.URL.String(), request.Body)
				if err != nil {
					TaskLog(task, "warning", fmt.Sprintf("create request error: %v", err))
					continue
				}
				req.Header = make(http.Header)
				for k, s := range request.Header {
					req.Header[k] = s
				}
			} else {
				// newBody begin
				offset = (chunkNumber - 1) * chunkSize
				chunkData = make([]byte, chunkSize)
				bytesRead, err = bufferFile.ReadAt(chunkData, offset)
				if err != nil && err != io.EOF {
					return http.StatusInternalServerError, err
				}

				newBody := &bytes.Buffer{}
				writer = multipart.NewWriter(newBody)

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

				part, err = writer.CreateFormFile("file", common.EscapeAndJoin(filename, "/"))
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

				if err != nil {
					TaskLog(task, "warning", fmt.Sprintf("generate body error: %v", err))
					continue
				}
				// newBody end

				req, err = http.NewRequest(request.Method, request.URL.String(), newBody)
				if err != nil {
					TaskLog(task, "warning", fmt.Sprintf("create request error: %v", err))
					continue
				}
				req.Header = make(http.Header)
				for k, s := range request.Header {
					req.Header[k] = s
				}
			}

			response, err = client.Do(req)
			klog.Infoln("Do Request (attempt", retry+1, ")")

			if err != nil {
				TaskLog(task, "warning", fmt.Sprintf("request error (attempt %d): %v", retry+1, err))

				if chunkNumber == totalChunks {
					if strings.Contains(err.Error(), "context deadline exceeded (Client.Timeout exceeded while awaiting headers)") {
						const gb = 1024 * 1024 * 1024
						additionalBlocks := size / (10 * gb)
						totalBubble := 15*time.Second + time.Duration(additionalBlocks)*15*time.Second
						TaskLog(task, "info", fmt.Sprintf("Waiting %ds for seafile to complete", int(totalBubble.Seconds())))
						time.Sleep(totalBubble)
						special = true
						if response != nil && response.Body != nil {
							response.Body.Close()
						}
						TaskLog(task, "info", fmt.Sprintf("Waiting for seafile to complete huge file done!"))
						break
					}
				}

				if response != nil && response.Body != nil {
					bodyBytes, err := io.ReadAll(response.Body)
					if err != nil {
						TaskLog(task, "warning", fmt.Sprintf("read body error: %v", err))
					} else {
						bodyString := string(bodyBytes)
						TaskLog(task, "info", fmt.Sprintf("error response: %s", bodyString))

						response.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					}
				} else {
					TaskLog(task, "info", fmt.Sprintf("got an empty error response"))
				}

				if retry < maxRetries-1 {
					waitTime := time.Duration(1<<uint(retry)) * time.Second
					klog.Warningf("Retrying in %v...", waitTime)
					time.Sleep(waitTime)
				}
				continue
			}

			if response.StatusCode == http.StatusOK {
				break
			}

			TaskLog(task, "warning", fmt.Sprintf("non-200 status: %s (attempt %d)", response.Status, retry+1))

			if response.Body != nil {
				response.Body.Close()
			}

			if retry < maxRetries-1 {
				waitTime := time.Duration(1<<uint(retry)) * time.Second
				klog.Warningf("Retrying in %v...", waitTime)
				time.Sleep(waitTime)
			}
		}

		if !special {
			if response == nil || response.StatusCode != http.StatusOK {
				statusCode := http.StatusInternalServerError
				statusMsg := "request failed after retries"

				if response != nil {
					statusCode = response.StatusCode
					statusMsg = response.Status
					if response.Body != nil {
						defer response.Body.Close()
					}
				}

				TaskLog(task, "warning", fmt.Sprintf("%s after %d attempts", statusMsg, maxRetries))
				return statusCode, fmt.Errorf("%s after %d attempts", statusMsg, maxRetries)
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

		TaskLog(task, "info", fmt.Sprintf("Chunk %d/%d from of bytes %d-%d/%d successfully transferred.", chunkNumber, totalChunks, chunkStart, chunkStart+int64(bytesRead)-1, size))
		task.Mu.Lock()
		task.Progress = finalProgress
		task.Mu.Unlock()

		time.Sleep(150 * time.Millisecond)
	}
	klog.Infoln("sync buffer to file success!")

	task.Mu.Lock()
	task.Progress = right
	task.Mu.Unlock()
	return 0, nil
}

func ResourceSyncDelete(path string, w http.ResponseWriter, r *http.Request, returnResp bool) (int, error) {
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
	if returnResp {
		type Response struct {
			Success  bool   `json:"success"`
			CommitID string `json:"commit_id"`
		}
		var resp Response
		if err = json.Unmarshal(deleteBody, &resp); err != nil {
			klog.Errorf("JSON parse failed: %v", err)
			return http.StatusInternalServerError, err
		}
		return common.RenderJSON(w, r, resp)
	}
	return 0, nil
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

func PasteSyncPatch(task *pool.Task, action, src, dst string, r *http.Request) error {
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
		TaskLog(task, "error", "Error:", err)
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
		//klog.Errorln("Error:", err)
		TaskLog(task, "error", "Error:", err)
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
	TaskLog(task, "info", requestBody)
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
	TaskLog(task, "info", "ReadAll")
	if err != nil {
		TaskLog(task, "error", "ReadAll error: ", err)
		return err
	}

	if response.StatusCode != http.StatusOK {
		TaskLog(task, "info", string(postBody))
		return fmt.Errorf("file paste failed with status: %d", response.StatusCode)
	}

	return nil
}
