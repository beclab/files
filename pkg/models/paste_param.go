package models

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"fmt"
	"net/http"
)

type PasteReq struct {
	Owner       string `json:"owner"`
	Extend      string `json:"extend"`
	Action      string `json:"action"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type PasteParam struct {
	Owner         string `json:"owner"`
	Action        string `json:"action"`
	UploadToCloud bool   `json:"uploadToCloud"`
	Src           *FileParam
	Dst           *FileParam
}

func NewPasteParam(r *http.Request) (*PasteParam, error) {
	var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var reqBody = &PasteReq{
		Owner: owner,
	}

	if e := json.NewDecoder(r.Body).Decode(&reqBody); e != nil {
		return nil, fmt.Errorf("failed to decode request body: %v", e)
	}
	defer r.Body.Close()

	src, err := CreateFileParam(owner, reqBody.Source)
	if err != nil {
		return nil, err
	}

	dst, err := CreateFileParam(owner, reqBody.Destination)
	if err != nil {
		return nil, err
	}

	var pasteParam = &PasteParam{
		Owner:  owner,
		Action: reqBody.Action,
		Src:    src,
		Dst:    dst,
	}

	return pasteParam, nil
}
