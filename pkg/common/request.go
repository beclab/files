package common

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func RequestWithContext(u string, method string, header *http.Header, requestParams []byte) ([]byte, error) {
	var backoff = wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    3,
	}

	var result []byte
	var err error
	var newRequest *http.Request
	_ = newRequest
	var requestBody *bytes.Buffer = nil
	requestBody = bytes.NewBuffer(requestParams)

	if err := retry.OnError(backoff, func(err error) bool {
		return true
	}, func() error {
		var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var newRequest *http.Request
		if requestParams != nil {
			newRequest, err = http.NewRequestWithContext(ctx, method, u, requestBody)
		} else {
			newRequest, err = http.NewRequestWithContext(ctx, method, u, nil)
		}

		if err != nil {
			return err
		}

		var body []byte
		if header != nil {
			newRequest.Header = *header
		}

		if newRequest.Header.Get("Content-Type") == "" {
			newRequest.Header.Set("Content-Type", "application/json")
		}
		newRequest.Header.Del("Traceparent")
		newRequest.Header.Del("Tracestate")

		client := &http.Client{}
		resp, err := client.Do(newRequest)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Error request with status code %d", resp.StatusCode)
		}

		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				klog.Errorln("Error creating gzip reader:", err)
				return err
			}
			defer reader.Close()

			body, err = ioutil.ReadAll(reader)
			if err != nil {
				klog.Errorln("Error reading gzipped response body:", err)
				return err
			}
		} else {
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				klog.Errorln("Error reading response body:", err)
				return err
			}
		}
		result = body
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil

}

func RenderSuccess(w http.ResponseWriter, _ *http.Request) (int, error) {
	return 0, nil
}

func RenderJSON(w http.ResponseWriter, _ *http.Request, data interface{}) (int, error) {
	marsh, err := json.Marshal(data)

	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(marsh); err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func GetHost(bflName string) string {
	hostUrl := "http://bfl.user-space-" + bflName + "/bfl/info/v1/terminus-info"

	resp, err := http.Get(hostUrl)
	if err != nil {
		klog.Errorln("Error making GET request:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Errorln("Error reading response body:", err)
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		klog.Infof("Received non-200 response: %d\n", resp.StatusCode)
		return ""
	}

	type BflResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TerminusName    string `json:"terminusName"`
			WizardStatus    string `json:"wizardStatus"`
			Selfhosted      bool   `json:"selfhosted"`
			TailScaleEnable bool   `json:"tailScaleEnable"`
			OsVersion       string `json:"osVersion"`
			LoginBackground string `json:"loginBackground"`
			Avatar          string `json:"avatar"`
			TerminusId      string `json:"terminusId"`
			Did             string `json:"did"`
			ReverseProxy    string `json:"reverseProxy"`
			Terminusd       string `json:"terminusd"`
		} `json:"data"`
	}

	var responseObj BflResponse
	err = json.Unmarshal(body, &responseObj)
	if err != nil {
		klog.Errorln("Error unmarshaling JSON:", err)
		return ""
	}

	modifiedTerminusName := strings.Replace(responseObj.Data.TerminusName, "@", ".", 1)
	return "https://files." + modifiedTerminusName
}
