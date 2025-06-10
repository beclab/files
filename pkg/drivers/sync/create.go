package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/models"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

func (s *SyncStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	klog.Infof("SYNC create, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())
	var w = s.Handler.ResponseWriter
	var r = s.Handler.Request

	if fileParam.Extend != "" {
		var url = "http://127.0.0.1:80/seahub/api2/repos/?from=web"
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		_ = writer.WriteField("name", fileParam.Extend)
		_ = writer.WriteField("passwd", "")
		res, err := s.Service.Get(url, http.MethodPost, body.Bytes())
		if err != nil {
			return common.ErrToStatus(err), err
		}

		return common.RenderJSON(w, r, res)
	}

	err := SyncMkdirAll(strings.TrimPrefix(r.URL.Path, "/sync"), 0, true, r)
	return common.ErrToStatus(err), err
}

func SyncMkdirAll(dst string, mode os.FileMode, isDir bool, r *http.Request) error {
	dst = strings.Trim(dst, "/")
	if !strings.Contains(dst, "/") {
		err := errors.New("invalid path format: path must contain at least one '/'")
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
			err = errors.New("mkdir failed")
			return err
		}
	}
	return nil
}

func RemoveAdditionalHeaders(header *http.Header) {
	header.Del("Traceparent")
	header.Del("Tracestate")
	return
}
