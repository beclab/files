package drives

import (
	"bytes"
	"compress/gzip"
	"context"
	e "errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/models"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type SyncResourceService struct {
	BaseResourceService
}

// TODO：protected
func (rc *SyncResourceService) PutHandler(fileParam *models.FileParam) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.HttpData) (int, error) {
		// Only allow PUT for files.
		var err error
		if strings.HasSuffix(fileParam.Path, "/") {
			return http.StatusMethodNotAllowed, nil
		}

		//repoID := fileParam.Extend
		prefix, filename := filepath.Split(fileParam.Path)
		//getUrl := "http://127.0.0.1:80/seahub/api2/repos/" + repoID + "/update-link/?p=/"
		//klog.Infoln(getUrl)
		//
		//_, getRespBody, err := syncCall(getUrl, "GET", nil, nil, r, nil, true)
		//if err != nil {
		//	return common.ErrToStatus(err), err
		//}
		getRespBody, err := seahub.HandleUpdateLink(fileParam, "api")
		if err != nil {
			return common.ErrToStatus(err), err
		}

		updateLink := string(getRespBody)
		updateLink = strings.Trim(updateLink, "\"")

		//updateUrl := "http://127.0.0.1:80/seahub/" + updateLink
		updateUrl := "http://127.0.0.1:80/" + updateLink
		klog.Infoln(updateUrl)

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return common.ErrToStatus(err), err
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		_ = writer.WriteField("target_file", filepath.Join(prefix, filename))
		_ = writer.WriteField("filename", filename)
		klog.Infoln("target_file", filepath.Join(prefix, filename))

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

func SuitableResponseReader(resp *http.Response) io.ReadCloser {
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			klog.Errorf("unzip response failed: %v\n", err)
			return nil
		}
		return &autoCloseReader{
			Reader: gzipReader,
			closer: resp.Body,
		}
	}
	return resp.Body
}

type autoCloseReader struct {
	io.Reader
	closer io.Closer
}

func (a *autoCloseReader) Close() error {
	return a.closer.Close()
}

func RemoveAdditionalHeaders(header *http.Header) {
	header.Del("Traceparent")
	header.Del("Tracestate")
	return
}
