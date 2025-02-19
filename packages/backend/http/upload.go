package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
)

var uploadLinkHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var err error
	query := r.URL.Query()
	srcType := query.Get("src")

	var targetURL string
	if srcType == "" || srcType == "drive" {
		targetURL = "http://127.0.0.1:40030/upload/upload-link"
	} else if srcType == "sync" {
		dst := r.URL.Query().Get("p")
		dst, err = unescapeURLIfEscaped(dst)
		if err != nil {
			return http.StatusBadRequest, err
		}
		fmt.Println("dst Path:", dst)

		valid, checkResponse := checkSpecialCharacters("", "", dst, "sync")
		if !valid {
			return renderJSON(w, r, checkResponse)
		}

		repoID, prefix, _ := parseSyncPath(dst)

		targetURL = "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/upload-link/?p=" + escapeURLWithSpace(prefix) + "&from=web"
		fmt.Println(targetURL)
	} else {
		return http.StatusBadRequest, errors.New("invalid src parameter")
	}

	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to create new request")
	}

	if srcType == "" || srcType == "drive" {
		req.URL.RawQuery = r.URL.RawQuery
	}

	req.Header = r.Header.Clone()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to forward request")
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to copy response")
	}
	return http.StatusOK, nil
})

var uploadedBytesHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var err error
	query := r.URL.Query()
	srcType := query.Get("src")

	var targetURL string
	if srcType == "" || srcType == "drive" {
		targetURL = "http://127.0.0.1:40030/upload/file-uploaded-bytes"
	} else if srcType == "sync" {
		parent := r.URL.Query().Get("parent_dir")
		parent, err = unescapeURLIfEscaped(parent)
		if err != nil {
			return http.StatusBadRequest, err
		}
		fmt.Println("dst parent_dir:", parent)

		valid, checkResponse := checkSpecialCharacters("", "", parent, "sync")
		if !valid {
			return renderJSON(w, r, checkResponse)
		}

		filename := r.URL.Query().Get("file_name")
		filename, err = unescapeURLIfEscaped(filename)
		if err != nil {
			return http.StatusBadRequest, err
		}
		fmt.Println("dst file_name:", filename)

		valid, checkResponse = checkSpecialCharacters("", "", filename, "sync")
		if !valid {
			return renderJSON(w, r, checkResponse)
		}

		repoID, prefix, _ := parseSyncPath(parent)

		targetURL = "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/file-uploaded-bytes/?parent_dir=" + escapeAndJoin(prefix, "/") + "&file_name=" + escapeAndJoin(filename, "/")
		fmt.Println(targetURL)
	} else {
		return http.StatusBadRequest, errors.New("invalid src parameter")
	}

	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to create new request")
	}

	if srcType == "" || srcType == "drive" {
		req.URL.RawQuery = r.URL.RawQuery
	}

	req.Header = r.Header.Clone()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to forward request")
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to copy response")
	}
	return http.StatusOK, nil
})

type ResumableInfo struct {
	ResumableChunkNumber      int                   `json:"resumableChunkNumber" form:"resumableChunkNumber"`
	ResumableChunkSize        int64                 `json:"resumableChunkSize" form:"resumableChunkSize"`
	ResumableCurrentChunkSize int64                 `json:"resumableCurrentChunkSize" form:"resumableCurrentChunkSize"`
	ResumableTotalSize        int64                 `json:"resumableTotalSize" form:"resumableTotalSize"`
	ResumableType             string                `json:"resumableType" form:"resumableType"`
	ResumableIdentifier       string                `json:"resumableIdentifier" form:"resumableIdentifier"`
	ResumableFilename         string                `json:"resumableFilename" form:"resumableFilename"`
	ResumableRelativePath     string                `json:"resumableRelativePath" form:"resumableRelativePath"`
	ResumableTotalChunks      int                   `json:"resumableTotalChunks" form:"resumableTotalChunks"`
	ParentDir                 string                `json:"parent_dir" form:"parent_dir"`
	File                      *multipart.FileHeader `json:"file" form:"file" binding:"required"`
}

var uploadChunksHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var err error
	query := r.URL.Query()
	srcType := query.Get("src")

	vars := mux.Vars(r)
	uploadID := vars["uid"]

	var targetURL string
	if srcType == "" || srcType == "drive" {
		targetURL = "http://127.0.0.1:40030/upload/upload-link/" + uploadID + "?ret-json=1"
	} else if srcType == "sync" {
		var resumableInfo ResumableInfo
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			//http.Error(w, "Failed to read request body", http.StatusBadRequest)
			fmt.Printf("uploadID:%s, err:%v", uploadID, err)
			return http.StatusBadRequest, err
		}
		defer r.Body.Close()

		err = json.Unmarshal(body, &resumableInfo)
		if err != nil {
			//http.Error(w, "Invalid parameters", http.StatusBadRequest)
			fmt.Printf("uploadID:%s, err:%v", uploadID, err)
			return http.StatusBadRequest, err
		}

		filename := resumableInfo.ResumableFilename

		valid, checkResponse := checkSpecialCharacters("", "", filename, "sync")
		if !valid {
			return renderJSON(w, r, checkResponse)
		}

		targetURL = "http://127.0.0.1:80/seafhttp/upload-aj/" + uploadID + "&ret-json=1"
		fmt.Println(targetURL)
	} else {
		return http.StatusBadRequest, errors.New("invalid src parameter")
	}

	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to create new request")
	}

	//if srcType == "" || srcType == "drive" {
	//	req.URL.RawQuery = r.URL.RawQuery
	//}

	req.Header = r.Header.Clone()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to forward request")
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return http.StatusInternalServerError, err // errors.New("failed to copy response")
	}
	return http.StatusOK, nil
})
