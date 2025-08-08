package models

import (
	"encoding/json"
	"errors"
	"files/pkg/utils"
	"fmt"
	"net/http"
	"strings"
)

type FileDeleteArgs struct {
	FileParam *FileParam `json:"fileParam"`
	Dirents   []string   `json:"dirents"`
}

func NewFileDeleteArgs(r *http.Request, prefix string) (*FileDeleteArgs, error) {
	var fileDeleteArgs = &FileDeleteArgs{
		FileParam: &FileParam{},
	}
	var err error

	var p = r.URL.Path
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, errors.New("path invalid")
	}

	var owner = r.Header.Get(utils.REQUEST_HEADER_OWNER)
	if owner == "" {
		return nil, errors.New("user not found")
	}

	fileDeleteArgs.FileParam, err = CreateFileParam(owner, path)
	if err != nil {
		return nil, err
	}

	if e := json.NewDecoder(r.Body).Decode(&fileDeleteArgs); e != nil {
		return nil, fmt.Errorf("failed to decode request body: %v", e)
	}
	defer r.Body.Close()

	return fileDeleteArgs, nil
}
