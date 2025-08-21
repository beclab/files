package models

import (
	"errors"
	"files/pkg/common"
	"fmt"
	"mime/multipart"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

type ResumableInfo struct {
	ResumableChunkNumber      int                   `json:"resumableChunkNumber" form:"resumableChunkNumber"`
	ResumableChunkSize        int64                 `json:"resumableChunkSize" form:"resumableChunkSize"`
	ResumableCurrentChunkSize int64                 `json:"resumableCurrentChunkSize" form:"resumableCurrentChunkSize"`
	ResumableTotalSize        int64                 `json:"resumableTotalSize" form:"resumableTotalSize"`
	ResumableType             string                `json:"resumableType" form:"resumableType"`
	ResumableIdentifier       string                `json:"resumableIdentifier" form:"resumableIdentifier"`
	ResumableFilename         string                `json:"resumableFilename" form:"resumableFilename"`
	ResumableRelativePath     string                `json:"resumableRelativePath" form:"resumableRelativePath"`
	ResumableTotalChunks      int                   `json:"resumableTotalChunks" form:"resumableTotalChunks"`
	UploadToCloud             int                   `json:"uploadToCloud" form:"uploadToCloud"`                     // if cloud, val = 1
	UploadToCloudTaskId       string                `json:"uploadToCloudTaskId" form:"uploadToCloudTaskId"`         // task id from serve
	UploadToCloudTaskCancel   int                   `json:"uploadToCloudTaskCancel" from:"uploadToCloudTaskCancel"` // if canceled, val = 1
	ParentDir                 string                `json:"parent_dir" form:"parent_dir"`
	MD5                       string                `json:"md5,omitempty" form:"md5"`
	File                      *multipart.FileHeader `json:"file" form:"file" binding:"required"`
}

type FileUploadArgs struct {
	Node      string         `json:"node"` // node name
	FileParam *FileParam     `json:"fileParam"`
	FileName  string         `json:"fileName,omitempty"`
	From      string         `json:"from,omitempty"`
	UploadId  string         `json:"uploadId,omitempty"`
	Ranges    string         `json:"ranges,omitempty"`
	ChunkInfo *ResumableInfo `json:"chunkInfo,omitempty"`
}

func NewFileUploadArgs(r *http.Request) (*FileUploadArgs, error) {
	var fileUploadArgs = &FileUploadArgs{
		FileParam: &FileParam{},
	}
	var err error

	vars := mux.Vars(r)
	fileUploadArgs.Node = vars["node"]

	p := r.URL.Query().Get("file_path")
	if p == "" {
		p = r.URL.Query().Get("parent_dir")
		if p == "" {
			fileUploadArgs.UploadId = vars["uid"]

			fileUploadArgs.ChunkInfo = &ResumableInfo{}
			if err = ParseFormData(r, fileUploadArgs.ChunkInfo); err != nil {
				klog.Warningf("err:%v", err)
				return nil, errors.New("missing parent dir query parameter in resumable info")
			}

			file, header, err := r.FormFile("file")
			if err != nil {
				klog.Warningf("uploadID:%s, Failed to parse file: %v\n", fileUploadArgs.UploadId, err)
				return nil, errors.New("param invalid")
			}
			defer file.Close()

			fileUploadArgs.ChunkInfo.File = header

			p = fileUploadArgs.ChunkInfo.ParentDir
			if p == "" {
				return nil, errors.New("path invalid")
			}

			fileUploadArgs.Ranges = r.Header.Get("Content-Range")
		}
	}

	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}

	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return nil, errors.New("user not found")
	}

	fileUploadArgs.FileParam, err = CreateFileParam(owner, p)
	if err != nil {
		return nil, err
	}

	if r.URL.Query().Get("file_name") != "" {
		fileUploadArgs.FileName = r.URL.Query().Get("file_name")
	}

	if r.URL.Query().Get("from") != "" {
		fileUploadArgs.From = r.URL.Query().Get("from")
	}

	return fileUploadArgs, nil
}

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
