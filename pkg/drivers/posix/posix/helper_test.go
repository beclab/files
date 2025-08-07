package posix

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-playground/assert/v2"
)

func Test1(t *testing.T) {
	var a = "abc"
	var b = "abc"

	fmt.Println(strings.HasPrefix(a, b))
}

func TestCutSuffix(t *testing.T) {
	var path = "/folder/"
	var result = gerRenamedSrcPrefixPath(path)
	assert.Equal(t, result, "/")

	path = "/folder/abc/"
	result = gerRenamedSrcPrefixPath(path)
	assert.Equal(t, result, "/folder/")

	path = "/folder/abc/c1/"
	result = gerRenamedSrcPrefixPath(path)
	assert.Equal(t, result, "/folder/abc/")

	path = "/folder/test/hello.pic"
	result = gerRenamedSrcPrefixPath(path)
	assert.Equal(t, result, "/folder/test/")

	path = "/folder/test/s1/hello.pic"
	result = gerRenamedSrcPrefixPath(path)
	assert.Equal(t, result, "/folder/test/s1/")

	path = "/"
	result = gerRenamedSrcPrefixPath(path)
	assert.Equal(t, result, "/")
}
