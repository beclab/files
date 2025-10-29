package common

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

var rest *resty.Client

func init() {
	rest = resty.New().SetTimeout(60*time.Second).SetHeader("Accept-Encoding", "gzip")
}

func Request(u string, method string, header map[string]string, data interface{}, debug bool) ([]byte, error) {
	var backoff = wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    1,
	}

	var err error
	var result []byte

	if e := retry.OnError(backoff, func(err error) bool {
		return true
	}, func() error {
		var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		r := rest.SetDebug(debug).R().SetHeaders(header).SetContext(ctx)
		if data != nil {
			r.SetBody(data)
		}

		var resp *resty.Response

		switch method {
		case http.MethodGet:
			resp, err = r.Get(u)
		case http.MethodPost:
			resp, err = r.Post(u)
		case http.MethodDelete:
			resp, err = r.Delete(u)
		default:
			return errors.New("invalid request method")
		}

		if err != nil {
			return err
		}

		result = resp.Body()

		return nil
	}); e != nil {
		return nil, err
	}

	return result, nil
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
