package handler

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// This is an addaptation if http.StripPrefix in which we don't
// return 404 if the page doesn't have the needed prefix.
func stripPrefix(prefix string, h http.Handler) http.Handler {
	if prefix == "" || prefix == "/" {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, prefix)
		rp := strings.TrimPrefix(r.URL.RawPath, prefix)
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p
		r2.URL.RawPath = rp
		h.ServeHTTP(w, r2)
	})
}

func getOwner(r *http.Request) (ownerID, ownerName string) {
	bflName := r.Header.Get("X-Bfl-User")
	url := "http://bfl.user-space-" + bflName + "/bfl/info/v1/terminus-info"

	resp, err := http.Get(url)
	if err != nil {
		klog.Errorln("Error making GET request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Errorln("Error reading response body:", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		klog.Infof("Received non-200 response: %d\n", resp.StatusCode)
		return
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
		return
	}

	ownerID = responseObj.Data.TerminusId
	ownerName = responseObj.Data.TerminusName
	return
}

// Universal form data parser with type conversion and error handling
func ParseFormData(r *http.Request, v interface{}) error {
	// 1. Validate content type
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return fmt.Errorf("invalid content type: expected multipart/form-data")
	}

	// 2. Parse form with memory limit (32MB)
	const maxMemory = 32 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return fmt.Errorf("form parsing failed: %w", err)
	}

	// 3. Get reflection values
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a struct pointer")
	}
	val = val.Elem()
	typ := val.Type()

	// 4. Get form data
	form := r.MultipartForm.Value
	files := r.MultipartForm.File

	// 5. Iterate through struct fields
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("form")
		if tag == "" {
			continue // Skip untagged fields
		}

		// 6. Field assignment with type conversion
		switch field.Type.Kind() {
		case reflect.String:
			if vals, ok := form[tag]; ok && len(vals) > 0 {
				val.Field(i).SetString(vals[0])
			}

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if vals, ok := form[tag]; ok && len(vals) > 0 {
				num, err := strconv.ParseInt(vals[0], 10, 64)
				if err != nil {
					return fmt.Errorf("invalid integer value for field '%s': %v", tag, err)
				}
				val.Field(i).SetInt(num)
			}

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if vals, ok := form[tag]; ok && len(vals) > 0 {
				num, err := strconv.ParseUint(vals[0], 10, 64)
				if err != nil {
					return fmt.Errorf("invalid unsigned integer value for field '%s': %v", tag, err)
				}
				val.Field(i).SetUint(num)
			}

		case reflect.Float32, reflect.Float64:
			if vals, ok := form[tag]; ok && len(vals) > 0 {
				num, err := strconv.ParseFloat(vals[0], 64)
				if err != nil {
					return fmt.Errorf("invalid float value for field '%s': %v", tag, err)
				}
				val.Field(i).SetFloat(num)
			}

		case reflect.Bool:
			if vals, ok := form[tag]; ok && len(vals) > 0 {
				b, err := strconv.ParseBool(vals[0])
				if err != nil {
					return fmt.Errorf("invalid boolean value for field '%s': %v", tag, err)
				}
				val.Field(i).SetBool(b)
			}

		case reflect.Ptr:
			// Handle file uploads
			if fileHeaders, ok := files[tag]; ok && len(fileHeaders) > 0 {
				val.Field(i).Set(reflect.ValueOf(fileHeaders[0]))
			}
		}
	}

	return nil
}
