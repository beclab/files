package utils

import (
	"testing"

	"github.com/go-playground/assert/v2"
)

func TestCutSuffix(t *testing.T) {
	var path = "/folder/"
	var result = GetPrefixPath(path)
	assert.Equal(t, result, "/")

	path = "/folder/abc/"
	result = GetPrefixPath(path)
	assert.Equal(t, result, "/folder/")

	path = "/folder/abc/c1/"
	result = GetPrefixPath(path)
	assert.Equal(t, result, "/folder/abc/")

	path = "/folder/test/hello.pic"
	result = GetPrefixPath(path)
	assert.Equal(t, result, "/folder/test/")

	path = "/folder/test/s1/hello.pic"
	result = GetPrefixPath(path)
	assert.Equal(t, result, "/folder/test/s1/")

	path = "/"
	result = GetPrefixPath(path)
	assert.Equal(t, result, "/")
}
