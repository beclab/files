package models

import (
	"fmt"
	"testing"
)

func TestCreateParam(t *testing.T) {
	var owner string
	var path string

	var param, err = CreateFileParam(owner, path)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(param.PrettyJson())
}
