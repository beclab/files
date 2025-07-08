package sync

import (
	"testing"

	"github.com/go-playground/assert/v2"
)

func TestGetCommonPrefix1(t *testing.T) {
	var dirents = []string{
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/f1/subfolder1/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/f2/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/f3/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/hello-1.txt",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/pic1.jpg",
	}

	var p = commonPathPrefix(dirents)
	assert.Equal(t, p, "/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/")
}

func TestGetCommonPrefix2(t *testing.T) {
	var dirents = []string{
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/f1/subfolder1/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/f2/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path2/f3/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path2/hello-1.txt",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path2/pic1.jpg",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/my-test/pic1.jpg",
	}

	var p = commonPathPrefix(dirents)
	assert.Equal(t, p, "/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/")
}

func TestGetCommonPrefix3(t *testing.T) {
	var dirents = []string{
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/f1/subfolder1/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/f2/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/f3/",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/hello-1.txt",
		"/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/path1/pic1.jpg",
		"/sync/10000001-5dd8-4051-98bd-000000000001/my-test/pic1.jpg",
	}

	var p = commonPathPrefix(dirents)
	assert.Equal(t, p, "/sync/")
}
