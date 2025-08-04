package utils

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func Request(ctx context.Context, u string, method string, requestParams []byte) ([]byte, error) {
	var backoff = wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    3,
	}

	var result []byte
	var err error

	if err := retry.OnError(backoff, func(err error) bool {
		return true
	}, func() error {
		var newRequest *http.Request
		var requestBody *bytes.Buffer = nil
		requestBody = bytes.NewBuffer(requestParams)

		if requestParams != nil {
			newRequest, err = http.NewRequestWithContext(ctx, method, u, requestBody)
		} else {
			newRequest, err = http.NewRequestWithContext(ctx, method, u, nil)
		}

		if err != nil {
			return err
		}

		var body []byte

		newRequest.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(newRequest)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			klog.Errorln("Error reading response body:", err)
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s", string(body))
		}

		result = body
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil

}
