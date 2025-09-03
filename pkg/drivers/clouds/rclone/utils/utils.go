package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"files/pkg/drivers/clouds/rclone/common"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func Request(ctx context.Context, u string, method string, header *http.Header, requestParams []byte) ([]byte, error) {
	var backoff = wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    2,
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
		if header != nil {
			newRequest.Header = *header
		}

		if newRequest.Header.Get("Content-Type") == "" {
			newRequest.Header.Set("Content-Type", "application/json")
		}

		var body []byte

		// newRequest.Header.Add("Content-Type", "application/octet-stream")

		client := &http.Client{}
		start := time.Now()
		resp, err := client.Do(newRequest)
		if err != nil {
			klog.Errorf("[request] do error: %v", err)
			return err
		}
		defer resp.Body.Close()
		if strings.Contains(u, "operations/") || strings.Contains(u, "sync/") {
			klog.Infof("[request] url: %s, code: %d, elapsed: %s", u, resp.StatusCode, time.Since(start))
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			klog.Errorln("[request] error reading response body:", err)
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return FormatError(body)
		}

		result = body
		return nil
	}); err != nil {
		return result, err
	}

	return result, nil

}

func FormatError(resp []byte) error {
	// klog.Infof("[request] result: %s", string(resp))
	var errMsg *common.ErrorMessage
	if err := json.Unmarshal(resp, &errMsg); err != nil {
		return fmt.Errorf("request unmarshal error: %v, resp: %s", err, string(resp))
	}

	switch errMsg.Status {
	case http.StatusOK:
		return nil
	default:
		switch errMsg.Path {
		case
			"operations/deletefile", "operations/movefile",
			"operations/list", "operations/stat",
			"operations/purge",
			"config/create",
			"job/list", "job/stop", "job/status",
			"core/stats",
			"sync/copy", "sync/move":
			return fmt.Errorf("%s", errMsg.Error)
		}
		return fmt.Errorf("%s", string(resp))
	}
}
