package handler

import (
	"encoding/json"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
	"net/http"
	"net/http/httptest"
	"reflect"
)

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
)

func ConvertHertzRequest(hertzReq *protocol.Request) (*http.Request, error) {
	// create basic request
	stdReq := &http.Request{
		Method:     string(hertzReq.Method()),
		URL:        &url.URL{Path: string(hertzReq.Path())},
		Header:     make(http.Header),
		Host:       string(hertzReq.Host()),
		RequestURI: string(hertzReq.RequestURI()),
	}

	// copy Header
	hertzReq.Header.VisitAll(func(key, value []byte) {
		stdReq.Header.Add(string(key), string(value))
	})

	// copy query params
	stdReq.URL.RawQuery = string(hertzReq.QueryString())

	// deal with Body
	if body := hertzReq.Body(); len(body) > 0 {
		stdReq.Body = io.NopCloser(bytes.NewReader(body))
	}

	// if is multipart request
	contentType := stdReq.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		// parse Hertz multipart form
		form, err := hertzReq.MultipartForm()
		if err != nil {
			return nil, fmt.Errorf("parse multipart form failed: %v", err)
		}

		// create mem-buffer
		buf := &bytes.Buffer{}
		writer := multipart.NewWriter(buf)

		// trans file part
		for name, hertzFiles := range form.File {
			for _, hertzFile := range hertzFiles {
				// create form file field
				part, err := writer.CreateFormFile(name, hertzFile.Filename)
				if err != nil {
					return nil, fmt.Errorf("create form file failed: %v", err)
				}

				// read file content
				file, err := hertzFile.Open()
				if err != nil {
					return nil, fmt.Errorf("open file failed: %v", err)
				}
				defer file.Close()

				// write content to new form
				if _, err := io.Copy(part, file); err != nil {
					return nil, fmt.Errorf("copy file content failed: %v", err)
				}
			}
		}

		// convert normal fields
		for key, vals := range form.Value {
			for _, val := range vals {
				if err := writer.WriteField(key, val); err != nil {
					return nil, fmt.Errorf("write field failed: %v", err)
				}
			}
		}

		// close writer to complete form creation
		writer.Close()

		// reset request body
		stdReq.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
		stdReq.Header.Set("Content-Type", writer.FormDataContentType())

		// parse form data
		if err := stdReq.ParseMultipartForm(32 << 20); err != nil { // 32MB mem-buffer
			if !strings.Contains(err.Error(), "request Content-Type isn't multipart/form-data") {
				return nil, fmt.Errorf("parse form failed: %v", err)
			}
		}
	}

	return stdReq, nil
}

func CopyHeaders(dst *protocol.ResponseHeader, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Set(k, v)
		}
	}
}

func Coalesce(vals ...interface{}) interface{} {
	for _, v := range vals {
		if val := reflect.ValueOf(v); !isNil(val) {
			if val.Kind() != reflect.Ptr && val.Kind() != reflect.Interface {
				return v
			}
			if !val.IsNil() {
				return v
			}
		}
	}
	return nil
}

func isNil(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return val.IsNil()
	default:
		return false
	}
}

// for normal request
func CommonConvert(c *app.RequestContext, originalHandler http.Handler, resp interface{}, direct bool) {
	stdReq, err := ConvertHertzRequest(&c.Request)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}
	recorder := httptest.NewRecorder()
	originalHandler.ServeHTTP(recorder, stdReq)
	CopyHeaders(&c.Response.Header, recorder.Header())

	bodyBytes := recorder.Body.Bytes()
	if direct {
		if len(bodyBytes) > 0 {
			var jsonResp map[string]interface{}
			err = json.Unmarshal(bodyBytes, &jsonResp)
			if err == nil {
				c.JSON(recorder.Code, jsonResp)
				return
			} else {
				klog.Infof("Failed to unmarshal response body: %v", err)
			}
		}
		c.JSON(recorder.Code, string(bodyBytes))
		return
	}

	if recorder.Code == http.StatusOK && len(bodyBytes) > 0 {
		if err = json.Unmarshal(bodyBytes, &resp); err != nil {
			klog.Errorf("Failed to unmarshal response body: %v", err)
			c.String(consts.StatusBadRequest, "Failed to unmarshal response body")
			return
		}
	}
	c.JSON(recorder.Code, resp)
}

func StraightForward(c *app.RequestContext, originalHandler http.Handler) {
	stdReq, err := ConvertHertzRequest(&c.Request)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	recorder := httptest.NewRecorder()
	originalHandler.ServeHTTP(recorder, stdReq)
	CopyHeaders(&c.Response.Header, recorder.Header())

	c.Status(recorder.Code)
	if recorder.Body.Len() > 0 {
		c.Response.SetBodyStream(recorder.Body, -1)
	}
}
