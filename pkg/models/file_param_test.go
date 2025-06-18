package models

import (
	"fmt"
	"testing"
)

func TestCache(t *testing.T) {
	var path = "/api/resources/cache/olares/fasfd/12/32"
	var data = PathFormatter(path)

	var param = &FileParam{
		UserPvc:  "pvc-user",
		CachePvc: "pvc-cache",
	}

	param.ConvertToBackendUrl(data)
	fmt.Println(param.Json())
}

func TestSync(t *testing.T) {
	var path = "/api/resources/sync/"
	var data = PathFormatter(path)

	var param = &FileParam{
		UserPvc:  "pvc-user",
		CachePvc: "pvc-cache",
	}

	param.ConvertToBackendUrl(data)
	fmt.Println(param.Json())
}

func TestExternal(t *testing.T) {
	var path = "/api/resources/external/"
	var data = PathFormatter(path)

	var param = &FileParam{
		UserPvc:  "pvc-user",
		CachePvc: "pvc-cache",
	}

	param.ConvertToBackendUrl(data)
	fmt.Println(param.Json())
}
