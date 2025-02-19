package http

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

func checkSpecialCharacters(src, srcType, dst, dstType string) (bool, map[string]interface{}) {
	if dstType == "sync" && strings.Contains(dst, "\\") {
		response := map[string]interface{}{
			"code": -1,
			"msg":  "Sync does not support directory entries with backslashes in their names.",
		}
		return false, response
	}
	return true, nil
}

func commonPrefixEndWithSlash(A, B string) string {
	minLength := len(A)
	if len(B) < minLength {
		minLength = len(B)
	}

	for i := 0; i < minLength; i++ {
		if A[i] != B[i] {
			return A[:i]
		}
	}

	commonPrefix := A[:minLength]

	lastIndex := strings.LastIndex(commonPrefix, "/")
	if lastIndex != -1 && lastIndex != len(commonPrefix)-1 {
		return commonPrefix[:lastIndex+1]
	}

	return commonPrefix
}

func parseOldAndNewName(src, dst string) (string, string) {
	commonPrefix := commonPrefixEndWithSlash(src, dst)
	oldName := strings.TrimPrefix(src, commonPrefix)
	newName := strings.TrimPrefix(dst, commonPrefix)
	fmt.Println("oldName: ", oldName, ", newName: ", newName)
	return oldName, newName
}

func parseSyncPath(path string) (repoID, prefix, filename string) {
	firstSlashIdx := strings.Index(path, "/")

	repoID = path[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(path, "/")

	// don't use, because this is only used for folders
	filename = path[lastSlashIdx+1:]

	prefix = ""
	if firstSlashIdx != lastSlashIdx {
		prefix = path[firstSlashIdx+1 : lastSlashIdx+1]
	}
	if prefix == "" {
		prefix = "/"
	}
	//prefix = url.QueryEscape(prefix)
	prefix = escapeURLWithSpace(prefix)

	fmt.Println("repo-id:", repoID)
	fmt.Println("prefix:", prefix)
	fmt.Println("filename:", filename)
	return
}

func resourceGetSync(w http.ResponseWriter, r *http.Request, stream int) (int, error) {
	// src is like [repo-id]/path/filename
	src := r.URL.Path
	src, err := unescapeURLIfEscaped(src) // url.QueryUnescape(src)
	if err != nil {
		return http.StatusBadRequest, err
	}
	fmt.Println("src Path:", src)
	src = strings.Trim(src, "/") + "/"
	//if !strings.Contains(src, "/") {
	//	err := e.New("invalid path format: path must contain at least one '/'")
	//	fmt.Println("Error:", err)
	//	return errToStatus(err), err
	//}

	repoID, prefix, _ := parseSyncPath(src)

	url := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace(prefix) + "&with_thumbnail=true"
	fmt.Println(url)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return errToStatus(err), err
	}

	request.Header = r.Header

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return errToStatus(err), err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil
	}

	// SSE
	if stream == 1 {
		var body []byte
		if response.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(response.Body)
			defer reader.Close()
			if err != nil {
				fmt.Println("Error creating gzip reader:", err)
				return errToStatus(err), err
			}

			body, err = ioutil.ReadAll(reader)
			if err != nil {
				fmt.Println("Error reading gzipped response body:", err)
				reader.Close()
				return errToStatus(err), err
			}
		} else {
			body, err = ioutil.ReadAll(response.Body)
			if err != nil {
				fmt.Println("Error reading response body:", err)
				return errToStatus(err), err
			}
		}
		//body, _ := ioutil.ReadAll(response.Body)
		streamSyncDirents(w, r, body, repoID)
		return 0, nil
	}

	// non-SSE
	var responseBody io.Reader = response.Body
	if response.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(response.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return errToStatus(err), err
		}
		defer reader.Close()
		responseBody = reader
	}

	_, err = io.Copy(w, responseBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return errToStatus(err), err
	}

	return 0, nil
}

func resourcePostSync(w http.ResponseWriter, r *http.Request) (int, error) {
	// src is like [repo-id]/path/filename
	src := r.URL.Path
	src, err := unescapeURLIfEscaped(src) // url.QueryUnescape(src)
	if err != nil {
		return http.StatusBadRequest, err
	}
	fmt.Println("src Path:", src)
	src = strings.Trim(src, "/") + "/"

	valid, checkResponse := checkSpecialCharacters("", "", src, "sync")
	if !valid {
		return renderJSON(w, r, checkResponse)
	}

	repoID, prefix, _ := parseSyncPath(src)

	url := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace(prefix)
	fmt.Println(url)

	type CreateDirRequest struct {
		Operation string `json:"operation"`
	}

	createDirReq := CreateDirRequest{
		Operation: "mkdir",
	}
	jsonBody, err := json.Marshal(createDirReq)
	if err != nil {
		fmt.Printf("failed to serialize the request body: %v\n", err)
		return errToStatus(err), err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return errToStatus(err), err
	}

	request.Header = r.Header

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return errToStatus(err), err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil
	}

	var responseBody io.Reader = response.Body
	if response.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(response.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return errToStatus(err), err
		}
		defer reader.Close()
		responseBody = reader
	}

	_, err = io.Copy(w, responseBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return errToStatus(err), err
	}

	return 0, nil
}

func resourcePutSync(w http.ResponseWriter, r *http.Request) (int, error) {
	// src is like [repo-id]/path/filename
	src := r.URL.Path
	src, err := unescapeURLIfEscaped(src) // url.QueryUnescape(src)
	if err != nil {
		return http.StatusBadRequest, err
	}
	fmt.Println("src Path:", src)
	src = strings.Trim(src, "/") + "/"

	dst := r.URL.Query().Get("destination")
	dst, err = unescapeURLIfEscaped(dst)
	if err != nil {
		return http.StatusBadRequest, err
	}
	fmt.Println("dst Path:", dst)

	valid, checkResponse := checkSpecialCharacters("", "", dst, "sync")
	if !valid {
		return renderJSON(w, r, checkResponse)
	}

	repoID, prefix, _ := parseSyncPath(src)

	url := "http://127.0.0.1:80/seahub/api/v2.1/repos/" + repoID + "/dir/?p=" + escapeURLWithSpace(prefix)
	fmt.Println(url)

	_, newName := parseOldAndNewName(src, dst)

	type CreateDirRequest struct {
		Operation string `json:"operation"`
		Newname   string `json:"newname"`
	}

	createDirReq := CreateDirRequest{
		Operation: "rename",
		Newname:   newName,
	}
	jsonBody, err := json.Marshal(createDirReq)
	if err != nil {
		fmt.Printf("failed to serialize the request body: %v\n", err)
		return errToStatus(err), err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return errToStatus(err), err
	}

	request.Header = r.Header

	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return errToStatus(err), err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil
	}

	var responseBody io.Reader = response.Body
	if response.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(response.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return errToStatus(err), err
		}
		defer reader.Close()
		responseBody = reader
	}

	_, err = io.Copy(w, responseBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return errToStatus(err), err
	}

	return 0, nil
}
