package models

import (
	"encoding/json"
	"errors"
	"files/pkg/constant"
	"fmt"
	"net/http"
)

type PasteParam struct {
	Owner       string `json:"owner"`
	Extend      string `json:"extend"`
	Action      string `json:"action"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

func NewPasteParam(r *http.Request) (*PasteParam, error) {
	var owner = r.Header.Get(constant.REQUEST_HEADER_OWNER)
	if owner == "" {
		return nil, errors.New("user not found")
	}

	var arg = &PasteParam{
		Owner: owner,
	}

	if e := json.NewDecoder(r.Body).Decode(&arg); e != nil {
		return nil, fmt.Errorf("failed to decode request body: %v", e)
	}
	defer r.Body.Close()

	return arg, nil
}
