package utils

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func RequestWithContext[T any](u string, method string, header *http.Header, requestParams []byte) (*T, error) {

	var backoff = wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    3,
	}

	var result *T
	var requestBody *bytes.Buffer = nil
	if requestParams != nil {
		requestBody = bytes.NewBuffer(requestParams)
	}

	if err := retry.OnError(backoff, func(err error) bool {
		return true
	}, func() error {
		var ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var newRequest *http.Request
		newRequest, err := http.NewRequestWithContext(ctx, method, u, requestBody)
		if err != nil {
			return err
		}

		newRequest.Header = *header
		newRequest.Header.Set("Content-Type", "application/json")
		// newRequest.Header.Del("Traceparent")
		// newRequest.Header.Del("Tracestate")

		client := &http.Client{}
		resp, err := client.Do(newRequest)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Error request with status code %d", resp.StatusCode)
		}

		var body []byte

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

		if err = json.Unmarshal(body, &result); err != nil {
			klog.Errorln("Error unmarshaling JSON response:", err)
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil

}
